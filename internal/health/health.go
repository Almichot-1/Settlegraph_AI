package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Pinger interface {
	PingContext(context.Context) error
}

// Handler exposes only process and required-DB health. Optional cloud services
// never affect local readiness.
func Handler(service string, db Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "UP", "service": service})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if db == nil || db.PingContext(ctx) != nil {
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"error": map[string]any{"code": "SERVICE_UNAVAILABLE", "message": "Required dependency is unavailable", "request_id": "health-check", "details": map[string]any{}},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "UP", "service": service})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
