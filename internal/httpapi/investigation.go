package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/settlegraph/settlegraph/internal/investigation"
	"net/http"
	"time"
)

func (a API) createInvestigation(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST")
	if !ok {
		return
	}
	actor, ok := a.subject(w, r)
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	if !demoUUID.MatchString(caseID) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case ID", reqID(r))
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		writeError(w, 400, "BAD_REQUEST", "Invalid Idempotency-Key", reqID(r))
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if !strictJSON(w, r, &body) {
		return
	}
	if body.Mode != "EVIDENCE_ANALYSIS" {
		writeError(w, 400, "BAD_REQUEST", "Unsupported investigation mode", reqID(r))
		return
	}
	hash := sha256.Sum256([]byte(caseID + ":" + body.Mode))
	digest := hex.EncodeToString(hash[:])
	const template = "/v1/cases/{caseId}/investigations"
	tx, err := a.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigation unavailable", reqID(r))
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org+":"+actor+":"+template+":"+key); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigation lock failed", reqID(r))
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
		var prior map[string]any
		if json.Unmarshal(oldBody, &prior) != nil {
			writeError(w, 500, "INTERNAL_ERROR", "Stored response invalid", reqID(r))
			return
		}
		if tx.Commit() != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Retry failed", reqID(r))
			return
		}
		writeJSON(w, 202, prior)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Retry lookup failed", reqID(r))
		return
	}
	var kind string
	var version int
	err = tx.QueryRowContext(r.Context(), "SELECT kind,version FROM investigation_cases WHERE organization_id=$1 AND id=$2 FOR UPDATE", org, caseID).Scan(&kind, &version)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Case not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case unavailable", reqID(r))
		return
	}
	rows, err := tx.QueryContext(r.Context(), "SELECT evidence_kind,evidence_id FROM case_evidence WHERE organization_id=$1 AND case_id=$2 ORDER BY evidence_id", org, caseID)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
		return
	}
	type evidence struct{ kind, id string }
	list := []evidence{}
	ids := []string{}
	for rows.Next() {
		var e evidence
		if err = rows.Scan(&e.kind, &e.id); err != nil {
			rows.Close()
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
			return
		}
		list = append(list, e)
		ids = append(ids, e.id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
		return
	}
	for _, e := range list {
		table := ""
		switch e.kind {
		case "ORDER":
			table = "merchant_orders"
		case "OBSERVATION":
			table = "observations"
		case "SOURCE_STATUS":
			table = "source_status_snapshots"
		default:
			writeError(w, 500, "INTERNAL_ERROR", "Unknown evidence kind", reqID(r))
			return
		}
		var exists bool
		if err = tx.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE organization_id=$1 AND id=$2)", org, e.id).Scan(&exists); err != nil || !exists {
			writeError(w, 500, "INTERNAL_ERROR", "Invalid evidence reference", reqID(r))
			return
		}
	}
	result := investigation.Offline(kind, ids)
	if investigation.ValidateResult(result, ids) != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Investigation evidence rejected", reqID(r))
		return
	}
	resultJSON, _ := json.Marshal(result)
	snapshot := investigation.SnapshotHash(ids)
	var investigationID string
	var created, completed time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO ai_investigations(organization_id,case_id,status,provider,evidence_snapshot_hash,result,completed_at) VALUES($1,$2,'COMPLETED','OFFLINE_STUB',$3,$4,now()) RETURNING id,created_at,completed_at`, org, caseID, snapshot, resultJSON).Scan(&investigationID, &created, &completed)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigation persistence failed", reqID(r))
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE investigation_cases SET version=version+1,updated_at=now() WHERE organization_id=$1 AND id=$2", org, caseID); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case update failed", reqID(r))
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO case_events(organization_id,case_id,version,action,actor,message) VALUES($1,$2,$3,'INVESTIGATION_COMPLETED',$4,'Offline evidence analysis completed')", org, caseID, version+1, actor); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case event failed", reqID(r))
		return
	}
	response := map[string]any{"id": investigationID, "case_id": caseID, "status": "COMPLETED", "provider": "OFFLINE_STUB", "created_at": created.UTC().Format(time.RFC3339), "completed_at": completed.UTC().Format(time.RFC3339), "evidence_snapshot_hash": snapshot, "result": result}
	responseJSON, _ := json.Marshal(response)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO idempotency_requests(organization_id,actor_sub,method,route_template,idempotency_key,request_sha256,response_status,response_body,processing_state) VALUES($1,$2,'POST',$3,$4,$5,202,$6,'COMPLETED')`, org, actor, template, key, digest, responseJSON)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Retry persistence failed", reqID(r))
		return
	}
	if tx.Commit() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigation commit failed", reqID(r))
		return
	}
	writeJSON(w, 202, response)
}

func scanInvestigation(rows interface{ Scan(...any) error }) (map[string]any, error) {
	var id, caseID, status, provider, hash string
	var created time.Time
	var completed sql.NullTime
	var resultJSON []byte
	if err := rows.Scan(&id, &caseID, &status, &provider, &hash, &created, &completed, &resultJSON); err != nil {
		return nil, err
	}
	var result any
	if len(resultJSON) > 0 {
		if json.Unmarshal(resultJSON, &result) != nil {
			return nil, errors.New("invalid stored result")
		}
	}
	var finished any
	if completed.Valid {
		finished = completed.Time.UTC().Format(time.RFC3339)
	}
	return map[string]any{"id": id, "case_id": caseID, "status": status, "provider": provider, "created_at": created.UTC().Format(time.RFC3339), "completed_at": finished, "evidence_snapshot_hash": hash, "result": result}, nil
}
func (a API) listInvestigations(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	if !demoUUID.MatchString(caseID) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case ID", reqID(r))
		return
	}
	var exists bool
	if a.DB.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM investigation_cases WHERE organization_id=$1 AND id=$2)", org, caseID).Scan(&exists) != nil || !exists {
		writeError(w, 404, "NOT_FOUND", "Case not found", reqID(r))
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id,case_id,status,provider,evidence_snapshot_hash,created_at,completed_at,result FROM ai_investigations WHERE organization_id=$1 AND case_id=$2 ORDER BY created_at,id", org, caseID)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigations unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, err := scanInvestigation(rows)
		if err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigations unavailable", reqID(r))
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigations unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nil})
}
func (a API) oneInvestigation(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	id := r.PathValue("investigationId")
	if !demoUUID.MatchString(caseID) || !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed ID", reqID(r))
		return
	}
	item, err := scanInvestigation(a.DB.QueryRowContext(r.Context(), "SELECT id,case_id,status,provider,evidence_snapshot_hash,created_at,completed_at,result FROM ai_investigations WHERE organization_id=$1 AND case_id=$2 AND id=$3", org, caseID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Investigation not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Investigation unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, item)
}
