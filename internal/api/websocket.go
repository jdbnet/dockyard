package api

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"git.jdbnet.co.uk/jamie/dockyard/internal/docker"
	"git.jdbnet.co.uk/jamie/dockyard/ui"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWSEvents(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch := s.eng.Subscribe()
	defer s.eng.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(ev); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleWSStats(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	id := r.URL.Query().Get("id")
	ticker := time.NewTicker(s.cfg.Stats.Interval.Duration)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if id != "" {
				stats := s.eng.ContainerStats(id)
				if err := conn.WriteJSON(stats); err != nil {
					return
				}
			} else {
				list, err := s.eng.Containers(ctx)
				if err != nil {
					continue
				}
				payload := make([]json.RawMessage, 0, len(list))
				for _, c := range list {
					b, _ := json.Marshal(map[string]any{
						"container": c,
						"stats":     s.eng.ContainerStats(c.ID),
					})
					payload = append(payload, b)
				}
				if err := conn.WriteJSON(payload); err != nil {
					return
				}
			}
		}
	}
}

func (s *Server) handleWSLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	list, _ := s.eng.Containers(r.Context())
	id = matchContainerID(list, id)

	err = s.eng.StreamLogs(r.Context(), id, docker.LogOptions{Tail: "100", Follow: true}, func(line string) error {
		return conn.WriteMessage(websocket.TextMessage, []byte(line))
	})
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
	}
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
		http.NotFound(w, r)
		return
	}

	staticFS, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		http.Error(w, "frontend not embedded", http.StatusInternalServerError)
		return
	}

	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	if ext := filepath.Ext(path); ext != "" {
		f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
			return
		}
	}

	index, err := staticFS.Open("index.html")
	if err != nil {
		http.Error(w, "Frontend not built. Run: cd ui && npm run build", http.StatusNotFound)
		return
	}
	defer index.Close()
	stat, _ := index.Stat()
	http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
}
