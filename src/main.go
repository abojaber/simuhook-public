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
	flag.Parse()

	if v := os.Getenv("PORT"); v != "" && *addr == ":8080" {
		*addr = ":" + v
	}
	if v := os.Getenv("STUBS_DIR"); v != "" {
		*stubsDir = v
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

	mux := http.NewServeMux()

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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		if ep == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"no matching endpoint"}`))
			return
		}

		if ep.Response.Delay > 0 {
			time.Sleep(ep.Response.Delay)
		}
		for k, v := range ep.Response.Headers {
			w.Header().Set(k, v)
		}
		code := ep.Response.StatusCode
		if code == 0 {
			code = http.StatusOK
		}
		w.WriteHeader(code)
		if len(ep.Response.Body) > 0 {
			w.Write(ep.Response.Body)
		}
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
	<-done
	fmt.Println("\nshutting down...")
}
