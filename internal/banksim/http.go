package banksim

import (
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/settlegraph/settlegraph/internal/config"
)

type HTTP struct {
	Ledger Ledger
	Config config.Config
	Now    func() time.Time
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (h HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /provider/v1/transfers", h.transfer)
	mux.HandleFunc("GET /provider/v1/transfers/{id}", h.getTransfer)
	mux.HandleFunc("GET /provider/v1/institutions", h.institutions)
	mux.HandleFunc("GET /provider/v1/institutions/{institutionId}/events", h.events)
	mux.HandleFunc("GET /admin/v1/scenarios", h.scenarios)
	mux.HandleFunc("POST /admin/v1/scenarios/{scenarioKey}/runs", h.startScenario)
	mux.HandleFunc("GET /admin/v1/runs/{runId}", h.getScenario)
	mux.HandleFunc("POST /admin/v1/runs/{runId}/advance", h.advanceScenario)
	mux.HandleFunc("GET /evaluator/v1/runs/{runId}/truth", h.truth)
	return mux
}

func (h HTTP) truth(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimEvaluatorToken) {
		return
	}
	id := r.PathValue("runId")
	if !uuidPattern.MatchString(id) {
		h.err(w, 400, "BAD_REQUEST", "Malformed run ID")
		return
	}
	var seed int
	var expectedPayments, expectedSettlements int
	var unbalanced int
	err := h.Ledger.DB.QueryRowContext(r.Context(), "SELECT seed FROM sim_scenarios WHERE id=$1", id).Scan(&seed)
	if errors.Is(err, sql.ErrNoRows) {
		h.err(w, 404, "NOT_FOUND", "Run not found")
		return
	}
	if err != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Truth unavailable")
		return
	}
	if h.Ledger.DB.QueryRowContext(r.Context(), "SELECT count(*),count(*) FILTER (WHERE state='SETTLED') FROM sim_transfers WHERE scenario_id=$1", id).Scan(&expectedPayments, &expectedSettlements) != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Truth unavailable")
		return
	}
	if h.Ledger.DB.QueryRowContext(r.Context(), `SELECT count(*) FROM (SELECT j.id FROM sim_journals j JOIN sim_transfers t ON t.id=j.transfer_id JOIN sim_entries e ON e.journal_id=j.id WHERE t.scenario_id=$1 GROUP BY j.id HAVING sum(e.signed_minor)<>0) bad`, id).Scan(&unbalanced) != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Truth unavailable")
		return
	}
	h.write(w, 200, map[string]any{"scenario_run_id": id, "seed": seed, "expected_payments": expectedPayments, "expected_settlements": expectedSettlements, "source_ledger_balanced": unbalanced == 0})
}
func (h HTTP) authorized(w http.ResponseWriter, r *http.Request, token string) bool {
	auth := r.Header.Get("Authorization")
	if token == "" || !strings.HasPrefix(auth, "Bearer ") || !hmac.Equal([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(token)) {
		h.err(w, 401, "UNAUTHORIZED", "Valid service token required")
		return false
	}
	return true
}
func (h HTTP) err(w http.ResponseWriter, status int, code, message string) {
	h.write(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": "banksim", "details": map[string]any{}}})
}
func (h HTTP) write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (h HTTP) transfer(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimAdminToken) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		h.err(w, 400, "BAD_REQUEST", "Idempotency-Key must be 8 to 128 characters")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body CreateRequest
	if decoder.Decode(&body) != nil {
		h.err(w, 400, "BAD_REQUEST", "Invalid transfer request")
		return
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		h.err(w, 400, "BAD_REQUEST", "One JSON object required")
		return
	}
	if !uuidPattern.MatchString(body.FromAccountID) || !uuidPattern.MatchString(body.ToAccountID) {
		h.err(w, 400, "BAD_REQUEST", "Malformed account ID")
		return
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	t, err := h.Ledger.Accept(r.Context(), body, key, "", now)
	if err != nil {
		switch {
		case errors.Is(err, ErrIdempotencyConflict):
			h.err(w, 409, "IDEMPOTENCY_CONFLICT", err.Error())
		case errors.Is(err, ErrInsufficientFunds):
			h.err(w, 409, "INSUFFICIENT_FUNDS", err.Error())
		default:
			h.err(w, 400, "BAD_REQUEST", err.Error())
		}
		return
	}
	h.write(w, 201, t)
}
func (h HTTP) getTransfer(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimProviderToken) {
		return
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		h.err(w, 400, "BAD_REQUEST", "Malformed transfer ID")
		return
	}
	var t Transfer
	var amount int64
	err := h.Ledger.DB.QueryRowContext(r.Context(), "SELECT id,from_account_id,to_account_id,amount_minor,state,transfer_reference,created_at FROM sim_transfers WHERE id=$1", id).Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &amount, &t.State, &t.Reference, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		h.err(w, 404, "NOT_FOUND", "Transfer not found")
		return
	}
	if err != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Transfer unavailable")
		return
	}
	t.Amount = Money{MinorUnits: strconv.FormatInt(amount, 10), Currency: "ETB"}
	h.write(w, 200, t)
}
func (h HTTP) institutions(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimProviderToken) {
		return
	}
	rows, err := h.Ledger.DB.QueryContext(r.Context(), "SELECT id,name,kind FROM sim_institutions ORDER BY id")
	if err != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Institutions unavailable")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, kind string
		if rows.Scan(&id, &name, &kind) != nil {
			h.err(w, 500, "INTERNAL_ERROR", "Institution read failed")
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "kind": kind})
	}
	if rows.Err() != nil {
		h.err(w, 500, "INTERNAL_ERROR", "Institution read failed")
		return
	}
	h.write(w, 200, map[string]any{"items": items})
}

func (h HTTP) scenarios(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimAdminToken) {
		return
	}
	h.write(w, 200, map[string]any{"items": []map[string]any{
		{"key": "normal_flow", "title": "Normal synthetic payment", "description": "Payment and settlement are observed", "steps": 2},
		{"key": "delayed_settlement", "title": "Delayed synthetic settlement", "description": "Settlement follows the configured virtual grace period", "steps": 3},
		{"key": "duplicate_webhook", "title": "Duplicate notification replay", "description": "A provider notification is delivered twice", "steps": 2},
		{"key": "provider_outage", "title": "Provider outage and recovery", "description": "Settlement visibility is delayed until recovery", "steps": 3},
	}})
}
func (h HTTP) startScenario(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimAdminToken) {
		return
	}
	key := r.PathValue("scenarioKey")
	if scenarioSteps(key) == 0 {
		h.err(w, 404, "NOT_FOUND", "Scenario not found")
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if len(idem) < 8 || len(idem) > 128 {
		h.err(w, 400, "BAD_REQUEST", "Idempotency-Key must be 8 to 128 characters")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var body struct {
		Seed           int    `json:"seed"`
		OrganizationID string `json:"organization_id"`
	}
	if d.Decode(&body) != nil || d.Decode(&struct{}{}) != io.EOF || !uuidPattern.MatchString(body.OrganizationID) {
		h.err(w, 400, "BAD_REQUEST", "Invalid scenario request")
		return
	}
	run, err := h.Ledger.StartScenario(r.Context(), key, body.OrganizationID, body.Seed, idem)
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) {
			h.err(w, 409, "IDEMPOTENCY_CONFLICT", err.Error())
		} else {
			h.err(w, 400, "BAD_REQUEST", err.Error())
		}
		return
	}
	h.write(w, 201, run)
}
func (h HTTP) getScenario(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimAdminToken) {
		return
	}
	id := r.PathValue("runId")
	if !uuidPattern.MatchString(id) {
		h.err(w, 400, "BAD_REQUEST", "Malformed run ID")
		return
	}
	run, err := h.Ledger.GetScenario(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		h.err(w, 404, "NOT_FOUND", "Scenario run not found")
		return
	}
	if err != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Scenario unavailable")
		return
	}
	h.write(w, 200, run)
}
func (h HTTP) advanceScenario(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimAdminToken) {
		return
	}
	id := r.PathValue("runId")
	if !uuidPattern.MatchString(id) {
		h.err(w, 400, "BAD_REQUEST", "Malformed run ID")
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if len(idem) < 8 || len(idem) > 128 {
		h.err(w, 400, "BAD_REQUEST", "Idempotency-Key must be 8 to 128 characters")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var body struct {
		ExpectedStep int `json:"expected_step"`
	}
	if d.Decode(&body) != nil || d.Decode(&struct{}{}) != io.EOF {
		h.err(w, 400, "BAD_REQUEST", "Invalid advance request")
		return
	}
	run, err := h.Ledger.AdvanceScenario(r.Context(), id, body.ExpectedStep, idem)
	if err != nil {
		switch {
		case errors.Is(err, ErrStaleStep):
			h.err(w, 412, "PRECONDITION_FAILED", err.Error())
		case errors.Is(err, sql.ErrNoRows):
			h.err(w, 404, "NOT_FOUND", "Scenario run not found")
		case errors.Is(err, ErrIdempotencyConflict):
			h.err(w, 409, "IDEMPOTENCY_CONFLICT", err.Error())
		default:
			h.err(w, 400, "BAD_REQUEST", err.Error())
		}
		return
	}
	if err := h.DeliverPending(r.Context(), run.ID); err != nil {
		slog.Warn("provider event delivery deferred", "run_id", run.ID, "error", err)
	}
	h.write(w, 200, run)
}
func (h HTTP) events(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r, h.Config.BankSimProviderToken) {
		return
	}
	provider := r.PathValue("institutionId")
	if provider != "BANK_A" && provider != "BANK_B" {
		h.err(w, 404, "NOT_FOUND", "Institution not found")
		return
	}
	cursor := int64(0)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			h.err(w, 400, "BAD_REQUEST", "Invalid cursor")
			return
		}
		cursor = n
	}
	scenarioID := r.URL.Query().Get("scenario_run_id")
	if scenarioID != "" && !uuidPattern.MatchString(scenarioID) {
		h.err(w, 400, "BAD_REQUEST", "Malformed scenario_run_id")
		return
	}
	query := "SELECT sequence,payload FROM sim_events WHERE provider_id=$1 AND sequence>$2 AND delivery_status<>'DELAYED' ORDER BY sequence LIMIT 101"
	args := []any{provider, cursor}
	if scenarioID != "" {
		query = "SELECT sequence,payload FROM sim_events WHERE provider_id=$1 AND sequence>$2 AND scenario_id=$3 AND delivery_status<>'DELAYED' ORDER BY sequence LIMIT 101"
		args = append(args, scenarioID)
	}
	rows, err := h.Ledger.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.err(w, 503, "SERVICE_UNAVAILABLE", "Events unavailable")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var seq int64
		var raw []byte
		if rows.Scan(&seq, &raw) != nil {
			h.err(w, 500, "INTERNAL_ERROR", "Event read failed")
			return
		}
		if len(items) == 100 {
			next = strconv.FormatInt(cursor, 10)
			break
		}
		var event map[string]any
		if json.Unmarshal(raw, &event) != nil {
			h.err(w, 500, "INTERNAL_ERROR", "Invalid stored event")
			return
		}
		event["type"] = event["event_type"]
		delete(event, "event_type")
		delete(event, "payload_version")
		items = append(items, event)
		cursor = seq
	}
	if rows.Err() != nil {
		h.err(w, 500, "INTERNAL_ERROR", "Event read failed")
		return
	}
	var complete sql.NullTime
	sourceStatus := "HEALTHY"
	if scenarioID == "" {
		_ = h.Ledger.DB.QueryRowContext(r.Context(), "SELECT max(virtual_now) FROM sim_scenarios").Scan(&complete)
	} else {
		var key string
		var step int
		err = h.Ledger.DB.QueryRowContext(r.Context(), "SELECT scenario_key,current_step,virtual_now FROM sim_scenarios WHERE id=$1", scenarioID).Scan(&key, &step, &complete)
		if errors.Is(err, sql.ErrNoRows) {
			h.err(w, 404, "NOT_FOUND", "Scenario not found")
			return
		}
		if err != nil {
			h.err(w, 503, "SERVICE_UNAVAILABLE", "Source watermark unavailable")
			return
		}
		if provider == "BANK_B" && key == "provider_outage" && step == 2 {
			sourceStatus = "OUTAGE"
			complete.Time = complete.Time.Add(-2 * time.Hour)
		}
	}
	var completeThrough any
	if complete.Valid {
		completeThrough = complete.Time.UTC().Format(time.RFC3339)
	}
	h.write(w, 200, map[string]any{"items": items, "next_cursor": next, "source_status": sourceStatus, "complete_through": completeThrough})
}
