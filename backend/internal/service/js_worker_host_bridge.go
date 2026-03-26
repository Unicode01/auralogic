package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"auralogic/internal/pkg/pluginutil"
	"auralogic/internal/pluginipc"
)

const defaultPluginHostBridgeConnTimeout = 30 * time.Second
const maxPluginHostBridgeConnTimeout = 30 * time.Minute

func (s *JSWorkerSupervisor) ensureHostBridgeRunning() error {
	if s == nil {
		return fmt.Errorf("js worker supervisor is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hostListener != nil {
		return nil
	}

	network, address, err := s.resolveHostBridgeEndpointLocked()
	if err != nil {
		return err
	}
	if err := pluginutil.ValidateJSWorkerSocketEndpoint(network, address); err != nil {
		return fmt.Errorf("invalid plugin host bridge endpoint: %w", err)
	}
	if network == "unix" {
		_ = os.Remove(address)
		if err := os.MkdirAll(filepath.Dir(address), 0o755); err != nil {
			return fmt.Errorf("create plugin host bridge socket dir failed: %w", err)
		}
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		return fmt.Errorf("listen plugin host bridge %s(%s) failed: %w", network, address, err)
	}

	actualAddress := address
	if network == "tcp" {
		actualAddress = listener.Addr().String()
	}
	s.hostListener = listener
	s.hostNetwork = network
	s.hostAddress = actualAddress

	go s.serveHostBridge(listener, network, actualAddress)
	log.Printf("Plugin host bridge started network=%s address=%s", network, actualAddress)
	return nil
}

func (s *JSWorkerSupervisor) stopHostBridgeLocked() {
	listener := s.hostListener
	network := s.hostNetwork
	address := s.hostAddress

	s.hostListener = nil
	s.hostNetwork = ""
	s.hostAddress = ""

	if listener != nil {
		_ = listener.Close()
	}
	if network == "unix" && strings.TrimSpace(address) != "" {
		_ = os.Remove(address)
	}
}

func (s *JSWorkerSupervisor) resolveHostBridgeEndpointLocked() (string, string, error) {
	if runtime.GOOS == "windows" {
		return "tcp", "127.0.0.1:0", nil
	}

	workerNetwork, workerAddress, err := s.workerEndpoint()
	if err == nil && workerNetwork == "unix" && strings.TrimSpace(workerAddress) != "" {
		return "unix", derivePluginHostBridgeSocketPath(workerAddress), nil
	}

	return "unix", filepath.Join(os.TempDir(), fmt.Sprintf("auralogic-plugin-host-%d.sock", os.Getpid())), nil
}

func derivePluginHostBridgeSocketPath(workerSocketPath string) string {
	trimmed := strings.TrimSpace(workerSocketPath)
	if trimmed == "" {
		return filepath.Join(os.TempDir(), fmt.Sprintf("auralogic-plugin-host-%d.sock", os.Getpid()))
	}
	cleaned := filepath.Clean(trimmed)
	ext := filepath.Ext(cleaned)
	if strings.EqualFold(ext, ".sock") {
		return strings.TrimSuffix(cleaned, ext) + "-host.sock"
	}
	return cleaned + ".host.sock"
}

func (s *JSWorkerSupervisor) serveHostBridge(listener net.Listener, network string, address string) {
	defer func() {
		if network == "unix" && strings.TrimSpace(address) != "" {
			_ = os.Remove(address)
		}
		s.mu.Lock()
		if s.hostListener == listener {
			s.hostListener = nil
			s.hostNetwork = ""
			s.hostAddress = ""
		}
		s.mu.Unlock()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if isClosedPluginHostBridgeError(err) {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				log.Printf("Plugin host bridge accept temporary error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			log.Printf("Plugin host bridge accept error: %v", err)
			return
		}
		go s.handleHostBridgeConn(conn)
	}
}

func isClosedPluginHostBridgeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func (s *JSWorkerSupervisor) handleHostBridgeConn(conn net.Conn) {
	if conn == nil {
		return
	}
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	runtime := NewPluginHostRuntime(s.db, s.cfg, s.pluginManager)
	secret := ""
	if s != nil && s.cfg != nil {
		secret = strings.TrimSpace(s.cfg.JWT.Secret)
	}
	if secret == "" {
		writePluginHostBridgeResponse(conn, pluginipc.HostResponse{
			Success: false,
			Status:  503,
			Error:   "plugin host bridge secret is unavailable",
		})
		return
	}
	cachedToken := ""
	var cachedClaims *PluginHostAccessClaims
	for {
		_ = conn.SetDeadline(time.Now().Add(defaultPluginHostBridgeConnTimeout))

		var req pluginipc.HostRequest
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			writePluginHostBridgeResponse(conn, pluginipc.HostResponse{
				Success: false,
				Status:  400,
				Error:   fmt.Sprintf("invalid plugin host request payload: %v", err),
			})
			return
		}
		_ = conn.SetDeadline(time.Now().Add(resolvePluginHostBridgeRequestTimeout(req)))
		token := strings.TrimSpace(req.AccessToken)
		if token == "" {
			writePluginHostBridgeResponse(conn, pluginipc.HostResponse{
				Success: false,
				Status:  401,
				Error:   "plugin host token is invalid",
			})
			return
		}
		if token != cachedToken || cachedClaims == nil {
			claims, err := ParsePluginHostAccessToken(secret, token)
			if err != nil {
				writePluginHostBridgeResponse(conn, pluginipc.HostResponse{
					Success: false,
					Status:  401,
					Error:   "plugin host token is invalid",
				})
				return
			}
			cachedToken = token
			cachedClaims = claims
		}

		data, execErr := ExecutePluginHostActionWithRuntime(
			runtime,
			cachedClaims,
			req.Action,
			req.Params,
		)
		if execErr != nil {
			status := 400
			if hostErr, ok := execErr.(*PluginHostActionError); ok && hostErr != nil && hostErr.Status > 0 {
				status = hostErr.Status
			}
			writePluginHostBridgeResponse(conn, pluginipc.HostResponse{
				Success: false,
				Status:  status,
				Error:   strings.TrimSpace(execErr.Error()),
			})
			continue
		}

		writePluginHostBridgeResponse(conn, pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data:    data,
		})
	}
}

func writePluginHostBridgeResponse(conn net.Conn, resp pluginipc.HostResponse) {
	if conn == nil {
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(defaultPluginHostBridgeConnTimeout))
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		log.Printf("Plugin host bridge write response failed: %v", err)
	}
}

func resolvePluginHostBridgeRequestTimeout(req pluginipc.HostRequest) time.Duration {
	timeout := defaultPluginHostBridgeConnTimeout
	if paramsTimeout := parsePluginHostPositiveInt(req.Params, 0, 1, int(maxPluginHostBridgeConnTimeout/time.Millisecond), "timeout_ms", "timeoutMs"); paramsTimeout > 0 {
		requested := time.Duration(paramsTimeout) * time.Millisecond
		if requested > timeout {
			timeout = requested
		}
	}
	if timeout > maxPluginHostBridgeConnTimeout {
		timeout = maxPluginHostBridgeConnTimeout
	}
	return timeout
}
