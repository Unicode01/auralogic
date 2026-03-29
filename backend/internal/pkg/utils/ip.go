package utils

import (
	"net"
	"net/netip"
	"strings"

	"auralogic/internal/config"
	"github.com/gin-gonic/gin"
)

var (
	defaultTrustedProxyCIDRs = []string{"127.0.0.1/32", "::1/128"}
	defaultRealIPHeaders     = []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"}
)

// GetRealIP returns the best-effort "real client IP" for logging/rate-limit/captcha.
//
// Security note:
// - Forwarded headers are attacker-controlled unless requests come from trusted proxies.
// - We therefore default to the TCP peer IP and only trust a configured header when the peer is trusted.
func GetRealIP(c *gin.Context) string {
	cfg := config.GetConfig()
	peerIP := getPeerIP(c)

	if cfg != nil {
		// Only trust forwarded headers from trusted proxies.
		// If TrustedProxies is empty, default to local loopback proxies so local
		// reverse proxy setups keep working without trusting arbitrary peers.
		trustHeaders := isTrustedProxy(peerIP, effectiveTrustedProxies(cfg.Security.TrustedProxies))

		if trustHeaders {
			// If no explicit header configured, use a safe default order.
			headers := []string{cfg.Security.IPHeader}
			if strings.TrimSpace(cfg.Security.IPHeader) == "" {
				headers = defaultRealIPHeaders
			}

			for _, h := range headers {
				h = strings.TrimSpace(h)
				if h == "" {
					continue
				}
				if ip := firstIPFromHeader(c.GetHeader(h)); ip != "" {
					return ip
				}
			}
		}
	}

	return peerIP
}

func effectiveTrustedProxies(trusted []string) []string {
	normalized := make([]string, 0, len(trusted))
	for _, entry := range trusted {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		normalized = append(normalized, entry)
	}
	if len(normalized) > 0 {
		return normalized
	}
	return defaultTrustedProxyCIDRs
}

func getPeerIP(c *gin.Context) string {
	// RemoteAddr is the TCP peer address (proxy or client), not derived from headers.
	if host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr)); err == nil && host != "" {
		return host
	}
	// Fallback: Gin's parsing (may use headers depending on Gin config).
	return c.ClientIP()
}

func firstIPFromHeader(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.Contains(v, ",") {
		v = strings.TrimSpace(strings.Split(v, ",")[0])
	}
	if _, err := netip.ParseAddr(v); err != nil {
		return ""
	}
	return v
}

func isTrustedProxy(peerIP string, trusted []string) bool {
	if len(trusted) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(peerIP)
	if err != nil {
		return false
	}

	for _, entry := range trusted {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if p, err := netip.ParsePrefix(entry); err == nil {
			if p.Contains(addr) {
				return true
			}
			continue
		}
		if a, err := netip.ParseAddr(entry); err == nil {
			if a == addr {
				return true
			}
			continue
		}
	}

	return false
}
