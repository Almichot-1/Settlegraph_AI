package httpapi

import (
	"github.com/settlegraph/settlegraph/internal/config"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimitWindowAndBoundedIdentities(t *testing.T) {
	l := &rateLimiter{}
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	if !l.allow("auth:user", 2, now) || !l.allow("auth:user", 2, now) || l.allow("auth:user", 2, now) {
		t.Fatal("burst limit not enforced")
	}
	if !l.allow("auth:user", 2, now.Add(time.Minute)) {
		t.Fatal("window did not expire")
	}
	for i := 0; i < 5000; i++ {
		l.allow(time.Duration(i).String(), 1, now)
	}
	if len(l.buckets) > 4096 {
		t.Fatal("unbounded rate identities")
	}
	if !l.allow("new", 1, now.Add(2*time.Minute)) {
		t.Fatal("old identities not reaped")
	}
}

func TestDemoLoginRateLimitHasRetryAfter(t *testing.T) {
	h := (API{Config: config.Config{AppMode: "demo", DemoAccessCode: "valid-demo-access-code", AuthRateLimit: 2}}).Handler()
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/v1/demo/sessions", strings.NewReader(`{"display_name":"test"}`))
		req.Header.Set("X-Demo-Access-Code", "invalid")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		want := 401
		if i == 2 {
			want = 429
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry delay")
			}
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
}
