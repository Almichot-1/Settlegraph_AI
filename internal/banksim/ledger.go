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

var (
	ErrInsufficientFunds   = errors.New("insufficient synthetic funds")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different request")
	ErrInvalidTransition   = errors.New("invalid transfer transition")
)

const ClearingAccountID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2"

type Transfer struct {
	ID            string    `json:"id"`
	FromAccountID string    `json:"from_account_id"`
	ToAccountID   string    `json:"to_account_id"`
	Amount        Money     `json:"amount"`
	State         string    `json:"state"`
	Reference     string    `json:"reference"`
	CreatedAt     time.Time `json:"created_at"`
}
type Money struct {
	MinorUnits string `json:"minor_units"`
	Currency   string `json:"currency"`
}
type CreateRequest struct {
	FromAccountID  string `json:"from_account_id"`
	ToAccountID    string `json:"to_account_id"`
	Amount         Money  `json:"amount"`
	OrderReference string `json:"order_reference"`
}

// Ledger owns BankSim's database. Product code must never import this package.
type Ledger struct{ DB *sql.DB }

func (l Ledger) Accept(ctx context.Context, req CreateRequest, key, scenarioID string, now time.Time) (Transfer, error) {
	amount, err := strconv.ParseInt(req.Amount.MinorUnits, 10, 64)
	if err != nil || amount <= 0 || req.Amount.Currency != "ETB" || req.FromAccountID == req.ToAccountID || req.OrderReference == "" || key == "" {
		return Transfer{}, errors.New("invalid transfer request")
	}
	body, _ := json.Marshal(req)
	hash := sha256.Sum256(body)
	digest := hex.EncodeToString(hash[:])
	tx, err := l.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Transfer{}, err
	}
	defer tx.Rollback()
	// The transaction-scoped lock serializes retries before any ledger effect.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "transfer:"+key); err != nil {
		return Transfer{}, err
	}
	var oldHash string
	var oldBody []byte
	var oldStatus int
	err = tx.QueryRowContext(ctx, "SELECT request_sha256,response_status,response_body FROM sim_idempotency WHERE scope='transfer' AND key=$1", key).Scan(&oldHash, &oldStatus, &oldBody)
	if err == nil {
		if oldHash != digest {
			return Transfer{}, ErrIdempotencyConflict
		}
		var prior Transfer
		if json.Unmarshal(oldBody, &prior) != nil {
			return Transfer{}, errors.New("stored idempotency response invalid")
		}
		if err = tx.Commit(); err != nil {
			return Transfer{}, err
		}
		if oldStatus == 409 && prior.State == "FAILED" {
			return prior, ErrInsufficientFunds
		}
		return prior, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Transfer{}, err
	}
	var payerBalance int64
	var payerKind string
	err = tx.QueryRowContext(ctx, "SELECT balance_minor,account_type FROM sim_accounts WHERE id=$1 FOR UPDATE", req.FromAccountID).Scan(&payerBalance, &payerKind)
	if err != nil {
		return Transfer{}, err
	}
	if payerKind != "CUSTOMER" {
		return Transfer{}, errors.New("source must be customer account")
	}
	var merchantKind string
	if err = tx.QueryRowContext(ctx, "SELECT account_type FROM sim_accounts WHERE id=$1", req.ToAccountID).Scan(&merchantKind); err != nil {
		return Transfer{}, err
	}
	if merchantKind != "MERCHANT" {
		return Transfer{}, errors.New("destination must be merchant account")
	}
	var clearingBalance int64
	if err = tx.QueryRowContext(ctx, "SELECT balance_minor FROM sim_accounts WHERE id=$1 FOR UPDATE", ClearingAccountID).Scan(&clearingBalance); err != nil {
		return Transfer{}, err
	}
	if clearingBalance > int64(^uint64(0)>>1)-amount {
		return Transfer{}, errors.New("clearing balance overflow")
	}
	var t Transfer
	t.FromAccountID = req.FromAccountID
	t.ToAccountID = req.ToAccountID
	t.Amount = req.Amount
	t.State = "CREATED"
	t.CreatedAt = now.UTC()
	var scenario any
	if scenarioID != "" {
		scenario = scenarioID
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO sim_transfers(scenario_id,from_account_id,to_account_id,order_reference,transfer_reference,amount_minor,state,created_at,updated_at)
 VALUES($1,$2,$3,$4,'PENDING-'||gen_random_uuid()::text,$5,'CREATED',$6,$6) RETURNING id`, scenario, req.FromAccountID, req.ToAccountID, req.OrderReference, amount, t.CreatedAt).Scan(&t.ID)
	if err != nil {
		return Transfer{}, err
	}
	t.Reference = "SIM-" + t.ID
	if _, err = tx.ExecContext(ctx, "UPDATE sim_transfers SET transfer_reference=$2 WHERE id=$1", t.ID, t.Reference); err != nil {
		return Transfer{}, err
	}
	if payerBalance < amount {
		t.State = "FAILED"
		if _, err = tx.ExecContext(ctx, "UPDATE sim_transfers SET state='FAILED' WHERE id=$1", t.ID); err != nil {
			return Transfer{}, err
		}
		response, _ := json.Marshal(t)
		if _, err = tx.ExecContext(ctx, "INSERT INTO sim_idempotency(scope,key,request_sha256,response_status,response_body) VALUES('transfer',$1,$2,409,$3)", key, digest, response); err != nil {
			return Transfer{}, err
		}
		if err = tx.Commit(); err != nil {
			return Transfer{}, err
		}
		return t, ErrInsufficientFunds
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor-$2 WHERE id=$1", req.FromAccountID, amount); err != nil {
		return Transfer{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor+$2 WHERE id=$1", ClearingAccountID, amount); err != nil {
		return Transfer{}, err
	}
	if err = postJournal(ctx, tx, t.ID, "ACCEPT", req.FromAccountID, ClearingAccountID, amount, now); err != nil {
		return Transfer{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_transfers SET state='ACCEPTED' WHERE id=$1", t.ID); err != nil {
		return Transfer{}, err
	}
	t.State = "ACCEPTED"
	if scenarioID != "" {
		if err = insertEvent(ctx, tx, scenarioID, "BANK_A", "PAYMENT_ACCEPTED", t, req.OrderReference, now); err != nil {
			return Transfer{}, err
		}
	}
	response, _ := json.Marshal(t)
	if _, err = tx.ExecContext(ctx, "INSERT INTO sim_idempotency(scope,key,request_sha256,response_status,response_body) VALUES('transfer',$1,$2,201,$3)", key, digest, response); err != nil {
		return Transfer{}, err
	}
	return t, tx.Commit()
}

func (l Ledger) Settle(ctx context.Context, id string, now time.Time) (Transfer, error) {
	tx, err := l.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Transfer{}, err
	}
	defer tx.Rollback()
	var t Transfer
	var amount int64
	var orderRef string
	var scenario sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,from_account_id,to_account_id,amount_minor,state,transfer_reference,created_at,order_reference,scenario_id FROM sim_transfers WHERE id=$1 FOR UPDATE`, id).
		Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &amount, &t.State, &t.Reference, &t.CreatedAt, &orderRef, &scenario)
	if err != nil {
		return Transfer{}, err
	}
	if t.State != "ACCEPTED" && t.State != "SETTLEMENT_PENDING" {
		return Transfer{}, ErrInvalidTransition
	}
	var clearingBalance int64
	if err = tx.QueryRowContext(ctx, "SELECT balance_minor FROM sim_accounts WHERE id=$1 FOR UPDATE", ClearingAccountID).Scan(&clearingBalance); err != nil {
		return Transfer{}, err
	}
	if clearingBalance < amount {
		return Transfer{}, errors.New("clearing account below settlement amount")
	}
	var merchantBalance int64
	if err = tx.QueryRowContext(ctx, "SELECT balance_minor FROM sim_accounts WHERE id=$1 FOR UPDATE", t.ToAccountID).Scan(&merchantBalance); err != nil {
		return Transfer{}, err
	}
	if merchantBalance > int64(^uint64(0)>>1)-amount {
		return Transfer{}, errors.New("merchant balance overflow")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_transfers SET state='SETTLEMENT_PENDING',updated_at=$2 WHERE id=$1", id, now.UTC()); err != nil {
		return Transfer{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor-$2 WHERE id=$1", ClearingAccountID, amount); err != nil {
		return Transfer{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_accounts SET balance_minor=balance_minor+$2 WHERE id=$1", t.ToAccountID, amount); err != nil {
		return Transfer{}, err
	}
	if err = postJournal(ctx, tx, t.ID, "SETTLE", ClearingAccountID, t.ToAccountID, amount, now); err != nil {
		return Transfer{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sim_transfers SET state='SETTLED',updated_at=$2 WHERE id=$1", id, now.UTC()); err != nil {
		return Transfer{}, err
	}
	t.State = "SETTLED"
	t.Amount = Money{MinorUnits: strconv.FormatInt(amount, 10), Currency: "ETB"}
	if scenario.Valid {
		if err = insertEvent(ctx, tx, scenario.String, "BANK_B", "SETTLEMENT_POSTED", t, orderRef, now); err != nil {
			return Transfer{}, err
		}
	}
	return t, tx.Commit()
}

func postJournal(ctx context.Context, tx *sql.Tx, transferID, action, debit, credit string, amount int64, at time.Time) error {
	var id string
	if err := tx.QueryRowContext(ctx, "INSERT INTO sim_journals(transfer_id,action,occurred_at) VALUES($1,$2,$3) RETURNING id", transferID, action, at.UTC()).Scan(&id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO sim_entries(journal_id,account_id,signed_minor) VALUES($1,$2,$3),($1,$4,$5)", id, debit, -amount, credit, amount); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE sim_journals SET status='POSTED' WHERE id=$1", id)
	return err
}

func insertEvent(ctx context.Context, tx *sql.Tx, scenario, provider, kind string, t Transfer, orderRef string, at time.Time) error {
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "provider-sequence:"+provider); err != nil {
		return err
	}
	var seq int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence),0)+1 FROM sim_events WHERE provider_id=$1", provider).Scan(&seq); err != nil {
		return err
	}
	var id string
	if err := tx.QueryRowContext(ctx, "SELECT gen_random_uuid()").Scan(&id); err != nil {
		return err
	}
	payload := map[string]any{"event_id": id, "provider_id": provider, "scenario_run_id": scenario, "sequence": seq, "event_type": kind, "occurred_at": at.UTC().Format(time.RFC3339), "transfer_reference": t.Reference, "order_reference": orderRef, "payment_reference": t.Reference, "amount": t.Amount, "payload_version": 1}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO sim_events(id,scenario_id,provider_id,sequence,payload,occurred_at) VALUES($1,$2,$3,$4,$5,$6)", id, scenario, provider, seq, data, at.UTC())
	if err != nil {
		return fmt.Errorf("insert provider event: %w", err)
	}
	return nil
}
