package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"simuhook/dashboard"
	"simuhook/mock"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	stubsDir := flag.String("stubs", "../stubs", "stubs directory")
	logDir := flag.String("log-dir", "logs", "log directory")
	tui := flag.String("tui", "auto", "dashboard mode: auto|on|off")
	recent := flag.Int("recent", 50, "recent log ring buffer size")
	flag.Parse()

	if *tui != "auto" && *tui != "on" && *tui != "off" {
		log.Fatalf("invalid --tui value %q: must be auto, on, or off", *tui)
	}
	if *recent <= 0 {
		log.Fatalf("invalid --recent %d: must be > 0", *recent)
	}

	if v := os.Getenv("PORT"); v != "" && *addr == ":8080" {
		*addr = ":" + v
	}
	if v := os.Getenv("STUBS_DIR"); v != "" {
		*stubsDir = v
	}
	if v := os.Getenv("LOG_DIR"); v != "" {
		*logDir = v
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	registry := mock.NewRegistry()

	watcher, err := mock.NewWatcher(*stubsDir, registry)
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}
	if err := watcher.LoadAll(); err != nil {
		log.Fatalf("load stubs: %v", err)
	}
	watcher.Watch()
	defer watcher.Close()

	// Decide whether to use the dashboard TUI
	tuiEnabled := *tui == "on" || (*tui == "auto" && term.IsTerminal(int(os.Stdout.Fd())))
	termWidth, termHeight := 80, 24
	if tuiEnabled {
		if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			termWidth, termHeight = w, h
		}
	}

	logger := mock.NewLogger(*logDir)
	defer logger.Close()

	// Redirect standard log output to a file when TUI is enabled to avoid
	// corrupting the dashboard screen (e.g. watcher reload messages).
	if tuiEnabled {
		logf, err := os.OpenFile(filepath.Join(*logDir, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("open server log: %v", err)
		}
		defer logf.Close()
		log.SetOutput(logf)
	}

	stats := mock.NewStatsTracker(*recent)

	// Build boxed startup banner (only when TUI is disabled)
	if !tuiEnabled {
		eps := registry.List()
		boxWidth := 60
		endpointLines := make([]string, 0, len(eps))
		for _, ep := range eps {
			method := ep.Method
			if method == "" {
				method = "ANY "
			}
			line := fmt.Sprintf("    %-5s %s → %s", method, ep.Path, ep.ID)
			if len(line)+4 > boxWidth {
				boxWidth = len(line) + 4
			}
			endpointLines = append(endpointLines, line)
		}
		// Ensure minimum width for title and config lines
		minWidth := 60
		if len("Simuhook Mock Server")+6 > minWidth {
			minWidth = len("Simuhook Mock Server") + 6
		}
		configLines := []string{
			fmt.Sprintf("  Address:    %s", *addr),
			fmt.Sprintf("  Stubs:      %s (%d files)", *stubsDir, len(eps)),
			fmt.Sprintf("  Log dir:    %s", *logDir),
			fmt.Sprintf("  Admin API:  http://localhost%s/_admin/endpoints", *addr),
		}
		for _, l := range configLines {
			if len(l)+2 > boxWidth {
				boxWidth = len(l) + 2
			}
		}
		if boxWidth < minWidth {
			boxWidth = minWidth
		}

		// If there are no endpoints, account for the no-endpoints line
		if len(eps) == 0 {
			boxWidth = max(boxWidth, 50)
		}

		hline := strings.Repeat("═", boxWidth-2)
		title := "Simuhook Mock Server"
		titlePad := (boxWidth - 2 - len(title)) / 2

		fmt.Println()
		fmt.Printf("╔%s╗\n", hline)
		fmt.Printf("║%*s%s%*s║\n", titlePad, "", title, boxWidth-2-titlePad-len(title), "")
		fmt.Printf("╠%s╣\n", hline)
		for _, l := range configLines {
			fmt.Printf("║ %-*s║\n", boxWidth-3, l[2:])
		}
		fmt.Printf("╠%s╣\n", hline)
		if len(eps) > 0 {
			fmt.Printf("║  %-*s║\n", boxWidth-4, "Endpoints:")
			for _, line := range endpointLines {
				fmt.Printf("║    %-*s║\n", boxWidth-6, line[4:])
			}
		} else {
			fmt.Printf("║  %-*s║\n", boxWidth-4, "Endpoints: (none loaded)")
		}
		fmt.Printf("╚%s╝\n", hline)
		fmt.Println()
	}

	mux := http.NewServeMux()

	// Admin endpoints (not logged)
	mux.HandleFunc("/_admin/endpoints", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(registry.List())

		case http.MethodPost:
			var ep mock.Endpoint
			if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := registry.Add(&ep); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(ep)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/_admin/endpoints/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/_admin/endpoints/")
		if id == "" {
			http.Error(w, "missing endpoint id", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			ep, err := registry.Get(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ep)

		case http.MethodPut:
			var ep mock.Endpoint
			if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := registry.Update(id, &ep); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ep)

		case http.MethodDelete:
			if err := registry.Remove(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Webhook handler — matched & unmatched requests are logged
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := mock.NewRequestID()

		body := make([]byte, 0)
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			defer r.Body.Close()
		}
		headers := make(map[string]string)
		for k := range r.Header {
			headers[k] = r.Header.Get(k)
		}
		query := make(map[string]string)
		for k := range r.URL.Query() {
			query[k] = r.URL.Query().Get(k)
		}

		ep := registry.Match(r.Method, r.URL.Path, headers, query, body)

		var (
			responseStatus int
			webhookName    string
			responseBody   []byte
		)

		if ep == nil {
			responseStatus = http.StatusNotFound
			responseBody = []byte(`{"error":"no matching endpoint"}`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(responseStatus)
			w.Write(responseBody)
		} else {
			webhookName = ep.ID

			// Pick response variant (singular or from responses array)
			resp := mock.PickResponse(ep)

			// Build template data (needed for both body and headers if template: true)
			var tmplData *mock.TemplateData
			if resp.Template {
				tmplData = mock.BuildTemplateData(requestID, r.Method, r.URL.Path, body)
			}

			if resp.Delay > 0 {
				time.Sleep(resp.Delay)
			}

			// Set response headers (with optional template execution)
			for k, v := range resp.Headers {
				if resp.Template {
					v = mock.ExecuteHeaderTemplate(v, tmplData, headers, query)
				}
				w.Header().Set(k, v)
			}

			responseStatus = resp.StatusCode
			if responseStatus == 0 {
				responseStatus = http.StatusOK
			}
			w.WriteHeader(responseStatus)

			// Echo mode takes precedence over everything
			if resp.Echo {
				responseBody = body
			} else if resp.Template && len(resp.Body) > 0 {
				rendered := mock.ExecuteBodyTemplate(string(resp.Body), tmplData, headers, query)
				responseBody = []byte(rendered)
			} else {
				responseBody = resp.Body
			}

			if len(responseBody) > 0 {
				w.Write(responseBody)
			}
		}

		// Log every request with full details
		latency := time.Since(start)
		entry := mock.LogEntry{
			Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
			RequestID:      requestID,
			Webhook:        webhookName,
			Method:         r.Method,
			Path:           r.URL.Path,
			Headers:        headers,
			Body:           string(body),
			Query:          query,
			ResponseStatus: responseStatus,
			LatencyMs:      float64(latency.Microseconds()) / 1000.0,
			Matched:        ep != nil,
		}
		logger.Log(entry)
		stats.RecordRequest(entry)

		// Print per-request line (only when TUI is disabled)
		if !tuiEnabled {
			timeStr := time.Now().Format("15:04:05")
			methodPad := fmt.Sprintf("%-5s", r.Method)
			statusLine := fmt.Sprintf("%d", responseStatus)
			label := webhookName
			if label == "" {
				label = "unmatched"
			}
			fmt.Printf("%s  %s %s → %s  (%s)\n", timeStr, methodPad, r.URL.Path, statusLine, label)
		}
	})

	srv := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		log.Printf("listening on %s", *addr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Shutdown path: TUI mode uses bubbletea's own signal handling;
	// non-TUI mode waits for OS signals directly.
	var exitErr error
	if tuiEnabled {
		exitErr = dashboard.Run(stats, termWidth, termHeight)
	} else {
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGTERM)
		<-done
		fmt.Println("\nshutting down...")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	if exitErr != nil {
		os.Exit(1)
	}
}
