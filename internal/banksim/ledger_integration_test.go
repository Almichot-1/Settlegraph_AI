//go:build integration

package banksim

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestConcurrentLedgerEffects(t *testing.T) {
	dsn := os.Getenv("BANKSIM_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("BANKSIM_TEST_DATABASE_URL required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(40)
	ledger := Ledger{DB: db}
	ctx := context.Background()
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	request := CreateRequest{FromAccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", ToAccountID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1", Amount: Money{MinorUnits: "250000", Currency: "ETB"}, OrderReference: "TEST-ONE"}
	ids := make(chan string, 100)
	errCh := make(chan error, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transfer, e := ledger.Accept(ctx, request, "same-key-100", "", now)
			if e != nil {
				errCh <- e
				return
			}
			ids <- transfer.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errCh)
	for e := range errCh {
		t.Errorf("same-key transfer: %v", e)
	}
	first := ""
	count := 0
	for id := range ids {
		if first != "" && first != id {
			t.Errorf("multiple transfer IDs: %s, %s", first, id)
		}
		first = id
		count++
	}
	if count != 100 {
		t.Fatalf("successful retries=%d", count)
	}
	var debits int
	var balance int64
	if err := db.QueryRow("SELECT count(*) FROM sim_journals WHERE transfer_id=$1 AND action='ACCEPT'", first).Scan(&debits); err != nil || debits != 1 {
		t.Fatalf("journals=%d err=%v", debits, err)
	}
	if err := db.QueryRow("SELECT balance_minor FROM sim_accounts WHERE id=$1", request.FromAccountID).Scan(&balance); err != nil || balance != 99750000 {
		t.Fatalf("payer balance=%d err=%v", balance, err)
	}
	request.Amount.MinorUnits = "2000000"
	var success, insufficient int
	var mu sync.Mutex
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := ledger.Accept(ctx, request, fmt.Sprintf("distinct-%03d", i), "", now)
			mu.Lock()
			defer mu.Unlock()
			if e == nil {
				success++
			} else if errors.Is(e, ErrInsufficientFunds) {
				insufficient++
			} else {
				errCh2 := e
				t.Errorf("distinct transfer: %v", errCh2)
			}
		}(i)
	}
	wg.Wait()
	if success != 49 || insufficient != 51 {
		t.Fatalf("distinct transfers: success=%d insufficient=%d", success, insufficient)
	}
	if err := db.QueryRow("SELECT balance_minor FROM sim_accounts WHERE id=$1", request.FromAccountID).Scan(&balance); err != nil || balance != 1750000 {
		t.Fatalf("payer after concurrency=%d err=%v", balance, err)
	}
	request.OrderReference = "TEST-FAILED-IDEMPOTENT"
	request.Amount.MinorUnits = "3000000"
	failed, err := ledger.Accept(ctx, request, "failed-key", "", now)
	if !errors.Is(err, ErrInsufficientFunds) || failed.State != "FAILED" {
		t.Fatalf("failed transfer=%+v err=%v", failed, err)
	}
	failedRetry, err := ledger.Accept(ctx, request, "failed-key", "", now)
	if !errors.Is(err, ErrInsufficientFunds) || failedRetry.ID != failed.ID {
		t.Fatalf("failed retry=%+v err=%v", failedRetry, err)
	}
	var failedJournals int
	if err := db.QueryRow("SELECT count(*) FROM sim_journals WHERE transfer_id=$1", failed.ID).Scan(&failedJournals); err != nil || failedJournals != 0 {
		t.Fatalf("failed transfer journal count=%d err=%v", failedJournals, err)
	}
	transfer, err := ledger.Settle(ctx, first, now.Add(time.Hour))
	if err != nil || transfer.State != "SETTLED" {
		t.Fatalf("settle=%+v err=%v", transfer, err)
	}
	if _, err := ledger.Settle(ctx, first, now.Add(2*time.Hour)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second settlement err=%v", err)
	}
	rows, err := db.Query("SELECT state FROM sim_transfer_state_events WHERE transfer_id=$1 ORDER BY occurred_at,id", first)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]bool{}
	for rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			t.Fatal(err)
		}
		states[state] = true
	}
	rows.Close()
	for _, state := range []string{"CREATED", "ACCEPTED", "SETTLEMENT_PENDING", "SETTLED"} {
		if !states[state] {
			t.Fatalf("missing transfer state %s: %v", state, states)
		}
	}
	if _, err := db.Exec("UPDATE sim_transfers SET state='ACCEPTED' WHERE id=$1", first); err == nil {
		t.Fatal("settled transfer accepted an invalid transition")
	}
	var unbalanced int
	if err := db.QueryRow(`SELECT count(*) FROM (SELECT j.id FROM sim_journals j JOIN sim_entries e ON e.journal_id=j.id WHERE j.status='POSTED' GROUP BY j.id HAVING sum(e.signed_minor)<>0) bad`).Scan(&unbalanced); err != nil || unbalanced != 0 {
		t.Fatalf("unbalanced journals=%d err=%v", unbalanced, err)
	}
}
