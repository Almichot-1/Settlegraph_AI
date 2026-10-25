package investigation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrLease = errors.New("JOB_LEASE_LOST")

// Process calls the model outside a transaction. Financial tables are read-only
// inputs; completion writes only investigation metadata, case version and audit.
func Process(ctx context.Context, db *sql.DB, org, job, lease, id string, analyzer Analyzer) error {
	var raw []byte
	var hash, provider, status, caseID string
	err := db.QueryRowContext(ctx, `SELECT evidence_snapshot,evidence_snapshot_hash,provider,status,case_id FROM ai_investigations WHERE organization_id=$1 AND id=$2`, org, id).Scan(&raw, &hash, &provider, &status, &caseID)
	if err != nil {
		return ErrProvider
	}
	if status == "COMPLETED" || status == "REJECTED" || status == "FAILED" {
		return nil
	}
	var packet Packet
	if json.Unmarshal(raw, &packet) != nil || packet.CaseID != caseID {
		return finish(ctx, db, org, job, lease, id, "REJECTED", nil, Output{}, 0, "SNAPSHOT_INVALID")
	}
	calculated, err := packet.Hash()
	if err != nil || calculated != hash {
		return finish(ctx, db, org, job, lease, id, "REJECTED", nil, Output{}, 0, "SNAPSHOT_INVALID")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ErrProvider
	}
	err = VerifyPacket(ctx, tx, org, packet)
	tx.Rollback()
	if errors.Is(err, ErrEvidence) {
		return finish(ctx, db, org, job, lease, id, "REJECTED", nil, Output{}, 0, "EVIDENCE_INVALID")
	}
	if err != nil {
		return ErrProvider
	}
	if analyzer == nil || analyzer.Provider() != provider {
		return ErrProvider
	}
	updated, err := db.ExecContext(ctx, `UPDATE ai_investigations SET status='RUNNING' WHERE organization_id=$1 AND id=$2 AND status IN ('QUEUED','RUNNING')`, org, id)
	if err != nil {
		return ErrProvider
	}
	count, _ := updated.RowsAffected()
	if count == 0 {
		return nil
	}
	started := time.Now()
	timeout := 15 * time.Second
	if provider == "BEDROCK" {
		timeout = 125 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	output, err := analyzer.Analyze(callCtx, packet)
	cancel()
	latency := time.Since(started).Milliseconds()
	if errors.Is(err, ErrOutput) {
		return finish(ctx, db, org, job, lease, id, "REJECTED", nil, output, latency, "OUTPUT_REJECTED")
	}
	if err != nil {
		return ErrProvider
	}
	if ValidatePacketResult(output.Result, packet) != nil {
		return finish(ctx, db, org, job, lease, id, "REJECTED", nil, output, latency, "OUTPUT_REJECTED")
	}
	return finish(ctx, db, org, job, lease, id, "COMPLETED", &packet, output, latency, "")
}

func finish(ctx context.Context, db *sql.DB, org, job, lease, id, status string, packet *Packet, output Output, latency int64, code string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owned bool
	err = tx.QueryRowContext(ctx, `SELECT true FROM jobs WHERE organization_id=$1 AND id=$2 AND lease_owner=$3 AND status='RUNNING' AND lease_expires_at>now() FOR UPDATE`, org, job, lease).Scan(&owned)
	if err != nil || !owned {
		return ErrLease
	}
	var current, caseID, actor string
	err = tx.QueryRowContext(ctx, `SELECT status,case_id,COALESCE(actor_sub,'worker') FROM ai_investigations WHERE organization_id=$1 AND id=$2 FOR UPDATE`, org, id).Scan(&current, &caseID, &actor)
	if err != nil {
		return err
	}
	if current == "COMPLETED" || current == "REJECTED" || current == "FAILED" {
		return tx.Commit()
	}
	if packet != nil {
		err = VerifyPacket(ctx, tx, org, *packet)
		if err != nil && !errors.Is(err, ErrEvidence) {
			return err
		}
		if err != nil {
			status = "REJECTED"
			code = "EVIDENCE_INVALID"
			packet = nil
		}
	}
	var result any
	validation := "REJECTED"
	if status == "COMPLETED" {
		result, err = json.Marshal(output.Result)
		if err != nil {
			return err
		}
		validation = "VALIDATED"
		var version int
		err = tx.QueryRowContext(ctx, `UPDATE investigation_cases SET version=version+1,updated_at=now() WHERE organization_id=$1 AND id=$2 RETURNING version`, org, caseID).Scan(&version)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO case_events(organization_id,case_id,version,action,actor,message) VALUES($1,$2,$3,'INVESTIGATION_COMPLETED',$4,'Evidence-linked analysis completed; review its source and limitations')`, org, caseID, version, actor)
		if err != nil {
			return err
		}
	}
	if status == "FAILED" {
		validation = "NOT_COMPLETED"
	}
	_, err = tx.ExecContext(ctx, `UPDATE ai_investigations SET status=$3,result=$4,completed_at=now(),model_id=$5,prompt_version=$6,invocation_id=$7,latency_ms=$8,input_tokens=$9,output_tokens=$10,validation_status=$11,failure_code=NULLIF($12,'') WHERE organization_id=$1 AND id=$2`, org, id, status, result, output.ModelID, PromptVersion, output.InvocationID, latency, output.InputTokens, output.OutputTokens, validation, code)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(organization_id,actor,action,resource_type,resource_id) VALUES($1,$2,$3,'INVESTIGATION',$4)`, org, actor, "INVESTIGATION_"+status, id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func Fail(ctx context.Context, db *sql.DB, org, job, lease, id string) error {
	return finish(ctx, db, org, job, lease, id, "FAILED", nil, Output{}, 0, "RETRY_BUDGET_EXHAUSTED")
}
