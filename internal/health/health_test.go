package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubPinger struct{ err error }

func (p stubPinger) PingContext(context.Context) error { return p.err }

func TestHealthRoutes(t *testing.T) {
	for _, tc := range []struct {
		path string
		err  error
		code int
		want string
	}{
		{"/health/live", errors.New("database down"), 200, `"status":"UP"`},
		{"/health/ready", nil, 200, `"status":"UP"`},
		{"/health/ready", errors.New("database down"), 503, `"code":"SERVICE_UNAVAILABLE"`},
	} {
		r := httptest.NewRecorder()
		Handler("api", stubPinger{tc.err}).ServeHTTP(r, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if r.Code != tc.code || !strings.Contains(r.Body.String(), tc.want) || r.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s got %d %s", tc.path, r.Code, r.Body.String())
		}
	}
}
