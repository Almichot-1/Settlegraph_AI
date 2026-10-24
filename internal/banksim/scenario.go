package banksim

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

var ErrStaleStep = errors.New("scenario step does not match expected_step")

const (
	payerID         = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1"
	merchantID      = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1"
	baseVirtualTime = "2026-10-24T09:00:00Z"
)

type Scenario struct {
	ID             string    `json:"id"`
	ScenarioKey    string    `json:"scenario_key"`
	Status         string    `json:"status"`
	Step           int       `json:"step"`
	TotalSteps     int       `json:"total_steps"`
	VirtualNow     time.Time `json:"virtual_now"`
	OrganizationID string    `json:"organization_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func scenarioSteps(key string) int {
	switch key {
	case "normal_flow", "duplicate_webhook":
		return 2
	case "delayed_settlement", "provider_outage":
		return 3
	}
	return 0
}

func (l Ledger) StartScenario(ctx context.Context, key, org string, seed int, idempotencyKey string) (Scenario, error) {
	steps := scenarioSteps(key)
	if steps == 0 || seed < 0 || idempotencyKey == "" {
		return Scenario{}, errors.New("invalid scenario request")
	}
	tx, err := l.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Scenario{}, err
	}
	defer tx.Rollback()
	scope := "scenario-start:" + org + ":" + key
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", scope+":"+idempotencyKey); err != nil {
		return Scenario{}, err
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", key, org, seed)))
	digest := hex.EncodeToString(hash[:])
	var priorHash string
	var priorJSON []byte
	err = tx.QueryRowContext(ctx, "SELECT request_sha256,response_body FROM sim_idempotency WHERE scope=$1 AND key=$2", scope, idempotencyKey).Scan(&priorHash, &priorJSON)
	if err == nil {
		if priorHash != digest {
			return Scenario{}, ErrIdempotencyConflict
		}
		var prior Scenario
		if json.Unmarshal(priorJSON, &prior) != nil {
			return Scenario{}, errors.New("stored scenario response invalid")
		}
		return prior, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Scenario{}, err
	}
	base, _ := time.Parse(time.RFC3339, baseVirtualTime)
	var run Scenario
	run.ScenarioKey = key
	run.OrganizationID = org
	run.Status = "READY"
	run.TotalSteps = steps
	run.VirtualNow = base
	err = tx.QueryRowContext(ctx, `INSERT INTO sim_scenarios(organization_id,scenario_key,seed,total_steps,virtual_now,status) VALUES($1,$2,$3,$4,$5,'READY') RETURNING id,created_at`, org, key, seed, steps, base).Scan(&run.ID, &run.CreatedAt)
	if err != nil {
		return Scenario{}, err
	}
	data, _ := json.Marshal(run)
	if _, err = tx.ExecContext(ctx, "INSERT INTO sim_idempotency(scope,key,request_sha256,response_status,response_body) VALUES($1,$2,$3,201,$4)", scope, idempotencyKey, digest, data); err != nil {
		return Scenario{}, err
	}
	return run, tx.Commit()
}

func (l Ledger) GetScenario(ctx context.Context, id string) (Scenario, error) {
	var run Scenario
	err := l.DB.QueryRowContext(ctx, "SELECT id,organization_id,scenario_key,status,current_step,total_steps,virtual_now,created_at FROM sim_scenarios WHERE id=$1", id).
		Scan(&run.ID, &run.OrganizationID, &run.ScenarioKey, &run.Status, &run.Step, &run.TotalSteps, &run.VirtualNow, &run.CreatedAt)
	return run, err
}

func (l Ledger) AdvanceScenario(ctx context.Context, id string, expected int, key string) (Scenario, error) {
	if expected < 0 || key == "" {
		return Scenario{}, errors.New("invalid advance request")
	}
	tx, err := l.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Scenario{}, err
	}
	defer tx.Rollback()
	scope := "scenario-advance:" + id
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", scope+":"+key); err != nil {
		return Scenario{}, err
	}
	hash := sha256.Sum256([]byte(strconv.Itoa(expected)))
	digest := hex.EncodeToString(hash[:])
	var priorHash string
	var priorJSON []byte
	err = tx.QueryRowContext(ctx, "SELECT request_sha256,response_body FROM sim_idempotency WHERE scope=$1 AND key=$2", scope, key).Scan(&priorHash, &priorJSON)
	if err == nil {
		if priorHash != digest {
			return Scenario{}, ErrIdempotencyConflict
		}
		var prior Scenario
		if json.Unmarshal(priorJSON, &prior) != nil {
			return Scenario{}, errors.New("stored advance response invalid")
		}
		return prior, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Scenario{}, err
	}
	var run Scenario
	var seed int
	err = tx.QueryRowContext(ctx, "SELECT id,organization_id,scenario_key,status,current_step,total_steps,virtual_now,created_at,seed FROM sim_scenarios WHERE id=$1 FOR UPDATE", id).
		Scan(&run.ID, &run.OrganizationID, &run.ScenarioKey, &run.Status, &run.Step, &run.TotalSteps, &run.VirtualNow, &run.CreatedAt, &seed)
	if err != nil {
		return Scenario{}, err
	}
	if run.Step != expected || run.Step >= run.TotalSteps {
		return Scenario{}, ErrStaleStep
	}
	next := run.Step + 1
	if next == 1 {
		err = l.scenarioAccept(ctx, tx, run, seed)
	} else if run.ScenarioKey == "delayed_settlement" && next == 2 {
		run.VirtualNow = run.VirtualNow.Add(2 * time.Hour)
	} else if run.ScenarioKey == "provider_outage" && next == 2 {
		run.VirtualNow = run.VirtualNow.Add(2 * time.Hour)
		err = l.scenarioSettle(ctx, tx, run)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE sim_events SET delivery_status='DELAYED' WHERE scenario_id=$1 AND provider_id='BANK_B' AND delivery_status='PENDING'", run.ID)
		}
	} else if run.ScenarioKey == "provider_outage" && next == 3 {
		run.VirtualNow = run.VirtualNow.Add(time.Hour)
		_, err = tx.ExecContext(ctx, "UPDATE sim_events SET delivery_status='PENDING' WHERE scenario_id=$1 AND provider_id='BANK_B' AND delivery_status='DELAYED'", run.ID)
	} else {
		run.VirtualNow = run.VirtualNow.Add(time.Hour)
		err = l.scenarioSettle(ctx, tx, run)
	}
	if err != nil {
		return Scenario{}, err
	}
	run.Step = next
	run.Status = "RUNNING"
	if next == run.TotalSteps {
		run.Status = "COMPLETED"
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_scenarios SET current_step=$2,status=$3,virtual_now=$4 WHERE id=$1", run.ID, run.Step, run.Status, run.VirtualNow); err != nil {
		return Scenario{}, err
	}
	data, _ := json.Marshal(run)
	if _, err = tx.ExecContext(ctx, "INSERT INTO sim_idempotency(scope,key,request_sha256,response_status,response_body) VALUES($1,$2,$3,200,$4)", scope, key, digest, data); err != nil {
		return Scenario{}, err
	}
	return run, tx.Commit()
}

func (l Ledger) scenarioAccept(ctx context.Context, tx *sql.Tx, run Scenario, seed int) error {
	const amount int64 = 250000
	var payer, clearing int64
	if err := tx.QueryRowContext(ctx, "SELECT balance_minor FROM sim_accounts WHERE id=$1 FOR UPDATE", payerID).Scan(&payer); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "SELECT balance_minor FROM sim_accounts WHERE id=$1 FOR UPDATE", ClearingAccountID).Scan(&clearing); err != nil {
		return err
	}
	if payer < amount {
		return ErrInsufficientFunds
	}
	if clearing > int64(^uint64(0)>>1)-amount {
		return errors.New("clearing balance overflow")
	}
	var t Transfer
	t.FromAccountID = payerID
	t.ToAccountID = merchantID
	t.Amount = Money{MinorUnits: "250000", Currency: "ETB"}
	t.State = "CREATED"
	t.CreatedAt = run.VirtualNow
	orderRef := fmt.Sprintf("ORD-%d", seed)
	if err := tx.QueryRowContext(ctx, `INSERT INTO sim_transfers(scenario_id,from_account_id,to_account_id,order_reference,transfer_reference,amount_minor,state,created_at,updated_at)
 VALUES($1,$2,$3,$4,'PENDING-'||gen_random_uuid()::text,$5,'CREATED',$6,$6) RETURNING id`, run.ID, payerID, merchantID, orderRef, amount, run.VirtualNow).Scan(&t.ID); err != nil {
		return err
	}
	t.Reference = "SIM-" + t.ID
	if _, err := tx.ExecContext(ctx, "UPDATE sim_transfers SET transfer_reference=$2 WHERE id=$1", t.ID, t.Reference); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor-$2 WHERE id=$1", payerID, amount); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor+$2 WHERE id=$1", ClearingAccountID, amount); err != nil {
		return err
	}
	if err := postJournal(ctx, tx, t.ID, "ACCEPT", payerID, ClearingAccountID, amount, run.VirtualNow); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_transfers SET state='ACCEPTED' WHERE id=$1", t.ID); err != nil {
		return err
	}
	t.State = "ACCEPTED"
	return insertEvent(ctx, tx, run.ID, "BANK_A", "PAYMENT_ACCEPTED", t, orderRef, run.VirtualNow)
}

func (l Ledger) scenarioSettle(ctx context.Context, tx *sql.Tx, run Scenario) error {
	var t Transfer
	var orderRef string
	var amount int64
	if err := tx.QueryRowContext(ctx, "SELECT id,from_account_id,to_account_id,transfer_reference,order_reference,amount_minor,state,created_at FROM sim_transfers WHERE scenario_id=$1 FOR UPDATE", run.ID).
		Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &t.Reference, &orderRef, &amount, &t.State, &t.CreatedAt); err != nil {
		return err
	}
	if t.State != "ACCEPTED" {
		return ErrInvalidTransition
	}
	var clearing int64
	if err := tx.QueryRowContext(ctx, "SELECT balance_minor FROM sim_accounts WHERE id=$1 FOR UPDATE", ClearingAccountID).Scan(&clearing); err != nil {
		return err
	}
	if clearing < amount {
		return ErrInsufficientFunds
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_transfers SET state='SETTLEMENT_PENDING',updated_at=$2 WHERE id=$1", t.ID, run.VirtualNow); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor-$2 WHERE id=$1", ClearingAccountID, amount); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor+$2 WHERE id=$1", merchantID, amount); err != nil {
		return err
	}
	if err := postJournal(ctx, tx, t.ID, "SETTLE", ClearingAccountID, merchantID, amount, run.VirtualNow); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sim_transfers SET state='SETTLED',updated_at=$2 WHERE id=$1", t.ID, run.VirtualNow); err != nil {
		return err
	}
	t.Amount = Money{MinorUnits: strconv.FormatInt(amount, 10), Currency: "ETB"}
	t.State = "SETTLED"
	return insertEvent(ctx, tx, run.ID, "BANK_B", "SETTLEMENT_POSTED", t, orderRef, run.VirtualNow)
}
