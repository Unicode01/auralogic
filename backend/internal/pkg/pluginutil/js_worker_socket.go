package pluginutil

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

const jsWorkerTCPScheme = "tcp://"

func ResolveJSWorkerSocketEndpoint(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("js worker socket path is empty")
	}

	if strings.HasPrefix(strings.ToLower(trimmed), jsWorkerTCPScheme) {
		address := strings.TrimSpace(trimmed[len(jsWorkerTCPScheme):])
		if err := ValidateJSWorkerSocketEndpoint("tcp", address); err != nil {
			return "", "", err
		}
		return "tcp", address, nil
	}

	if err := ValidateJSWorkerSocketEndpoint("unix", trimmed); err != nil {
		return "", "", err
	}
	return "unix", trimmed, nil
}

func ValidateJSWorkerSocketEndpoint(network string, address string) error {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "unix":
		if strings.TrimSpace(address) == "" {
			return fmt.Errorf("js worker unix socket path is empty")
		}
		return nil
	case "tcp":
		return validateLoopbackTCPAddress(address)
	default:
		return fmt.Errorf("unsupported js worker network %q", network)
	}
}

func validateLoopbackTCPAddress(address string) error {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return fmt.Errorf("js worker tcp address is empty")
	}

	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return fmt.Errorf("invalid js worker tcp address: %w", err)
	}

	normalizedHost := strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if normalizedHost == "localhost" || strings.HasSuffix(normalizedHost, ".localhost") {
		return nil
	}

	addr, err := netip.ParseAddr(normalizedHost)
	if err != nil || !addr.IsLoopback() {
		return fmt.Errorf("js worker tcp host must be loopback or localhost")
	}
	return nil
}
