package httpx

import (
	"net"
	"net/http"
	"strings"
)

// TrustedRealIP replaces chi's middleware.RealIP (SECURITY M1). X-Forwarded-For and
// X-Real-IP are honoured only when the TCP peer is one of the trusted proxy networks; the
// client address is then the right-most XFF entry that is not itself a trusted proxy.
// Requests from anyone else keep their socket address, so rate-limit keys and the IP
// stored with refresh tokens cannot be forged by sending headers.
func TrustedRealIP(cidrs []string) func(http.Handler) http.Handler {
	var nets []*net.IPNet
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(strings.TrimSpace(c)); err == nil {
			nets = append(nets, n)
		}
	}
	trusted := func(ip net.IP) bool {
		for _, n := range nets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(nets) > 0 {
				if ip := ClientIP(r, trusted); ip != "" {
					r.RemoteAddr = net.JoinHostPort(ip, "0")
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP resolves the client address behind trusted proxies ("" = keep RemoteAddr).
func ClientIP(r *http.Request, trusted func(net.IP) bool) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !trusted(peer) {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := net.ParseIP(strings.TrimSpace(parts[i]))
			if ip == nil {
				return ""
			}
			if !trusted(ip) {
				return ip.String()
			}
		}
		return ""
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return ""
}

// SecurityHeaders sets defensive headers on every API response (SECURITY L11).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if !strings.HasPrefix(r.URL.Path, "/playground") {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		if r.URL.Path == "/graphql" || strings.HasPrefix(r.URL.Path, "/webhooks/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
