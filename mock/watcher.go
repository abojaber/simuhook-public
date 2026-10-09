package mock

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	registry *Registry
	dir      string
	watcher  *fsnotify.Watcher
	done     chan struct{}
}

func NewWatcher(dir string, registry *Registry) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}
	return &Watcher{
		registry: registry,
		dir:      dir,
		watcher:  w,
		done:     make(chan struct{}),
	}, nil
}

func (w *Watcher) LoadAll() error {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		path := filepath.Join(w.dir, entry.Name())
		ep, err := ParseStubFile(path)
		if err != nil {
			log.Printf("stub: skip %s: %v", entry.Name(), err)
			continue
		}
		if err := w.registry.Add(ep); err != nil {
			if errors.Is(err, ErrDuplicate) {
				log.Printf("stub: duplicate %s, updating", ep.ID)
				if uerr := w.registry.Update(ep.ID, ep); uerr != nil {
					log.Printf("stub: update %s: %v", ep.ID, uerr)
				}
			} else {
				// Compile/validation error — a genuine load failure, not a
				// hot-reload update. Log and skip, consistent with the
				// existing invalid-delay handling.
				log.Printf("stub: skip %s: %v", entry.Name(), err)
				continue
			}
		}
		log.Printf("stub: loaded %s from %s", ep.ID, entry.Name())
	}
	return nil
}

func (w *Watcher) Watch() {
	go func() {
		for {
			select {
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				if !strings.HasSuffix(event.Name, ".yaml") && !strings.HasSuffix(event.Name, ".yml") {
					continue
				}
				id := fileID(event.Name)
				switch {
				case event.Has(fsnotify.Create) || event.Has(fsnotify.Write):
					ep, err := ParseStubFile(event.Name)
					if err != nil {
						log.Printf("stub: %s parse error: %v", id, err)
						continue
					}
					if err := w.registry.Add(ep); err != nil {
						if errors.Is(err, ErrDuplicate) {
							log.Printf("stub: duplicate %s, updating", ep.ID)
							if uerr := w.registry.Update(ep.ID, ep); uerr != nil {
								log.Printf("stub: update %s: %v", ep.ID, uerr)
							}
						} else {
							log.Printf("stub: %s parse error: %v", id, err)
							continue
						}
					}
					log.Printf("stub: loaded %s", ep.ID)
				case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
					w.registry.Remove(id)
					log.Printf("stub: removed %s", id)
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("stub: watcher error: %v", err)
			case <-w.done:
				return
			}
		}
	}()
}

func (w *Watcher) Close() {
	close(w.done)
	w.watcher.Close()
}

func fileID(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}
