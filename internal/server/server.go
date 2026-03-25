package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/micwin/mono-repo/vaultline/pkg/api"
	"github.com/micwin/mono-repo/vaultline/pkg/storage"
)

// Server exposes the REST API for vaultline.
type Server struct {
	store   *storage.Store
	router  chi.Router
	version string
}

// New constructs the HTTP server with registered routes.
func New(store *storage.Store, version string) *Server {
	s := &Server{
		store:   store,
		router:  chi.NewRouter(),
		version: version,
	}
	s.router.Get("/", s.handleDashboard)
	s.router.Get("/api/v1/health", s.handleHealth)
	s.router.Post("/api/v1/seal", s.handleSeal)
	s.router.Post("/api/v1/unseal", s.handleUnseal)
	s.router.Post("/api/v1/shutdown", s.handleShutdown)
	s.router.Get("/api/v1/secrets/{name}", s.handleGetSecret)
	s.router.Put("/api/v1/secrets/{name}", s.handlePutSecret)
	s.router.Delete("/api/v1/secrets/{name}", s.handleDeleteSecret)

	return s
}

// Handler returns the configured router.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "<html><head><title>vaultline</title><style>body{font-family:sans-serif;margin:2rem;}h1{margin-bottom:1rem;}section{margin-bottom:1.5rem;}code{background:#f4f4f4;padding:2px 4px;border-radius:3px;}</style></head><body>")
	fmt.Fprintf(builder, "<h1>vaultline %s</h1>", s.version)
	if s.store.Sealed() {
		builder.WriteString("<p>Status: <strong>sealed</strong>. Unseal via CLI to view secrets.</p>")
		builder.WriteString("</body></html>")
		w.Write([]byte(builder.String()))
		return
	}
	keys, err := s.store.ListKeys(20)
	if err != nil {
		builder.WriteString("<p>Unable to list secrets.</p>")
		builder.WriteString("</body></html>")
		w.Write([]byte(builder.String()))
		return
	}
	if len(keys) == 0 {
		builder.WriteString("<p>No secrets stored yet.</p>")
		builder.WriteString("</body></html>")
		w.Write([]byte(builder.String()))
		return
	}
	builder.WriteString("<ul>")
	for _, key := range keys {
		fmt.Fprintf(builder, "<li><code>%s</code></li>", key)
	}
	builder.WriteString("</ul>")
	builder.WriteString("</body></html>")
	w.Write([]byte(builder.String()))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"sealed":  s.store.Sealed(),
		"status":  "ok",
		"version": s.version,
	})
}

func (s *Server) handleSeal(w http.ResponseWriter, r *http.Request) {
	if s.store.Sealed() {
		writeError(w, http.StatusConflict, "ALREADY_SEALED", "store is already sealed")
		return
	}
	s.store.Seal()
	writeJSON(w, http.StatusOK, map[string]any{"sealed": true})
}

func (s *Server) handleUnseal(w http.ResponseWriter, r *http.Request) {
	var req api.UnsealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := s.store.Unseal(req.Passphrase); err != nil {
		writeError(w, http.StatusBadRequest, "UNSEAL_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.UnsealResponse{Sealed: false})
}

func (s *Server) handlePutSecret(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if s.store.Sealed() {
		writeError(w, http.StatusConflict, "SEALED", "vaultline is sealed")
		return
	}
	var req api.SecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	payload, err := base64.StdEncoding.DecodeString(req.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "value must be base64 encoded")
		return
	}
	version, err := s.store.Put(name, payload)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.VersionResponse{Version: version})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "shutting_down"})
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}()
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	secret, err := s.store.Get(name)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	resp := api.SecretResponse{
		Value:   base64.StdEncoding.EncodeToString(secret.Data),
		Version: secret.Version,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.Delete(name); err != nil {
		handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrSealed):
		writeError(w, http.StatusConflict, "SEALED", "vaultline is sealed")
	case errors.Is(err, storage.ErrSecretNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, api.ErrorResponse{
		Error:   code,
		Message: message,
	})
}
