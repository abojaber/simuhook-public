package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"webhooker/mock"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	stubsDir := flag.String("stubs", "../stubs", "stubs directory")
	logDir := flag.String("log-dir", "logs", "log directory")
	flag.Parse()

	if v := os.Getenv("PORT"); v != "" && *addr == ":8080" {
		*addr = ":" + v
	}
	if v := os.Getenv("STUBS_DIR"); v != "" {
		*stubsDir = v
	}
	if v := os.Getenv("LOG_DIR"); v != "" {
		*logDir = v
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

	logger := mock.NewLogger(*logDir)
	defer logger.Close()

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
		)

		if ep == nil {
			responseStatus = http.StatusNotFound
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(responseStatus)
			w.Write([]byte(`{"error":"no matching endpoint"}`))
		} else {
			webhookName = ep.ID
			if ep.Response.Delay > 0 {
				time.Sleep(ep.Response.Delay)
			}
			for k, v := range ep.Response.Headers {
				w.Header().Set(k, v)
			}
			responseStatus = ep.Response.StatusCode
			if responseStatus == 0 {
				responseStatus = http.StatusOK
			}
			w.WriteHeader(responseStatus)

			// Echo mode: return the request body verbatim, ignore configured body
			if ep.Response.Echo {
				w.Write(body)
			} else if len(ep.Response.Body) > 0 {
				w.Write(ep.Response.Body)
			}
		}

		// Log every request with full details
		latency := time.Since(start)
		logger.Log(mock.LogEntry{
			Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
			RequestID:      mock.NewRequestID(),
			Webhook:        webhookName,
			Method:         r.Method,
			Path:           r.URL.Path,
			Headers:        headers,
			Body:           string(body),
			Query:          query,
			ResponseStatus: responseStatus,
			LatencyMs:      float64(latency.Microseconds()) / 1000.0,
			Matched:        ep != nil,
		})
	})

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("listening on %s", *addr)
		if err := http.ListenAndServe(*addr, mux); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	fmt.Println("Webhooker mock server started")
	fmt.Println("Admin API: http://localhost" + *addr + "/_admin/endpoints")
	fmt.Printf("Stubs dir: %s\n", *stubsDir)
	fmt.Printf("Log dir: %s\n", *logDir)
	<-done
	fmt.Println("\nshutting down...")
}
