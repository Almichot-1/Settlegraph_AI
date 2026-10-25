package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func (a API) orderRecord(r *http.Request, org, id string) (map[string]any, error) {
	var ref, currency string
	var amount int64
	var at time.Time
	var run sql.NullString
	err := a.DB.QueryRowContext(r.Context(), "SELECT external_reference,amount_minor,currency,created_at,scenario_run_id FROM merchant_orders WHERE organization_id=$1 AND id=$2", org, id).Scan(&ref, &amount, &currency, &at, &run)
	if err != nil {
		return nil, err
	}
	status := "OPEN"
	var payment, settlement bool
	var scopedRun any
	if run.Valid {
		scopedRun = run.String
	}
	err = a.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM observations p WHERE p.organization_id=$1 AND p.kind='PAYMENT_ACCEPTED' AND p.order_reference=$2 AND p.amount_minor=$3 AND p.scenario_run_id IS NOT DISTINCT FROM $4::uuid),EXISTS(SELECT 1 FROM observations s WHERE s.organization_id=$1 AND s.kind='SETTLEMENT_POSTED' AND s.order_reference=$2 AND s.amount_minor=$3 AND s.scenario_run_id IS NOT DISTINCT FROM $4::uuid)`, org, ref, amount, scopedRun).Scan(&payment, &settlement)
	if err != nil {
		return nil, err
	}
	if payment {
		status = "PAYMENT_OBSERVED"
	}
	if payment && settlement {
		status = "RECONCILED"
	}
	var runID any
	if run.Valid {
		runID = run.String
	}
	return map[string]any{"id": id, "external_reference": ref, "amount": map[string]string{"minor_units": strconv.FormatInt(amount, 10), "currency": currency}, "status": status, "created_at": at.UTC().Format(time.RFC3339), "scenario_run_id": runID}, nil
}
func (a API) listOrders(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id FROM merchant_orders WHERE organization_id=$1 AND ($2='' OR id::text>$2) ORDER BY id LIMIT $3", org, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Orders unavailable", reqID(r))
		return
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Orders unavailable", reqID(r))
			return
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Orders unavailable", reqID(r))
		return
	}
	rows.Close()
	var next any
	if len(ids) > limit {
		ids = ids[:limit]
		next = a.nextCursor(r, org, ids[len(ids)-1])
	}
	items := []map[string]any{}
	for _, id := range ids {
		item, err := a.orderRecord(r, org, id)
		if err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Orders unavailable", reqID(r))
			return
		}
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a API) oneOrder(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed order ID", reqID(r))
		return
	}
	item, err := a.orderRecord(r, org, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Order not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Order unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, item)
}

func (a API) observationRecord(r *http.Request, org, id, recordType string) (map[string]any, error) {
	var provider, ref, paymentRef, orderRef, kind, currency string
	var amount int64
	var occurred, observed time.Time
	var run sql.NullString
	err := a.DB.QueryRowContext(r.Context(), `SELECT provider_id,transfer_reference,COALESCE(payment_reference,''),COALESCE(order_reference,''),kind,amount_minor,currency,occurred_at,observed_at,scenario_run_id FROM observations WHERE organization_id=$1 AND id=$2`, org, id).Scan(&provider, &ref, &paymentRef, &orderRef, &kind, &amount, &currency, &occurred, &observed, &run)
	if err != nil {
		return nil, err
	}
	var runID any
	if run.Valid {
		runID = run.String
	}
	value := map[string]any{"id": id, "provider_id": provider, "external_reference": ref, "amount": map[string]string{"minor_units": strconv.FormatInt(amount, 10), "currency": currency}, "occurred_at": occurred.UTC().Format(time.RFC3339), "observed_at": observed.UTC().Format(time.RFC3339), "scenario_run_id": runID}
	if recordType == "settlement" {
		if kind != "SETTLEMENT_POSTED" {
			return nil, sql.ErrNoRows
		}
		value["payment_reference"] = paymentRef
	} else {
		if kind == "SETTLEMENT_POSTED" {
			return nil, sql.ErrNoRows
		}
		state := "ACCEPTED"
		if kind == "TRANSFER_FAILED" {
			state = "FAILED"
		}
		if kind == "REFUND_POSTED" {
			state = "REFUNDED"
		}
		value["state"] = state
		value["order_reference"] = orderRef
		value["source_attestation"] = "SIGNED_SIMULATED_PROVIDER"
	}
	return value, nil
}
func (a API) listObservations(w http.ResponseWriter, r *http.Request, recordType string) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	kindFilter := "SETTLEMENT_POSTED"
	operator := "="
	if recordType == "transaction" {
		operator = "<>"
	}
	provider := r.URL.Query().Get("provider_id")
	scenario := r.URL.Query().Get("scenario_run_id")
	if scenario != "" && !demoUUID.MatchString(scenario) {
		writeError(w, 400, "BAD_REQUEST", "Malformed scenario filter", reqID(r))
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id FROM observations WHERE organization_id=$1 AND kind"+operator+"$2 AND ($3='' OR id::text>$3) AND ($5='' OR provider_id=$5) AND ($6='' OR scenario_run_id::text=$6) ORDER BY id LIMIT $4", org, kindFilter, cursor, limit+1, provider, scenario)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Observations unavailable", reqID(r))
		return
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Observations unavailable", reqID(r))
			return
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Observations unavailable", reqID(r))
		return
	}
	rows.Close()
	var next any
	if len(ids) > limit {
		ids = ids[:limit]
		next = a.nextCursor(r, org, ids[len(ids)-1])
	}
	items := []map[string]any{}
	for _, id := range ids {
		item, err := a.observationRecord(r, org, id, recordType)
		if err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Observations unavailable", reqID(r))
			return
		}
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a API) oneObservation(w http.ResponseWriter, r *http.Request, recordType string) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed observation ID", reqID(r))
		return
	}
	item, err := a.observationRecord(r, org, id, recordType)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Observation not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Observation unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, item)
}
func (a API) listTransactions(w http.ResponseWriter, r *http.Request) {
	a.listObservations(w, r, "transaction")
}
func (a API) getTransaction(w http.ResponseWriter, r *http.Request) {
	a.oneObservation(w, r, "transaction")
}
func (a API) listSettlements(w http.ResponseWriter, r *http.Request) {
	a.listObservations(w, r, "settlement")
}
func (a API) getSettlement(w http.ResponseWriter, r *http.Request) {
	a.oneObservation(w, r, "settlement")
}

func scanRun(rows interface{ Scan(...any) error }) (map[string]any, error) {
	var id, status, scope, rules string
	var created time.Time
	var completed sql.NullTime
	var op, ps, unmatched int
	var scenario sql.NullString
	if err := rows.Scan(&id, &status, &scope, &rules, &created, &completed, &op, &ps, &unmatched, &scenario); err != nil {
		return nil, err
	}
	var done, scenarioID any
	if completed.Valid {
		done = completed.Time.UTC().Format(time.RFC3339)
	}
	if scenario.Valid {
		scenarioID = scenario.String
	}
	return map[string]any{"id": id, "status": status, "scope": scope, "rules_version": rules, "created_at": created.UTC().Format(time.RFC3339), "completed_at": done, "matched_order_payment": op, "matched_payment_settlement": ps, "unmatched_count": unmatched, "scenario_run_id": scenarioID}, nil
}

const runSelect = `SELECT id,status,scope,rules_version,created_at,completed_at,matched_order_payment,matched_payment_settlement,unmatched_count,scenario_run_id FROM reconciliation_runs`

func (a API) listRuns(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), runSelect+` WHERE organization_id=$1 AND ($2='' OR id::text>$2) ORDER BY id LIMIT $3`, org, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Runs unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		item, err := scanRun(rows)
		if err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Runs unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Runs unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a API) oneRun(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("runId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed run ID", reqID(r))
		return
	}
	item, err := scanRun(a.DB.QueryRowContext(r.Context(), runSelect+` WHERE organization_id=$1 AND id=$2`, org, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Run not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, item)
}
func (a API) listMatches(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	runID := r.PathValue("runId")
	if !demoUUID.MatchString(runID) {
		writeError(w, 400, "BAD_REQUEST", "Malformed run ID", reqID(r))
		return
	}
	var exists bool
	if a.DB.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM reconciliation_runs WHERE organization_id=$1 AND id=$2)", org, runID).Scan(&exists) != nil || !exists {
		writeError(w, 404, "NOT_FOUND", "Run not found", reqID(r))
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,stage,order_id,payment_observation_id,settlement_observation_id,allocated_minor FROM match_edges WHERE organization_id=$1 AND run_id=$2 AND ($3='' OR id::text>$3) ORDER BY id LIMIT $4`, org, runID, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Matches unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var id, stage, payment string
		var order, settlement sql.NullString
		var amount int64
		if rows.Scan(&id, &stage, &order, &payment, &settlement, &amount) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Matches unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		left, right, rule := payment, settlement.String, "EXACT_PROVIDER_REFERENCE_AND_AMOUNT"
		if stage == "ORDER_PAYMENT" {
			left, right, rule = order.String, payment, "EXACT_ORDER_REFERENCE_AND_AMOUNT"
		}
		items = append(items, map[string]any{"id": id, "run_id": runID, "stage": stage, "left_evidence_id": left, "right_evidence_id": right, "allocated_amount": money(amount), "rule": rule, "proof_id": id})
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Matches unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
func (a API) proof(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed proof ID", reqID(r))
		return
	}
	var runID, stage, payment string
	var order, settlement sql.NullString
	err := a.DB.QueryRowContext(r.Context(), "SELECT run_id,stage,order_id,payment_observation_id,settlement_observation_id FROM match_edges WHERE organization_id=$1 AND id=$2", org, id).Scan(&runID, &stage, &order, &payment, &settlement)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Proof not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Proof unavailable", reqID(r))
		return
	}
	evidence := []string{payment}
	if order.Valid {
		evidence = append(evidence, order.String)
	}
	if settlement.Valid {
		evidence = append(evidence, settlement.String)
	}
	writeJSON(w, 200, map[string]any{"id": id, "run_id": runID, "rule_version": "mvp-v1", "stage": stage, "checks": []string{"reference_equal", "amount_equal", "currency_equal", "unique_allocation"}, "source_evidence_ids": evidence, "result": "MATCHED_IN_IMPORTED_RECORDS"})
}
func (a API) getJob(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed job ID", reqID(r))
		return
	}
	var kind, status string
	var created time.Time
	var done sql.NullTime
	err := a.DB.QueryRowContext(r.Context(), "SELECT job_kind,status,created_at,completed_at FROM jobs WHERE organization_id=$1 AND id=$2", org, id).Scan(&kind, &status, &created, &done)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Job not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Job unavailable", reqID(r))
		return
	}
	var finished any
	if done.Valid {
		finished = done.Time.UTC().Format(time.RFC3339)
	}
	writeJSON(w, 200, map[string]any{"id": id, "kind": kind, "status": status, "created_at": created.UTC().Format(time.RFC3339), "finished_at": finished, "error_code": nil})
}
func (a API) auditEvents(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), "SELECT id,actor,action,resource_type,resource_id,occurred_at FROM audit_events WHERE organization_id=$1 AND ($2='' OR (occurred_at,id)<(SELECT occurred_at,id FROM audit_events WHERE organization_id=$1 AND id=NULLIF($2,'')::uuid)) ORDER BY occurred_at DESC,id DESC LIMIT $3", org, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Audit unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var id, actor, action, resourceType, resourceID string
		var at time.Time
		if rows.Scan(&id, &actor, &action, &resourceType, &resourceID, &at) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Audit unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		items = append(items, map[string]any{"id": id, "actor": actor, "action": action, "resource_type": resourceType, "resource_id": resourceID, "occurred_at": at.UTC().Format(time.RFC3339)})
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Audit unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
