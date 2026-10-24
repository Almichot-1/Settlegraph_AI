package reconcile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Summary struct {
	RunID             string
	OrderPayment      int
	PaymentSettlement int
	Unmatched         int
}

// Run derives an immutable reconciliation snapshot using only the product DB.
// BankSim's private ledger is never queried here.
func Run(ctx context.Context, db *sql.DB, org, scenarioID, jobKey string, virtualNow, cutoff time.Time) (Summary, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "reconcile:"+scenarioID); err != nil {
		return Summary{}, err
	}
	var existing Summary
	var existingStatus string
	err = tx.QueryRowContext(ctx, "SELECT id,status,matched_order_payment,matched_payment_settlement,unmatched_count FROM reconciliation_runs WHERE organization_id=$1 AND job_key=$2", org, jobKey).Scan(&existing.RunID, &existingStatus, &existing.OrderPayment, &existing.PaymentSettlement, &existing.Unmatched)
	if err == nil && existingStatus == "COMPLETED" {
		return existing, tx.Commit()
	}
	if err == nil && existingStatus != "QUEUED" {
		return Summary{}, errors.New("reconciliation run is not claimable")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Summary{}, err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM scenario_runs WHERE id=$1 AND organization_id=$2)", scenarioID, org).Scan(&exists); err != nil {
		return Summary{}, err
	}
	if !exists {
		return Summary{}, sql.ErrNoRows
	}
	orders := []Order{}
	rows, err := tx.QueryContext(ctx, "SELECT id,external_reference,amount_minor,currency FROM merchant_orders WHERE organization_id=$1 AND scenario_run_id=$2 ORDER BY id", org, scenarioID)
	if err != nil {
		return Summary{}, err
	}
	for rows.Next() {
		var o Order
		if err = rows.Scan(&o.ID, &o.Reference, &o.Amount, &o.Currency); err != nil {
			rows.Close()
			return Summary{}, err
		}
		orders = append(orders, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Summary{}, err
	}
	events := []Observation{}
	rows, err = tx.QueryContext(ctx, `SELECT id,kind,transfer_reference,COALESCE(payment_reference,''),COALESCE(order_reference,''),amount_minor,currency,occurred_at FROM observations WHERE organization_id=$1 AND scenario_run_id=$2 AND observed_at<=$3 ORDER BY occurred_at,source_sequence,id`, org, scenarioID, cutoff)
	if err != nil {
		return Summary{}, err
	}
	for rows.Next() {
		var o Observation
		if err = rows.Scan(&o.ID, &o.Kind, &o.TransferReference, &o.PaymentReference, &o.OrderReference, &o.Amount, &o.Currency, &o.OccurredAt); err != nil {
			rows.Close()
			return Summary{}, err
		}
		events = append(events, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Summary{}, err
	}
	var coverageID string
	var complete sql.NullTime
	var sourceStatus string
	err = tx.QueryRowContext(ctx, `SELECT id,complete_through,source_status FROM source_status_snapshots WHERE organization_id=$1 AND scenario_run_id=$2 AND provider_id='BANK_B' AND observed_at<=$3 ORDER BY observed_at DESC,id DESC LIMIT 1`, org, scenarioID, cutoff).Scan(&coverageID, &complete, &sourceStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Summary{}, err
	}
	result := Evaluate(orders, events, virtualNow, complete.Time, complete.Valid && sourceStatus == "HEALTHY", time.Hour)
	var summary Summary
	if existingStatus == "QUEUED" {
		summary.RunID = existing.RunID
		_, err = tx.ExecContext(ctx, "UPDATE reconciliation_runs SET status='RUNNING',snapshot_cutoff=$2 WHERE id=$1", summary.RunID, cutoff)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO reconciliation_runs(organization_id,scenario_run_id,scope,rules_version,status,snapshot_cutoff,job_key)
	 VALUES($1,$2,'SCENARIO_RUN','mvp-v1','RUNNING',$3,$4) RETURNING id`, org, scenarioID, cutoff, jobKey).Scan(&summary.RunID)
	}
	if err != nil {
		return Summary{}, err
	}
	eventByID := map[string]Observation{}
	for _, e := range events {
		eventByID[e.ID] = e
	}
	for _, pair := range result.OrderPayment {
		proof, _ := json.Marshal(map[string]any{"rule": "EXACT_ORDER_REFERENCE_AMOUNT_CURRENCY", "order_id": pair.Left, "payment_observation_id": pair.Right})
		_, err = tx.ExecContext(ctx, `INSERT INTO match_edges(organization_id,run_id,stage,order_id,payment_observation_id,allocated_minor,rule,proof)
 VALUES($1,$2,'ORDER_PAYMENT',$3,$4,$5,'EXACT_ORDER_REFERENCE_AMOUNT_CURRENCY',$6)`, org, summary.RunID, pair.Left, pair.Right, eventByID[pair.Right].Amount, proof)
		if err != nil {
			return Summary{}, err
		}
	}
	for _, pair := range result.PaymentSettlement {
		proof, _ := json.Marshal(map[string]any{"rule": "EXACT_TRANSFER_REFERENCE_AMOUNT_CURRENCY", "payment_observation_id": pair.Left, "settlement_observation_id": pair.Right})
		_, err = tx.ExecContext(ctx, `INSERT INTO match_edges(organization_id,run_id,stage,payment_observation_id,settlement_observation_id,allocated_minor,rule,proof)
 VALUES($1,$2,'PAYMENT_SETTLEMENT',$3,$4,$5,'EXACT_TRANSFER_REFERENCE_AMOUNT_CURRENCY',$6)`, org, summary.RunID, pair.Left, pair.Right, eventByID[pair.Left].Amount, proof)
		if err != nil {
			return Summary{}, err
		}
	}
	matchedOrders := map[string]string{}
	for _, pair := range result.OrderPayment {
		matchedOrders[pair.Right] = pair.Left
	}
	for _, paymentID := range result.Missing {
		if err = upsertCase(ctx, tx, org, scenarioID, "SETTLEMENT_NOT_OBSERVED", paymentID, matchedOrders[paymentID], coverageID); err != nil {
			return Summary{}, err
		}
	}
	for _, paymentID := range result.Mismatched {
		if err = upsertCase(ctx, tx, org, scenarioID, "AMOUNT_MISMATCH", paymentID, matchedOrders[paymentID], coverageID); err != nil {
			return Summary{}, err
		}
	}
	for _, paymentID := range result.Ambiguous {
		if err = upsertCase(ctx, tx, org, scenarioID, "AMBIGUOUS_MATCH", paymentID, matchedOrders[paymentID], coverageID); err != nil {
			return Summary{}, err
		}
	}
	// A later settlement updates the evidence timeline without erasing the old run.
	for _, pair := range result.PaymentSettlement {
		if err = markSettlementEvidence(ctx, tx, org, pair.Left, pair.Right); err != nil {
			return Summary{}, err
		}
	}
	summary.OrderPayment = len(result.OrderPayment)
	summary.PaymentSettlement = len(result.PaymentSettlement)
	summary.Unmatched = len(result.Missing) + len(result.Mismatched) + len(result.Ambiguous)
	_, err = tx.ExecContext(ctx, `UPDATE reconciliation_runs SET status='COMPLETED',completed_at=now(),matched_order_payment=$2,matched_payment_settlement=$3,unmatched_count=$4 WHERE id=$1`, summary.RunID, summary.OrderPayment, summary.PaymentSettlement, summary.Unmatched)
	if err != nil {
		return Summary{}, err
	}
	return summary, tx.Commit()
}

func upsertCase(ctx context.Context, tx *sql.Tx, org, runID, kind, paymentID, orderID, coverageID string) error {
	fingerprint := kind + ":" + paymentID
	var caseID string
	err := tx.QueryRowContext(ctx, `INSERT INTO investigation_cases(organization_id,scenario_run_id,kind,fingerprint,title,summary,severity)
 VALUES($1,$2,$3,$4,$5,$6,'MEDIUM') ON CONFLICT (organization_id,fingerprint) DO UPDATE SET fingerprint=EXCLUDED.fingerprint RETURNING id`, org, runID, kind, fingerprint, kind, "Synthetic provider observations require review").Scan(&caseID)
	if err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM case_events WHERE organization_id=$1 AND case_id=$2", org, caseID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err = tx.ExecContext(ctx, "INSERT INTO case_events(organization_id,case_id,version,action,actor,message) VALUES($1,$2,1,'OPENED','SYSTEM','Discrepancy observed in a completed source window')", org, caseID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO case_evidence(organization_id,case_id,evidence_kind,evidence_id) VALUES($1,$2,'OBSERVATION',$3) ON CONFLICT DO NOTHING", org, caseID, paymentID); err != nil {
		return err
	}
	if orderID != "" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO case_evidence(organization_id,case_id,evidence_kind,evidence_id) VALUES($1,$2,'ORDER',$3) ON CONFLICT DO NOTHING", org, caseID, orderID); err != nil {
			return err
		}
	}
	if coverageID != "" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO case_evidence(organization_id,case_id,evidence_kind,evidence_id) VALUES($1,$2,'SOURCE_STATUS',$3) ON CONFLICT DO NOTHING", org, caseID, coverageID); err != nil {
			return err
		}
	}
	return nil
}

func markSettlementEvidence(ctx context.Context, tx *sql.Tx, org, paymentID, settlementID string) error {
	var caseID string
	err := tx.QueryRowContext(ctx, "SELECT id FROM investigation_cases WHERE organization_id=$1 AND fingerprint=$2", org, "SETTLEMENT_NOT_OBSERVED:"+paymentID).Scan(&caseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var inserted string
	err = tx.QueryRowContext(ctx, `INSERT INTO case_evidence(organization_id,case_id,evidence_kind,evidence_id) VALUES($1,$2,'OBSERVATION',$3) ON CONFLICT DO NOTHING RETURNING evidence_id`, org, caseID, settlementID).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var version int
	err = tx.QueryRowContext(ctx, `UPDATE investigation_cases SET status=CASE WHEN status='RESOLVED' THEN 'RESOLVED' ELSE 'EVIDENCE_UPDATED' END,version=version+1,updated_at=now() WHERE id=$1 AND organization_id=$2 RETURNING version`, caseID, org).Scan(&version)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO case_events(organization_id,case_id,version,action,actor,message) VALUES($1,$2,$3,'EVIDENCE_ADDED','SYSTEM',$4)", org, caseID, version, fmt.Sprintf("Settlement observation %s arrived", settlementID))
	return err
}
