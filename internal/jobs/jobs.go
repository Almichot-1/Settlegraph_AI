package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"github.com/settlegraph/settlegraph/internal/investigation"
	"github.com/settlegraph/settlegraph/internal/reconcile"
)

var investigationIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var errInvalidJob = errors.New("INVALID_JOB_PAYLOAD")

// Loop is the local PostgreSQL-backed worker. Leasing and idempotent run keys
// keep crash recovery safe without making SQS a local demo dependency.
func Loop(ctx context.Context, db *sql.DB) {
	LoopWithAnalyzer(ctx, db, investigation.OfflineAnalyzer{})
}
func LoopWithAnalyzer(ctx context.Context, db *sql.DB, analyzer investigation.Analyzer) {
	// Model compilation/inference can take over a minute. Keep reconciliation
	// on its own worker loop so a slow provider cannot hold up financial jobs.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processKind(ctx, db, analyzer, "INVESTIGATE"); err != nil {
					slog.Warn("investigation job failed", "error", err)
				}
			}
		}
	}()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := dispatch(ctx, db); err != nil {
				slog.Warn("outbox dispatch failed", "error", err)
			}
			if err := processKind(ctx, db, analyzer, "RECONCILE"); err != nil {
				slog.Warn("job processing failed", "error", err)
			}
		}
	}
}

func dispatch(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,organization_id,aggregate_ref,created_at FROM outbox_events WHERE published_at IS NULL AND event_type='OBSERVATION_RECEIVED' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 10`)
	if err != nil {
		return err
	}
	type item struct {
		id, org, ref string
		cutoff       time.Time
	}
	var items []item
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.id, &v.org, &v.ref, &v.cutoff); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		var scenario string
		var virtualNow time.Time
		err = tx.QueryRowContext(ctx, "SELECT scenario_run_id,occurred_at FROM observations WHERE organization_id=$1 AND inbox_event_id=$2", v.org, v.ref).Scan(&scenario, &virtualNow)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"scenario_run_id": scenario, "virtual_now": virtualNow, "cutoff": v.cutoff})
		if _, err = tx.ExecContext(ctx, `INSERT INTO jobs(organization_id,job_kind,job_key,payload) VALUES($1,'RECONCILE',$2,$3) ON CONFLICT (organization_id,job_kind,job_key) DO NOTHING`, v.org, "inbox:"+v.ref, payload); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE outbox_events SET published_at=now(),attempts=attempts+1 WHERE id=$1", v.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ProcessOne(ctx context.Context, db *sql.DB, analyzer investigation.Analyzer) error {
	return processKind(ctx, db, analyzer, "")
}
func processKind(ctx context.Context, db *sql.DB, analyzer investigation.Analyzer, only string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, org, key, lease, kind string
	var raw []byte
	var attempts int
	err = tx.QueryRowContext(ctx, `SELECT id,organization_id,job_key,payload,attempts,job_kind FROM jobs
 WHERE job_kind IN ('RECONCILE','INVESTIGATE') AND ($1='' OR job_kind=$1) AND next_attempt_at<=now() AND (status='QUEUED' OR (status='RUNNING' AND lease_expires_at<now()))
 ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, only).Scan(&id, &org, &key, &raw, &attempts, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	leaseSeconds := 30
	if kind == "INVESTIGATE" {
		leaseSeconds = 180
	}
	if err = tx.QueryRowContext(ctx, `UPDATE jobs SET status='RUNNING',attempts=attempts+1,lease_owner=gen_random_uuid()::text,lease_expires_at=now()+($2*interval '1 second') WHERE id=$1 RETURNING lease_owner`, id, leaseSeconds).Scan(&lease); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	var investigationID string
	workTimeout := 20 * time.Second
	if kind == "INVESTIGATE" {
		workTimeout = 150 * time.Second
	}
	workCtx, cancel := context.WithTimeout(ctx, workTimeout)
	if kind == "INVESTIGATE" {
		var payload struct {
			ID string `json:"investigation_id"`
		}
		err = json.Unmarshal(raw, &payload)
		if err != nil || !investigationIDPattern.MatchString(payload.ID) {
			err = errInvalidJob
		} else {
			investigationID = payload.ID
		}
		if err == nil {
			err = investigation.Process(workCtx, db, org, id, lease, investigationID, analyzer)
		}
	} else {
		var payload struct {
			ScenarioRunID string    `json:"scenario_run_id"`
			VirtualNow    time.Time `json:"virtual_now"`
			Cutoff        time.Time `json:"cutoff"`
		}
		err = json.Unmarshal(raw, &payload)
		if err == nil && (payload.VirtualNow.IsZero() || payload.Cutoff.IsZero()) {
			err = errors.New("job snapshot timestamps missing")
		}
		if err == nil {
			_, err = reconcile.Run(workCtx, db, org, payload.ScenarioRunID, key, payload.VirtualNow, payload.Cutoff)
		}
	}
	cancel()
	if err == nil {
		return acknowledge(ctx, db, id, org, key, lease, kind, "COMPLETED", 0)
	}
	status := "QUEUED"
	if attempts+1 >= 5 || errors.Is(err, errInvalidJob) {
		status = "FAILED"
	}
	if status == "FAILED" && kind == "INVESTIGATE" && investigationID != "" {
		if failureErr := investigation.Fail(ctx, db, org, id, lease, investigationID); failureErr != nil && !errors.Is(failureErr, sql.ErrNoRows) {
			return failureErr
		}
	}
	if updateErr := acknowledge(ctx, db, id, org, key, lease, kind, status, 1<<min(attempts, 6)); updateErr != nil {
		return updateErr
	}
	return err
}

func acknowledge(ctx context.Context, db *sql.DB, id, org, key, lease, kind, status string, delay int) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE jobs SET status=$3,lease_owner=NULL,lease_expires_at=NULL,
      completed_at=CASE WHEN $3 IN ('COMPLETED','FAILED') THEN now() ELSE NULL END,
      next_attempt_at=now()+($4*interval '1 second')
      WHERE id=$1 AND lease_owner=$2 AND lease_expires_at>now()`, id, lease, status, delay)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return investigation.ErrLease
	}
	if kind == "RECONCILE" && status == "FAILED" {
		if _, err = tx.ExecContext(ctx, `UPDATE reconciliation_runs SET status='FAILED',completed_at=now() WHERE organization_id=$1 AND job_key=$2 AND status IN ('QUEUED','RUNNING')`, org, key); err != nil {
			return err
		}
	}
	if kind == "INVESTIGATE" && status == "QUEUED" {
		if _, err = tx.ExecContext(ctx, `UPDATE ai_investigations SET status='QUEUED' WHERE organization_id=$1 AND id=(SELECT (payload->>'investigation_id')::uuid FROM jobs WHERE id=$2) AND status='RUNNING'`, org, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
