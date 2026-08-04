package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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
	maxLogBody := flag.Int("max-log-body", 0, "cap stored request/response body size in bytes for logs and dashboard (0 = unlimited)")
	flag.Parse()

	if *tui != "auto" && *tui != "on" && *tui != "off" {
		log.Fatalf("invalid --tui value %q: must be auto, on, or off", *tui)
	}
	if *recent <= 0 {
		log.Fatalf("invalid --recent %d: must be > 0", *recent)
	}
	if *maxLogBody < 0 {
		log.Fatalf("invalid --max-log-body %d: must be >= 0 (0 = unlimited)", *maxLogBody)
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
	if v := os.Getenv("LOG_MAX_BODY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			log.Fatalf("invalid LOG_MAX_BODY %q: must be a non-negative integer", v)
		}
		*maxLogBody = n
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
		if err := os.MkdirAll(*logDir, 0o755); err != nil {
			log.Fatalf("create log dir: %v", err)
		}
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

	// Admin endpoints (control plane — not matched against stubs, not logged).
	admin := mock.NewAdminHandler(registry)
	mux.Handle(mock.AdminEndpointsPath, admin)      // collection
	mux.Handle(mock.AdminEndpointsPath+"/", admin)  // item: /{id}

	// Webhook handler — matched & unmatched requests are logged
	observers := []func(mock.LogEntry){
		func(e mock.LogEntry) { _ = logger.Log(e) },
		stats.RecordRequest,
	}
	if !tuiEnabled {
		observers = append(observers, func(e mock.LogEntry) {
			fmt.Println(mock.FormatRequestLine(e))
		})
	}
	mux.Handle("/", mock.NewHandler(mock.HandlerConfig{
		Registry:     registry,
		Observers:    observers,
		MaxBodyBytes: *maxLogBody,
	}))

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
		exitErr = dashboard.Run(stats, registry, dashboard.Config{
			Addr:     *addr,
			StubsDir: *stubsDir,
			LogDir:   *logDir,
		}, termWidth, termHeight)
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
