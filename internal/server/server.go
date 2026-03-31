package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/micwin/vaultline/internal/daemoncfg"
	"github.com/micwin/vaultline/pkg/api"
	"github.com/micwin/vaultline/pkg/storage"
	"github.com/micwin/vaultline/pkg/stores"
)

type daemonNetwork interface {
	AddBind(addr string) error
	RemoveBind(addr string) error
	AddAllow(addr, rule string) error
	RemoveAllow(addr, rule string) error
	ListBinds() []daemoncfg.Bind
	ListAllows(addr string) ([]string, error)
}

// Server exposes the REST API for vaultline.
type Server struct {
	stores  *stores.Manager
	network daemonNetwork
	router  chi.Router
	version string
}

// New constructs the HTTP server with registered routes.
func New(manager *stores.Manager, network daemonNetwork, version string) *Server {
	s := &Server{
		stores:  manager,
		network: network,
		router:  chi.NewRouter(),
		version: version,
	}
	s.router.Get("/", s.handleDashboard)
	s.router.Get("/api/v1/health", s.handleHealth)
	s.router.Post("/api/v1/seal", s.handleLocalSeal)
	s.router.Post("/api/v1/unseal", s.handleLocalUnseal)
	s.router.Post("/api/v1/shutdown", s.handleShutdown)
	s.router.Get("/api/v1/daemon/binds", s.handleListBinds)
	s.router.Post("/api/v1/daemon/binds", s.handleAddBind)
	s.router.Delete("/api/v1/daemon/binds/{addr}", s.handleDeleteBind)
	s.router.Get("/api/v1/daemon/binds/{addr}/allows", s.handleListAllows)
	s.router.Post("/api/v1/daemon/binds/{addr}/allows", s.handleAddAllow)
	s.router.Delete("/api/v1/daemon/binds/{addr}/allows/{rule}", s.handleDeleteAllow)
	s.router.Get("/api/v1/secrets", s.handleLocalListSecrets)
	s.router.Get("/api/v1/secrets/{name}", s.handleLocalGetSecret)
	s.router.Put("/api/v1/secrets/{name}", s.handleLocalPutSecret)
	s.router.Delete("/api/v1/secrets/{name}", s.handleLocalDeleteSecret)
	s.router.Get("/api/v1/stores", s.handleListStores)
	s.router.Post("/api/v1/stores", s.handleCreateStore)
	s.router.Get("/api/v1/stores/{store}", s.handleStoreInfo)
	s.router.Delete("/api/v1/stores/{store}", s.handleDeleteStore)
	s.router.Post("/api/v1/stores/{store}/seal", s.handleStoreSeal)
	s.router.Post("/api/v1/stores/{store}/unseal", s.handleStoreUnseal)
	s.router.Get("/api/v1/stores/{store}/secrets", s.handleListSecrets)
	s.router.Get("/api/v1/stores/{store}/secrets/{name}", s.handleGetSecret)
	s.router.Put("/api/v1/stores/{store}/secrets/{name}", s.handlePutSecret)
	s.router.Delete("/api/v1/stores/{store}/secrets/{name}", s.handleDeleteSecret)
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "<html><head><title>vaultline</title><style>body{font-family:sans-serif;margin:2rem;}h1{margin-bottom:1rem;}section{margin-bottom:1.5rem;}code{background:#f4f4f4;padding:2px 4px;border-radius:3px;}</style></head><body>")
	fmt.Fprintf(builder, "<h1>vaultline %s</h1>", s.version)
	infos, err := s.stores.List()
	if err != nil {
		builder.WriteString("<p>Unable to list stores.</p></body></html>")
		_, _ = w.Write([]byte(builder.String()))
		return
	}
	builder.WriteString("<ul>")
	for _, info := range infos {
		state := "sealed"
		if info.Available && !info.Sealed {
			state = "unsealed"
		}
		if !info.Available {
			state = "unavailable"
		}
		fmt.Fprintf(builder, "<li><code>%s</code> — %s</li>", info.Name, state)
	}
	builder.WriteString("</ul></body></html>")
	_, _ = w.Write([]byte(builder.String()))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	infos, err := s.stores.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	status := "ok"
	for _, info := range infos {
		if !info.Available {
			status = "degraded"
			break
		}
	}
	payload := map[string]any{
		"status":        status,
		"version":       s.version,
		"default_store": s.stores.DefaultStore(),
		"stores":        infos,
	}
	if isLoopbackRequest(r) {
		binds := s.network.ListBinds()
		bindInfos := make([]map[string]any, 0, len(binds))
		for _, bind := range binds {
			state := "blocked"
			if len(bind.Allows) > 0 {
				state = "allowlisted"
			}
			bindInfos = append(bindInfos, map[string]any{
				"addr":        bind.Addr,
				"allow_count": len(bind.Allows),
				"state":       state,
			})
		}
		payload["binds"] = bindInfos
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleLocalSeal(w http.ResponseWriter, r *http.Request) {
	s.handleSealForStore(stores.DefaultStoreName, w, r)
}
func (s *Server) handleLocalListSecrets(w http.ResponseWriter, r *http.Request) {
	s.handleListSecretsForStore(stores.DefaultStoreName, w)
}
func (s *Server) handleLocalGetSecret(w http.ResponseWriter, r *http.Request) {
	s.handleGetSecretForStore(stores.DefaultStoreName, chi.URLParam(r, "name"), w)
}
func (s *Server) handleLocalPutSecret(w http.ResponseWriter, r *http.Request) {
	s.handlePutSecretForStore(stores.DefaultStoreName, chi.URLParam(r, "name"), w, r)
}
func (s *Server) handleLocalDeleteSecret(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteSecretForStore(stores.DefaultStoreName, chi.URLParam(r, "name"), w)
}
func (s *Server) handleLocalUnseal(w http.ResponseWriter, r *http.Request) {
	s.handleUnsealForStore(stores.DefaultStoreName, w, r)
}

func (s *Server) handleListStores(w http.ResponseWriter, r *http.Request) {
	infos, err := s.stores.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stores": infos})
}

func (s *Server) handleStoreInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.stores.Info(chi.URLParam(r, "store"))
	if err != nil {
		writeError(w, http.StatusNotFound, "STORE_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleDeleteStore(w http.ResponseWriter, r *http.Request) {
	if err := s.stores.Remove(chi.URLParam(r, "store")); err != nil {
		writeError(w, http.StatusBadRequest, "STORE_DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListBinds(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "LOOPBACK_ONLY", "daemon bind information is only available via loopback")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"binds": s.network.ListBinds()})
}

func (s *Server) handleAddBind(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "LOOPBACK_ONLY", "daemon bind management is only available via loopback")
		return
	}
	var req api.DaemonBindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := s.network.AddBind(req.Addr); err != nil {
		writeError(w, http.StatusBadRequest, "BIND_ADD_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"addr": req.Addr})
}

func (s *Server) handleDeleteBind(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "LOOPBACK_ONLY", "daemon bind management is only available via loopback")
		return
	}
	addr, err := url.PathUnescape(chi.URLParam(r, "addr"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BIND", err.Error())
		return
	}
	if err := s.network.RemoveBind(addr); err != nil {
		writeError(w, http.StatusBadRequest, "BIND_DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAllows(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "LOOPBACK_ONLY", "daemon allow information is only available via loopback")
		return
	}
	addr, err := url.PathUnescape(chi.URLParam(r, "addr"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BIND", err.Error())
		return
	}
	allows, err := s.network.ListAllows(addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ALLOW_LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"addr": addr, "allows": allows})
}

func (s *Server) handleAddAllow(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "LOOPBACK_ONLY", "daemon allow management is only available via loopback")
		return
	}
	addr, err := url.PathUnescape(chi.URLParam(r, "addr"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BIND", err.Error())
		return
	}
	var req api.DaemonAllowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := s.network.AddAllow(addr, req.Rule); err != nil {
		writeError(w, http.StatusBadRequest, "ALLOW_ADD_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"addr": addr, "rule": req.Rule})
}

func (s *Server) handleDeleteAllow(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "LOOPBACK_ONLY", "daemon allow management is only available via loopback")
		return
	}
	addr, err := url.PathUnescape(chi.URLParam(r, "addr"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BIND", err.Error())
		return
	}
	rule, err := url.PathUnescape(chi.URLParam(r, "rule"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ALLOW", err.Error())
		return
	}
	if err := s.network.RemoveAllow(addr, rule); err != nil {
		writeError(w, http.StatusBadRequest, "ALLOW_DELETE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateStore(w http.ResponseWriter, r *http.Request) {
	var req api.StoreCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := s.stores.Add(req.Name, req.Path, req.Initialize); err != nil {
		writeError(w, http.StatusBadRequest, "STORE_CREATE_FAILED", err.Error())
		return
	}
	response := api.StoreCreateResponse{}
	if req.Initialize {
		passphrase, err := generatePassphrase()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORE_CREATE_FAILED", err.Error())
			return
		}
		if err := s.stores.Unseal(req.Name, passphrase); err != nil {
			writeError(w, http.StatusInternalServerError, "STORE_CREATE_FAILED", err.Error())
			return
		}
		response.Passphrase = passphrase
	}
	info, _ := s.stores.Info(req.Name)
	response.Name = info.Name
	response.Path = info.Path
	response.Default = info.Default
	response.Available = info.Available
	response.Sealed = info.Sealed
	response.HasKey = info.HasKey
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleStoreSeal(w http.ResponseWriter, r *http.Request) {
	s.handleSealForStore(chi.URLParam(r, "store"), w, r)
}

func (s *Server) handleStoreUnseal(w http.ResponseWriter, r *http.Request) {
	s.handleUnsealForStore(chi.URLParam(r, "store"), w, r)
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	s.handleListSecretsForStore(chi.URLParam(r, "store"), w)
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	s.handleGetSecretForStore(chi.URLParam(r, "store"), chi.URLParam(r, "name"), w)
}

func (s *Server) handlePutSecret(w http.ResponseWriter, r *http.Request) {
	s.handlePutSecretForStore(chi.URLParam(r, "store"), chi.URLParam(r, "name"), w, r)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteSecretForStore(chi.URLParam(r, "store"), chi.URLParam(r, "name"), w)
}

func (s *Server) handleSealForStore(name string, w http.ResponseWriter, r *http.Request) {
	store, err := s.stores.Store(name)
	if err != nil {
		handleStoreError(w, name, err)
		return
	}
	var req api.SealRequest
	if r != nil && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if store.Sealed() {
		writeError(w, http.StatusConflict, "ALREADY_SEALED", "store is already sealed")
		return
	}
	if err := s.stores.Seal(name, req.KeepKeys); err != nil {
		handleStoreError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sealed": true, "store": name, "keep_keys": req.KeepKeys})
}

func (s *Server) handleUnsealForStore(name string, w http.ResponseWriter, r *http.Request) {
	var req api.UnsealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if err := s.stores.Unseal(name, req.Passphrase); err != nil {
		handleStoreError(w, name, err)
		return
	}
	writeJSON(w, http.StatusOK, api.UnsealResponse{Store: name, Sealed: false})
}

func (s *Server) handlePutSecretForStore(storeName, name string, w http.ResponseWriter, r *http.Request) {
	store, err := s.stores.Store(storeName)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	if store.Sealed() {
		writeError(w, http.StatusConflict, "SEALED", fmt.Sprintf("store %s is sealed", storeName))
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
	version, err := store.Put(name, payload)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	writeJSON(w, http.StatusOK, api.VersionResponse{Version: version})
}

func (s *Server) handleListSecretsForStore(storeName string, w http.ResponseWriter) {
	store, err := s.stores.Store(storeName)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	keys, err := store.ListKeys(0)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "shutting_down"})
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}()
}

func (s *Server) handleGetSecretForStore(storeName, name string, w http.ResponseWriter) {
	store, err := s.stores.Store(storeName)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	secret, err := store.Get(name)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	resp := api.SecretResponse{Value: base64.StdEncoding.EncodeToString(secret.Data), Version: secret.Version}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteSecretForStore(storeName, name string, w http.ResponseWriter) {
	store, err := s.stores.Store(storeName)
	if err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	if err := store.Delete(name); err != nil {
		handleStoreError(w, storeName, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleStoreError(w http.ResponseWriter, storeName string, err error) {
	switch {
	case errors.Is(err, storage.ErrSealed):
		writeError(w, http.StatusConflict, "SEALED", fmt.Sprintf("store %s is sealed", storeName))
	case errors.Is(err, storage.ErrSecretNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("secret not found in %s", storeName))
	case errors.Is(err, stores.ErrStoreUnavailable):
		writeError(w, http.StatusServiceUnavailable, "STORE_UNAVAILABLE", err.Error())
	case strings.Contains(err.Error(), "unknown store"):
		writeError(w, http.StatusNotFound, "STORE_NOT_FOUND", err.Error())
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
	writeJSON(w, status, api.ErrorResponse{Error: code, Message: message})
}

func generatePassphrase() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
