package daemon

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/micwin/vaultline/internal/daemoncfg"
)

type bindRuntime struct {
	server   *http.Server
	listener net.Listener
}

type NetworkManager struct {
	cfg      *daemoncfg.Manager
	handler  http.Handler
	mu       sync.Mutex
	runtimes map[string]*bindRuntime
}

func NewNetworkManager(cfg *daemoncfg.Manager) *NetworkManager {
	return &NetworkManager{cfg: cfg, runtimes: map[string]*bindRuntime{}}
}

func (m *NetworkManager) SetHandler(handler http.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

func (m *NetworkManager) Apply() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.handler == nil {
		return fmt.Errorf("network manager handler not configured")
	}
	binds := m.cfg.ListBinds()
	wanted := map[string]daemoncfg.Bind{}
	for _, bind := range binds {
		wanted[bind.Addr] = bind
	}
	for addr, runtime := range m.runtimes {
		if _, ok := wanted[addr]; !ok {
			_ = runtime.server.Close()
			_ = runtime.listener.Close()
			delete(m.runtimes, addr)
		}
	}
	for _, bind := range binds {
		if existing, ok := m.runtimes[bind.Addr]; ok {
			_ = existing.server.Close()
			_ = existing.listener.Close()
			delete(m.runtimes, bind.Addr)
		}
		listener, err := net.Listen("tcp", bind.Addr)
		if err != nil {
			return err
		}
		server := &http.Server{Addr: bind.Addr, Handler: allowMiddleware(bind.Allows, m.handler)}
		runtime := &bindRuntime{server: server, listener: listener}
		m.runtimes[bind.Addr] = runtime
		go func(addr string, srv *http.Server, ln net.Listener, rules []string) {
			state := "blocked"
			if len(rules) > 0 {
				state = "allowlisted"
			}
			log.Printf("vaultline extra listener active on %s (%s)", addr, state)
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				log.Printf("vaultline extra listener %s failed: %v", addr, err)
			}
		}(bind.Addr, server, listener, bind.Allows)
	}
	return nil
}

func (m *NetworkManager) AddBind(addr string) error {
	if err := m.cfg.AddBind(addr); err != nil {
		return err
	}
	return m.Apply()
}

func (m *NetworkManager) RemoveBind(addr string) error {
	if err := m.cfg.RemoveBind(addr); err != nil {
		return err
	}
	return m.Apply()
}

func (m *NetworkManager) AddAllow(addr, rule string) error {
	if err := m.cfg.AddAllow(addr, rule); err != nil {
		return err
	}
	return m.Apply()
}

func (m *NetworkManager) RemoveAllow(addr, rule string) error {
	if err := m.cfg.RemoveAllow(addr, rule); err != nil {
		return err
	}
	return m.Apply()
}

func (m *NetworkManager) ListBinds() []daemoncfg.Bind {
	return m.cfg.ListBinds()
}

func (m *NetworkManager) ListAllows(addr string) ([]string, error) {
	return m.cfg.ListAllows(addr)
}

func (m *NetworkManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for addr, runtime := range m.runtimes {
		_ = runtime.server.Close()
		_ = runtime.listener.Close()
		delete(m.runtimes, addr)
	}
}

func allowMiddleware(allows []string, next http.Handler) http.Handler {
	nets := make([]*net.IPNet, 0, len(allows))
	for _, rule := range allows {
		_, network, err := net.ParseCIDR(rule)
		if err == nil {
			nets = append(nets, network)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(nets) == 0 {
			http.Error(w, "remote access blocked until allow rules are configured", http.StatusForbidden)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "invalid remote address", http.StatusForbidden)
			return
		}
		ip := net.ParseIP(host)
		if ip == nil {
			http.Error(w, "invalid remote address", http.StatusForbidden)
			return
		}
		for _, network := range nets {
			if network.Contains(ip) {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, fmt.Sprintf("client %s is not allowed", strings.TrimSpace(host)), http.StatusForbidden)
	})
}
