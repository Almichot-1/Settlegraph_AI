package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/settlegraph/settlegraph/internal/reconcile"
)

// Loop is the local PostgreSQL-backed worker. Leasing and idempotent run keys
// keep crash recovery safe without making SQS a local demo dependency.
func Loop(ctx context.Context, db *sql.DB) {
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
			if err := processOne(ctx, db); err != nil {
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

func processOne(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, org, key, lease string
	var raw []byte
	var attempts int
	err = tx.QueryRowContext(ctx, `SELECT id,organization_id,job_key,payload,attempts FROM jobs
 WHERE job_kind='RECONCILE' AND next_attempt_at<=now() AND (status='QUEUED' OR (status='RUNNING' AND lease_expires_at<now()))
 ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &org, &key, &raw, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `UPDATE jobs SET status='RUNNING',attempts=attempts+1,lease_owner=gen_random_uuid()::text,lease_expires_at=now()+interval '30 seconds' WHERE id=$1 RETURNING lease_owner`, id).Scan(&lease); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
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
		_, err = reconcile.Run(ctx, db, org, payload.ScenarioRunID, key, payload.VirtualNow, payload.Cutoff)
	}
	if err == nil {
		_, err = db.ExecContext(ctx, "UPDATE jobs SET status='COMPLETED',completed_at=now(),lease_owner=NULL,lease_expires_at=NULL WHERE id=$1 AND lease_owner=$2", id, lease)
		return err
	}
	status := "QUEUED"
	if attempts+1 >= 5 {
		status = "FAILED"
	}
	_, updateErr := db.ExecContext(ctx, `UPDATE jobs SET status=$3,lease_owner=NULL,lease_expires_at=NULL,next_attempt_at=now()+($4*interval '1 second') WHERE id=$1 AND lease_owner=$2`, id, lease, status, 1<<min(attempts, 6))
	if updateErr != nil {
		return updateErr
	}
	return err
}
