package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func (a API) assignCase(w http.ResponseWriter, r *http.Request) { a.mutateCase(w, r, "assign") }
func (a API) decideCase(w http.ResponseWriter, r *http.Request) { a.mutateCase(w, r, "decision") }

func (a API) mutateCase(w http.ResponseWriter, r *http.Request, operation string) {
	roles := []string{"OWNER", "REVIEWER"}
	if operation == "assign" {
		roles = []string{"OWNER", "ANALYST"}
	}
	org, ok := a.tenant(w, r, roles...)
	if !ok {
		return
	}
	actor, ok := a.subject(w, r)
	if !ok {
		return
	}
	id := r.PathValue("caseId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case ID", reqID(r))
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		writeError(w, 400, "BAD_REQUEST", "Idempotency-Key must be 8 to 128 characters", reqID(r))
		return
	}
	match := r.Header.Get("If-Match")
	if match == "" {
		writeError(w, 428, "PRECONDITION_REQUIRED", "If-Match is required", reqID(r))
		return
	}
	if !strings.HasPrefix(match, `"`) || !strings.HasSuffix(match, `"`) {
		writeError(w, 400, "BAD_REQUEST", "If-Match must quote the version", reqID(r))
		return
	}
	if len(match) < 3 {
		writeError(w, 400, "BAD_REQUEST", "Invalid If-Match version", reqID(r))
		return
	}
	wantVersion, err := strconv.Atoi(match[1 : len(match)-1])
	if err != nil || wantVersion < 1 {
		writeError(w, 400, "BAD_REQUEST", "Invalid If-Match version", reqID(r))
		return
	}
	var assign struct {
		AssigneeID string `json:"assignee_id"`
	}
	var decision struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if operation == "assign" {
		if !strictJSON(w, r, &assign) {
			return
		}
		if !demoUUID.MatchString(assign.AssigneeID) {
			writeError(w, 400, "BAD_REQUEST", "Malformed assignee ID", reqID(r))
			return
		}
	} else {
		if !strictJSON(w, r, &decision) {
			return
		}
		decision.Reason = strings.TrimSpace(decision.Reason)
		if len(decision.Reason) < 5 || len(decision.Reason) > 2000 {
			writeError(w, 400, "BAD_REQUEST", "reason must contain 5 to 2000 characters", reqID(r))
			return
		}
		switch decision.Decision {
		case "RESOLVE", "REOPEN", "NEEDS_MORE_EVIDENCE":
		default:
			writeError(w, 400, "BAD_REQUEST", "Invalid decision", reqID(r))
			return
		}
	}
	var body []byte
	if operation == "assign" {
		body, _ = json.Marshal(assign)
	} else {
		body, _ = json.Marshal(decision)
	}
	hash := sha256.Sum256(append([]byte(id+":"), body...))
	digest := hex.EncodeToString(hash[:])
	template := "/v1/cases/{caseId}/" + map[string]string{"assign": "assign", "decision": "decisions"}[operation]
	tx, err := a.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case database unavailable", reqID(r))
		return
	}
	defer tx.Rollback()
	lock := org + ":" + actor + ":" + template + ":" + key
	if _, err = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", lock); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case lock unavailable", reqID(r))
		return
	}
	var priorHash string
	var priorBody []byte
	var priorStatus int
	err = tx.QueryRowContext(r.Context(), `SELECT request_sha256,response_status,response_body FROM idempotency_requests WHERE organization_id=$1 AND actor_sub=$2 AND method='POST' AND route_template=$3 AND idempotency_key=$4`, org, actor, template, key).Scan(&priorHash, &priorStatus, &priorBody)
	if err == nil {
		if priorHash != digest {
			writeError(w, 409, "IDEMPOTENCY_CONFLICT", "Key was used for a different request", reqID(r))
			return
		}
		var prior map[string]any
		if json.Unmarshal(priorBody, &prior) != nil {
			writeError(w, 500, "INTERNAL_ERROR", "Stored response invalid", reqID(r))
			return
		}
		if tx.Commit() != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Case retry unavailable", reqID(r))
			return
		}
		w.Header().Set("ETag", `"`+strconv.Itoa(int(prior["version"].(float64)))+`"`)
		writeJSON(w, priorStatus, prior)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case retry unavailable", reqID(r))
		return
	}
	var current int
	var status string
	err = tx.QueryRowContext(r.Context(), "SELECT version,status FROM investigation_cases WHERE organization_id=$1 AND id=$2 FOR UPDATE", org, id).Scan(&current, &status)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Case not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case unavailable", reqID(r))
		return
	}
	if current != wantVersion {
		writeError(w, 412, "PRECONDITION_FAILED", "Case version changed", reqID(r))
		return
	}
	if operation == "decision" && ((decision.Decision == "RESOLVE" && status == "RESOLVED") || (decision.Decision == "REOPEN" && status != "RESOLVED")) {
		writeError(w, 409, "CONFLICT", "Invalid case transition", reqID(r))
		return
	}
	message := ""
	action := ""
	if operation == "assign" {
		var member bool
		if err = tx.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM memberships WHERE organization_id=$1 AND user_id=$2 AND active AND role IN ('OWNER','ANALYST','REVIEWER'))", org, assign.AssigneeID).Scan(&member); err != nil || !member {
			writeError(w, 400, "BAD_REQUEST", "Assignee is not an active reviewer", reqID(r))
			return
		}
		_, err = tx.ExecContext(r.Context(), "UPDATE investigation_cases SET assigned_to=$3,version=version+1,updated_at=now() WHERE organization_id=$1 AND id=$2", org, id, assign.AssigneeID)
		message = "Assigned to " + assign.AssigneeID
		action = "ASSIGNED"
	} else {
		newStatus := status
		switch decision.Decision {
		case "RESOLVE":
			newStatus = "RESOLVED"
			action = "RESOLVED"
		case "REOPEN":
			newStatus = "OPEN"
			action = "DECISION_RECORDED"
		case "NEEDS_MORE_EVIDENCE":
			newStatus = "OPEN"
			action = "DECISION_RECORDED"
		}
		_, err = tx.ExecContext(r.Context(), "UPDATE investigation_cases SET status=$3,version=version+1,updated_at=now() WHERE organization_id=$1 AND id=$2", org, id, newStatus)
		message = decision.Decision + ": " + decision.Reason
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case update failed", reqID(r))
		return
	}
	_, err = tx.ExecContext(r.Context(), "INSERT INTO case_events(organization_id,case_id,version,action,actor,message) VALUES($1,$2,$3,$4,$5,$6)", org, id, current+1, action, actor, message)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case timeline failed", reqID(r))
		return
	}
	audit, _ := json.Marshal(map[string]any{"operation": operation, "version": current + 1})
	_, err = tx.ExecContext(r.Context(), "INSERT INTO audit_events(organization_id,actor,action,resource_type,resource_id,payload) VALUES($1,$2,$3,'CASE',$4,$5)", org, actor, action, id, audit)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case audit failed", reqID(r))
		return
	}
	item, _, err := scanCase(tx.QueryRowContext(r.Context(), caseSelect+` WHERE c.organization_id=$1 AND c.id=$2`, org, id))
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case response unavailable", reqID(r))
		return
	}
	response, _ := json.Marshal(item)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO idempotency_requests(organization_id,actor_sub,method,route_template,idempotency_key,request_sha256,response_status,response_body,processing_state)
 VALUES($1,$2,'POST',$3,$4,$5,200,$6,'COMPLETED')`, org, actor, template, key, digest, response)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case retry persistence failed", reqID(r))
		return
	}
	if tx.Commit() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case commit failed", reqID(r))
		return
	}
	w.Header().Set("ETag", `"`+strconv.Itoa(current+1)+`"`)
	writeJSON(w, 200, item)
}
