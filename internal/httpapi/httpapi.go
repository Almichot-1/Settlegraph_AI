package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/settlegraph/settlegraph/internal/config"
)

const demoOrg = "11111111-1111-4111-8111-111111111111"

type API struct {
	DB     *sql.DB
	Config config.Config
	Now    func() time.Time
}

func (a API) Handler() http.Handler {
	limiter := &rateLimiter{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/demo/sessions", a.session)
	mux.HandleFunc("GET /v1/me", a.me)
	mux.HandleFunc("GET /v1/members", a.members)
	mux.HandleFunc("GET /v1/providers", a.providers)
	mux.HandleFunc("GET /v1/accounts", a.accounts)
	mux.HandleFunc("POST /v1/integrations/banksim/events", a.ingestEvent)
	mux.HandleFunc("GET /v1/demo/scenarios", a.listDemoScenarios)
	mux.HandleFunc("POST /v1/demo/scenarios/{scenarioKey}/runs", a.createDemoScenario)
	mux.HandleFunc("GET /v1/demo/scenario-runs/{scenarioRunId}", a.getDemoScenario)
	mux.HandleFunc("POST /v1/demo/scenario-runs/{scenarioRunId}/advance", a.advanceDemoScenario)
	mux.HandleFunc("GET /v1/dashboard/summary", a.dashboardSummary)
	mux.HandleFunc("GET /v1/dashboard/series", a.dashboardSeries)
	mux.HandleFunc("GET /v1/orders", a.listOrders)
	mux.HandleFunc("GET /v1/orders/{id}", a.oneOrder)
	mux.HandleFunc("GET /v1/transactions", a.listTransactions)
	mux.HandleFunc("GET /v1/transactions/{id}", a.getTransaction)
	mux.HandleFunc("GET /v1/settlements", a.listSettlements)
	mux.HandleFunc("GET /v1/settlements/{id}", a.getSettlement)
	mux.HandleFunc("GET /v1/reconciliation/runs", a.listRuns)
	mux.HandleFunc("POST /v1/reconciliation/runs", a.createRun)
	mux.HandleFunc("GET /v1/reconciliation/runs/{runId}", a.oneRun)
	mux.HandleFunc("GET /v1/reconciliation/runs/{runId}/matches", a.listMatches)
	mux.HandleFunc("GET /v1/reconciliation/proofs/{id}", a.proof)
	mux.HandleFunc("GET /v1/jobs/{id}", a.getJob)
	mux.HandleFunc("GET /v1/audit-events", a.auditEvents)
	mux.HandleFunc("GET /v1/imports", a.listImports)
	mux.HandleFunc("GET /v1/imports/{id}", a.getImport)
	mux.HandleFunc("GET /v1/cases", a.cases)
	mux.HandleFunc("GET /v1/cases/{caseId}", a.oneCase)
	mux.HandleFunc("GET /v1/cases/{caseId}/evidence", a.caseEvidence)
	mux.HandleFunc("GET /v1/cases/{caseId}/events", a.caseEvents)
	mux.HandleFunc("POST /v1/cases/{caseId}/assign", a.assignCase)
	mux.HandleFunc("POST /v1/cases/{caseId}/decisions", a.decideCase)
	mux.HandleFunc("POST /v1/cases/{caseId}/investigations", a.createInvestigation)
	mux.HandleFunc("GET /v1/cases/{caseId}/investigations", a.listInvestigations)
	mux.HandleFunc("GET /v1/cases/{caseId}/investigations/{investigationId}", a.oneInvestigation)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if len(id) < 8 || len(id) > 128 || strings.ContainsAny(id, "\r\n ") {
			id = requestID()
		}
		w.Header().Set("X-Request-Id", id)
		defer func() {
			if recover() != nil {
				writeError(w, 500, "INTERNAL_ERROR", "Internal server error", id)
			}
		}()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(context.WithValue(ctx, requestIDKey{}, id))
		if !a.rateLimit(limiter, w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func itoa(v int) string { return strconv.Itoa(v) }

type requestIDKey struct{}

func requestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(b[:])
}
func reqID(r *http.Request) string { v, _ := r.Context().Value(requestIDKey{}).(string); return v }
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code, message, id string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": id, "details": map[string]any{}}})
}

func strictJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.ContentLength > 1<<20 {
		writeError(w, 413, "PAYLOAD_TOO_LARGE", "Request body is too large", reqID(r))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		status := 400
		code := "BAD_REQUEST"
		if strings.Contains(err.Error(), "http: request body too large") {
			status = 413
			code = "PAYLOAD_TOO_LARGE"
		}
		writeError(w, status, code, "Invalid JSON request", reqID(r))
		return false
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "BAD_REQUEST", "Request must contain one JSON object", reqID(r))
		return false
	}
	return true
}

type organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mode string `json:"mode"`
}
type membership struct {
	Organization organization `json:"organization"`
	Role         string       `json:"role"`
}
type userProfile struct {
	ID          string       `json:"id"`
	DisplayName string       `json:"display_name"`
	Memberships []membership `json:"memberships"`
}

func (a API) session(w http.ResponseWriter, r *http.Request) {
	if a.Config.AppMode != "demo" {
		writeError(w, 404, "NOT_FOUND", "Resource not found", reqID(r))
		return
	}
	code := r.Header.Get("X-Demo-Access-Code")
	subject := ""
	if a.Config.DemoAccessCode != "" && hmac.Equal([]byte(code), []byte(a.Config.DemoAccessCode)) {
		subject = "demo-owner"
	}
	for principal, allowedCode := range a.Config.DemoRoleCodes {
		if allowedCode != "" && hmac.Equal([]byte(code), []byte(allowedCode)) {
			subject = principal
		}
	}
	if subject == "" {
		writeError(w, 401, "UNAUTHORIZED", "Invalid demo access code", reqID(r))
		return
	}
	var body struct {
		DisplayName string `json:"display_name"`
	}
	if !strictJSON(w, r, &body) {
		return
	}
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.DisplayName == "" || len(body.DisplayName) > 80 {
		writeError(w, 400, "BAD_REQUEST", "display_name must contain 1 to 80 characters", reqID(r))
		return
	}
	// Demo principals are server-owned. A supplied name does not rename a
	// shared identity or change its authorization.
	profile, err := a.profile(r.Context(), subject)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Demo identity unavailable", reqID(r))
		return
	}
	now := a.now()
	token, expires := mintToken(a.Config.DemoJWTSecret, subject, now)
	writeJSON(w, 201, map[string]any{"access_token": token, "token_type": "Bearer", "expires_at": expires.UTC().Format(time.RFC3339), "user": profile, "default_organization_id": demoOrg})
}

func (a API) now() time.Time {
	if a.Now != nil {
		return a.Now().UTC()
	}
	return time.Now().UTC()
}
func (a API) subject(w http.ResponseWriter, r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		writeError(w, 401, "UNAUTHORIZED", "Bearer token required", reqID(r))
		return "", false
	}
	sub, err := verifyToken(a.Config.DemoJWTSecret, strings.TrimPrefix(auth, "Bearer "), a.now())
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Invalid or expired token", reqID(r))
		return "", false
	}
	return sub, true
}
func (a API) profile(ctx context.Context, subject string) (userProfile, error) {
	var p userProfile
	if err := a.DB.QueryRowContext(ctx, "SELECT id,display_name FROM app_users WHERE auth_subject=$1", subject).Scan(&p.ID, &p.DisplayName); err != nil {
		return p, err
	}
	p.Memberships = []membership{}
	rows, err := a.DB.QueryContext(ctx, `SELECT o.id,o.name,o.mode,m.role FROM memberships m JOIN organizations o ON o.id=m.organization_id JOIN app_users u ON u.id=m.user_id WHERE u.auth_subject=$1 AND m.active ORDER BY o.id`, subject)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var m membership
		if err := rows.Scan(&m.Organization.ID, &m.Organization.Name, &m.Organization.Mode, &m.Role); err != nil {
			return p, err
		}
		p.Memberships = append(p.Memberships, m)
	}
	return p, rows.Err()
}
func (a API) me(w http.ResponseWriter, r *http.Request) {
	sub, ok := a.subject(w, r)
	if !ok {
		return
	}
	p, err := a.profile(r.Context(), sub)
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Unknown user", reqID(r))
		return
	}
	writeJSON(w, 200, p)
}

func (a API) tenant(w http.ResponseWriter, r *http.Request, roles ...string) (string, bool) {
	sub, ok := a.subject(w, r)
	if !ok {
		return "", false
	}
	org := r.Header.Get("X-Organization-Id")
	if org == "" {
		writeError(w, 400, "BAD_REQUEST", "X-Organization-Id is required", reqID(r))
		return "", false
	}
	if len(org) != 36 {
		writeError(w, 400, "BAD_REQUEST", "Malformed organization ID", reqID(r))
		return "", false
	}
	var role string
	err := a.DB.QueryRowContext(r.Context(), `SELECT m.role FROM memberships m JOIN app_users u ON u.id=m.user_id WHERE m.organization_id=$1 AND u.auth_subject=$2 AND m.active`, org, sub).Scan(&role)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Resource not found", reqID(r))
		return "", false
	}
	if len(roles) > 0 {
		allowed := false
		for _, v := range roles {
			if v == role {
				allowed = true
			}
		}
		if !allowed {
			writeError(w, 403, "FORBIDDEN", "Insufficient role", reqID(r))
			return "", false
		}
	}
	return org, true
}
func (a API) providers(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	limit := 50
	cursor := ""
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var err error
		cursor, err = a.parseCursor(r, org, raw)
		if err != nil || len(cursor) == 0 || len(cursor) > 128 {
			writeError(w, 400, "BAD_REQUEST", "Invalid cursor", reqID(r))
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			writeError(w, 400, "BAD_REQUEST", "limit must be 1 to 100", reqID(r))
			return
		}
		limit = n
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,label,source_type,sync_status,last_observed_at FROM provider_sources WHERE organization_id=$1 AND id>$2 ORDER BY id LIMIT $3`, org, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Providers unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var id, name, kind, status string
		var last sql.NullTime
		if rows.Scan(&id, &name, &kind, &status, &last) != nil {
			writeError(w, 500, "INTERNAL_ERROR", "Provider read failed", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		var observed any
		if last.Valid {
			observed = last.Time.UTC().Format(time.RFC3339)
		}
		items = append(items, map[string]any{"id": id, "name": name, "kind": kind, "sync_status": status, "last_observed_at": observed})
	}
	if rows.Err() != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Provider read failed", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
