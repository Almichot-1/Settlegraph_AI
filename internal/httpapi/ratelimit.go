package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	start time.Time
	count int
}
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
}

func (l *rateLimiter) allow(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = map[string]rateBucket{}
	}
	if len(l.buckets) >= 4096 {
		for k, b := range l.buckets {
			if now.Sub(b.start) >= time.Minute {
				delete(l.buckets, k)
			}
		}
		if _, ok := l.buckets[key]; !ok && len(l.buckets) >= 4096 {
			return false
		}
	}
	b := l.buckets[key]
	if b.start.IsZero() || now.Sub(b.start) >= time.Minute {
		b = rateBucket{start: now}
	}
	if b.count >= limit {
		return false
	}
	b.count++
	l.buckets[key] = b
	return true
}
func (a API) rateLimit(l *rateLimiter, w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "POST" || r.URL.Path == "/v1/integrations/banksim/events" {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	category, limit := "mutation", a.Config.ExpensiveRateLimit
	if limit <= 0 {
		limit = 120
	}
	if r.URL.Path == "/v1/demo/sessions" {
		category = "auth"
		limit = a.Config.AuthRateLimit
		if limit <= 0 {
			limit = 30
		}
	} else {
		if token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); token != "" {
			if sub, err := verifyToken(a.Config.DemoJWTSecret, token, a.now()); err == nil {
				host = sub
			}
		}
	}
	if l.allow(category+":"+host, limit, a.now()) {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(60))
	writeError(w, 429, "RATE_LIMITED", "Too many requests; retry after the indicated delay", reqID(r))
	return false
}
