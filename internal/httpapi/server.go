package httpapi

import (
	"encoding/json"
	"net/http"

	"simple_auth_by_doubao/internal/config"
)

type Server struct {
	cfg *config.Config
	mux *http.ServeMux
}

func NewServer(cfg *config.Config) http.Handler {
	s := &Server{
		cfg: cfg,
		mux: http.NewServeMux(),
	}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "UI is not implemented yet", http.StatusNotImplemented)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
