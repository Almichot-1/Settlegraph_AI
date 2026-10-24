package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

func (a API) createRun(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST")
	if !ok {
		return
	}
	actor, ok := a.subject(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		writeError(w, 400, "BAD_REQUEST", "Invalid Idempotency-Key", reqID(r))
		return
	}
	var body struct {
		Scope         string  `json:"scope"`
		ScenarioRunID *string `json:"scenario_run_id"`
		RulesVersion  string  `json:"rules_version"`
	}
	if !strictJSON(w, r, &body) {
		return
	}
	if body.Scope != "SCENARIO_RUN" && body.Scope != "LATEST_OBSERVATIONS" {
		writeError(w, 400, "BAD_REQUEST", "Invalid reconciliation scope", reqID(r))
		return
	}
	if body.Scope == "SCENARIO_RUN" && (body.ScenarioRunID == nil || !demoUUID.MatchString(*body.ScenarioRunID)) {
		writeError(w, 400, "BAD_REQUEST", "scenario_run_id required", reqID(r))
		return
	}
	if body.RulesVersion != "" && body.RulesVersion != "v1" && body.RulesVersion != "mvp-v1" {
		writeError(w, 400, "BAD_REQUEST", "Unsupported rules version", reqID(r))
		return
	}
	data, _ := json.Marshal(body)
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	const template = "/v1/reconciliation/runs"
	tx, err := a.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Reconciliation unavailable", reqID(r))
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org+":"+actor+":"+template+":"+key); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Reconciliation lock failed", reqID(r))
		return
	}
	var oldHash string
	var oldBody []byte
	err = tx.QueryRowContext(r.Context(), `SELECT request_sha256,response_body FROM idempotency_requests WHERE organization_id=$1 AND actor_sub=$2 AND method='POST' AND route_template=$3 AND idempotency_key=$4`, org, actor, template, key).Scan(&oldHash, &oldBody)
	if err == nil {
		if oldHash != digest {
			writeError(w, 409, "IDEMPOTENCY_CONFLICT", "Key used for another request", reqID(r))
			return
		}
		var old map[string]any
		if json.Unmarshal(oldBody, &old) != nil {
			writeError(w, 500, "INTERNAL_ERROR", "Stored response invalid", reqID(r))
			return
		}
		if tx.Commit() != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Retry failed", reqID(r))
			return
		}
		writeJSON(w, 202, old)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Retry lookup failed", reqID(r))
		return
	}
	var scenario string
	var virtual time.Time
	if body.ScenarioRunID != nil {
		scenario = *body.ScenarioRunID
		err = tx.QueryRowContext(r.Context(), "SELECT virtual_now FROM scenario_runs WHERE organization_id=$1 AND id=$2", org, scenario).Scan(&virtual)
	} else {
		err = tx.QueryRowContext(r.Context(), "SELECT id,virtual_now FROM scenario_runs WHERE organization_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1", org).Scan(&scenario, &virtual)
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Scenario run not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Scenario unavailable", reqID(r))
		return
	}
	var cutoff time.Time
	if err = tx.QueryRowContext(r.Context(), "SELECT clock_timestamp()").Scan(&cutoff); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Snapshot clock unavailable", reqID(r))
		return
	}
	jobKey := "manual:" + actor + ":" + key
	var runID string
	var created time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO reconciliation_runs(organization_id,scenario_run_id,scope,rules_version,status,snapshot_cutoff,job_key) VALUES($1,$2,$3,'mvp-v1','QUEUED',$4,$5) RETURNING id,created_at`, org, scenario, body.Scope, cutoff, jobKey).Scan(&runID, &created)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run creation failed", reqID(r))
		return
	}
	payload, _ := json.Marshal(map[string]any{"scenario_run_id": scenario, "virtual_now": virtual, "cutoff": cutoff})
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO jobs(organization_id,job_kind,job_key,payload) VALUES($1,'RECONCILE',$2,$3)", org, jobKey, payload); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run queue failed", reqID(r))
		return
	}
	response := map[string]any{"id": runID, "status": "QUEUED", "scope": body.Scope, "rules_version": "mvp-v1", "created_at": created.UTC().Format(time.RFC3339), "completed_at": nil, "matched_order_payment": 0, "matched_payment_settlement": 0, "unmatched_count": 0, "scenario_run_id": scenario}
	responseJSON, _ := json.Marshal(response)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO idempotency_requests(organization_id,actor_sub,method,route_template,idempotency_key,request_sha256,response_status,response_body,processing_state) VALUES($1,$2,'POST',$3,$4,$5,202,$6,'COMPLETED')`, org, actor, template, key, digest, responseJSON)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Retry persistence failed", reqID(r))
		return
	}
	if tx.Commit() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run commit failed", reqID(r))
		return
	}
	writeJSON(w, 202, response)
}
