package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func money(v int64) map[string]string {
	return map[string]string{"minor_units": strconv.FormatInt(v, 10), "currency": "ETB"}
}
func (a API) limitCursor(w http.ResponseWriter, r *http.Request, org string) (int, string, bool) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			writeError(w, 400, "BAD_REQUEST", "limit must be 1 to 100", reqID(r))
			return 0, "", false
		}
		limit = n
	}
	cursor := ""
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var err error
		cursor, err = a.parseCursor(r, org, raw)
		if err != nil || !demoUUID.MatchString(cursor) {
			writeError(w, 400, "BAD_REQUEST", "Invalid cursor", reqID(r))
			return 0, "", false
		}
	}
	return limit, cursor, true
}

func (a API) cursorSignature(r *http.Request, org, id string) []byte {
	query := r.URL.Query()
	query.Del("cursor")
	query.Del("limit")
	mac := hmac.New(sha256.New, []byte(a.Config.DemoJWTSecret))
	_, _ = mac.Write([]byte(org + "\n" + r.URL.Path + "\n" + query.Encode() + "\n" + id))
	return mac.Sum(nil)
}
func (a API) nextCursor(r *http.Request, org, id string) string {
	value := id + "." + hex.EncodeToString(a.cursorSignature(r, org, id))
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
func (a API) parseCursor(r *http.Request, org, raw string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 256 {
		return "", errors.New("invalid cursor")
	}
	id, signature, found := strings.Cut(string(data), ".")
	if !found || id == "" {
		return "", errors.New("invalid cursor")
	}
	provided, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(provided, a.cursorSignature(r, org, id)) {
		return "", errors.New("invalid cursor")
	}
	return id, nil
}

func (a API) dashboardSummary(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	var orders, payments, settlements, unmatched int64
	var open int
	queries := []struct {
		sql  string
		dest any
	}{
		{`SELECT COALESCE(sum(amount_minor),0) FROM merchant_orders WHERE organization_id=$1`, &orders},
		{`SELECT COALESCE(sum(amount_minor),0) FROM observations WHERE organization_id=$1 AND kind='PAYMENT_ACCEPTED'`, &payments},
		{`SELECT COALESCE(sum(amount_minor),0) FROM observations WHERE organization_id=$1 AND kind='SETTLEMENT_POSTED'`, &settlements},
		{`SELECT COALESCE(sum(p.amount_minor),0) FROM observations p WHERE p.organization_id=$1 AND p.kind='PAYMENT_ACCEPTED' AND NOT EXISTS(SELECT 1 FROM observations s WHERE s.organization_id=p.organization_id AND s.kind='SETTLEMENT_POSTED' AND s.transfer_reference=p.transfer_reference AND s.payment_reference=p.transfer_reference AND s.amount_minor=p.amount_minor AND s.currency=p.currency)`, &unmatched},
		{`SELECT count(*) FROM investigation_cases WHERE organization_id=$1 AND status<>'RESOLVED'`, &open},
	}
	for _, q := range queries {
		if a.DB.QueryRowContext(r.Context(), q.sql, org).Scan(q.dest) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Dashboard unavailable", reqID(r))
			return
		}
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,label,source_type,sync_status,last_observed_at FROM provider_sources WHERE organization_id=$1 ORDER BY id`, org)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Dashboard unavailable", reqID(r))
		return
	}
	defer rows.Close()
	providers := []map[string]any{}
	for rows.Next() {
		var id, name, kind, status string
		var last sql.NullTime
		if rows.Scan(&id, &name, &kind, &status, &last) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Dashboard unavailable", reqID(r))
			return
		}
		var observed any
		if last.Valid {
			observed = last.Time.UTC().Format(time.RFC3339)
		}
		providers = append(providers, map[string]any{"id": id, "name": name, "kind": kind, "sync_status": status, "last_observed_at": observed})
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Dashboard unavailable", reqID(r))
		return
	}
	var complete sql.NullTime
	var status sql.NullString
	var virtual sql.NullTime
	_ = a.DB.QueryRowContext(r.Context(), `SELECT source_status,complete_through FROM source_status_snapshots WHERE organization_id=$1 AND provider_id='BANK_B' ORDER BY observed_at DESC,id DESC LIMIT 1`, org).Scan(&status, &complete)
	_ = a.DB.QueryRowContext(r.Context(), `SELECT max(virtual_now) FROM scenario_runs WHERE organization_id=$1`, org).Scan(&virtual)
	isComplete := status.Valid && status.String == "HEALTHY" && complete.Valid && (!virtual.Valid || !complete.Time.Before(virtual.Time))
	coverage := "INCOMPLETE_SOURCE"
	if isComplete {
		coverage = "COMPLETE"
	} else if status.Valid && status.String == "STALE" {
		coverage = "STALE"
	}
	writeJSON(w, 200, map[string]any{"as_of": a.now().Format(time.RFC3339), "orders_expected": money(orders), "payments_accepted": money(payments), "settlements_observed": money(settlements), "unmatched_payments": money(unmatched), "open_cases": open, "providers": providers, "coverage": map[string]any{"source_status": coverage, "is_complete": isComplete}})
}

func scanCase(rows interface{ Scan(...any) error }) (map[string]any, string, error) {
	var id, kind, status, severity, title, summary string
	var version, evidenceCount int
	var created, updated time.Time
	var assigned, run sql.NullString
	err := rows.Scan(&id, &kind, &status, &severity, &title, &summary, &version, &created, &updated, &assigned, &run, &evidenceCount)
	if err != nil {
		return nil, "", err
	}
	var assignee, runID any
	if assigned.Valid {
		assignee = assigned.String
	}
	if run.Valid {
		runID = run.String
	}
	return map[string]any{"id": id, "kind": kind, "status": status, "severity": severity, "title": title, "summary": summary, "version": version, "created_at": created.UTC().Format(time.RFC3339), "updated_at": updated.UTC().Format(time.RFC3339), "assigned_to": assignee, "scenario_run_id": runID, "evidence_count": evidenceCount}, id, nil
}

const caseSelect = `SELECT c.id,c.kind,c.status,c.severity,c.title,c.summary,c.version,c.created_at,c.updated_at,c.assigned_to,c.scenario_run_id,(SELECT count(*) FROM case_evidence e WHERE e.organization_id=c.organization_id AND e.case_id=c.id) FROM investigation_cases c`

func (a API) cases(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "OPEN" && status != "EVIDENCE_UPDATED" && status != "RESOLVED" {
		writeError(w, 400, "BAD_REQUEST", "Invalid case status", reqID(r))
		return
	}
	assigned := r.URL.Query().Get("assigned_to")
	run := r.URL.Query().Get("scenario_run_id")
	if (assigned != "" && !demoUUID.MatchString(assigned)) || (run != "" && !demoUUID.MatchString(run)) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case filter ID", reqID(r))
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), caseSelect+` WHERE c.organization_id=$1 AND ($2='' OR c.id::text>$2)
 AND ($4='' OR c.status=$4) AND ($5='' OR c.assigned_to::text=$5)
 AND ($6='' OR c.scenario_run_id::text=$6) ORDER BY c.id LIMIT $3`, org, cursor, limit+1, status, assigned, run)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Cases unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		item, id, err := scanCase(rows)
		if err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Cases unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		items = append(items, item)
		_ = id
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Cases unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a API) oneCase(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("caseId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case ID", reqID(r))
		return
	}
	item, _, err := scanCase(a.DB.QueryRowContext(r.Context(), caseSelect+` WHERE c.organization_id=$1 AND c.id=$2`, org, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Case not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case unavailable", reqID(r))
		return
	}
	w.Header().Set("ETag", `"`+strconv.Itoa(item["version"].(int))+`"`)
	writeJSON(w, 200, item)
}
func (a API) caseEvidence(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("caseId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case ID", reqID(r))
		return
	}
	var exists bool
	if a.DB.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM investigation_cases WHERE organization_id=$1 AND id=$2)", org, id).Scan(&exists) != nil || !exists {
		writeError(w, 404, "NOT_FOUND", "Case not found", reqID(r))
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT evidence_kind,evidence_id FROM case_evidence WHERE organization_id=$1 AND case_id=$2 AND ($3='' OR evidence_id::text>$3) ORDER BY evidence_id LIMIT $4`, org, id, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var kind, evidenceID string
		if rows.Scan(&kind, &evidenceID) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		item, err := a.evidenceRecord(r, org, kind, evidenceID)
		if err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Evidence unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a API) evidenceRecord(r *http.Request, org, kind, id string) (map[string]any, error) {
	item := map[string]any{"id": id, "record_type": "SOURCE_STATUS", "reference": "", "amount": nil, "occurred_at": "", "provider_id": nil}
	switch kind {
	case "ORDER":
		var ref string
		var amount int64
		var at time.Time
		err := a.DB.QueryRowContext(r.Context(), "SELECT external_reference,amount_minor,created_at FROM merchant_orders WHERE organization_id=$1 AND id=$2", org, id).Scan(&ref, &amount, &at)
		if err != nil {
			return nil, err
		}
		item["record_type"] = "ORDER"
		item["reference"] = ref
		item["amount"] = money(amount)
		item["occurred_at"] = at.UTC().Format(time.RFC3339)
	case "OBSERVATION":
		var ref, provider, evKind, hash string
		var amount int64
		var at time.Time
		err := a.DB.QueryRowContext(r.Context(), `SELECT o.transfer_reference,o.provider_id,o.kind,o.amount_minor,o.occurred_at,i.payload_sha256 FROM observations o JOIN inbox_events i ON i.id=o.inbox_event_id WHERE o.organization_id=$1 AND o.id=$2`, org, id).Scan(&ref, &provider, &evKind, &amount, &at, &hash)
		if err != nil {
			return nil, err
		}
		item["record_type"] = "PAYMENT"
		if evKind == "SETTLEMENT_POSTED" {
			item["record_type"] = "SETTLEMENT"
		}
		item["reference"] = ref
		item["provider_id"] = provider
		item["amount"] = money(amount)
		item["occurred_at"] = at.UTC().Format(time.RFC3339)
		item["raw_payload_sha256"] = hash
	case "SOURCE_STATUS":
		var provider string
		var at time.Time
		err := a.DB.QueryRowContext(r.Context(), "SELECT provider_id,observed_at FROM source_status_snapshots WHERE organization_id=$1 AND id=$2", org, id).Scan(&provider, &at)
		if err != nil {
			return nil, err
		}
		item["reference"] = provider
		item["provider_id"] = provider
		item["occurred_at"] = at.UTC().Format(time.RFC3339)
	default:
		return nil, errors.New("unknown evidence kind")
	}
	return item, nil
}
func (a API) caseEvents(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("caseId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed case ID", reqID(r))
		return
	}
	var exists bool
	if a.DB.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM investigation_cases WHERE organization_id=$1 AND id=$2)", org, id).Scan(&exists) != nil || !exists {
		writeError(w, 404, "NOT_FOUND", "Case not found", reqID(r))
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,action,actor,message,version,occurred_at FROM case_events WHERE organization_id=$1 AND case_id=$2 ORDER BY version`, org, id)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case events unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var eventID, action, actor, message string
		var version int
		var at time.Time
		if rows.Scan(&eventID, &action, &actor, &message, &version, &at) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Case events unavailable", reqID(r))
			return
		}
		if action == "EVIDENCE_UPDATED" {
			action = "EVIDENCE_ADDED"
		}
		items = append(items, map[string]any{"id": eventID, "case_id": id, "type": action, "actor": actor, "message": message, "version": version, "occurred_at": at.UTC().Format(time.RFC3339)})
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Case events unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nil})
}
