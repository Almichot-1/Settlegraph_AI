//go:build integration

package reconcile

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestConcurrentRunDeduplication(t *testing.T) {
	dsn := os.Getenv("PRODUCT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PRODUCT_TEST_DATABASE_URL required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(24)
	ctx := context.Background()
	const org = "11111111-1111-4111-8111-111111111111"
	base := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	var runID, orderID, paymentInbox, settlementInbox, paymentID, settlementID string
	err = db.QueryRow(`INSERT INTO scenario_runs(organization_id,scenario_key,seed,status,current_step,total_steps,virtual_now) VALUES($1,'normal_flow',123,'COMPLETED',2,2,$2) RETURNING id`, org, base.Add(2*time.Hour)).Scan(&runID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(`INSERT INTO merchant_orders(organization_id,scenario_run_id,external_reference,amount_minor,created_at) VALUES($1,$2,'ORD-123',250000,$3) RETURNING id`, org, runID, base).Scan(&orderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		provider string
		target   *string
	}{{"BANK_A", &paymentInbox}, {"BANK_B", &settlementInbox}} {
		err = db.QueryRow(`INSERT INTO inbox_events(organization_id,provider_id,provider_event_id,source_sequence,payload,payload_sha256) VALUES($1,$2,gen_random_uuid(),1,'{}','test') RETURNING id`, org, entry.provider).Scan(entry.target)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = db.QueryRow(`INSERT INTO observations(organization_id,inbox_event_id,provider_id,kind,transfer_reference,payment_reference,order_reference,amount_minor,occurred_at,scenario_run_id,source_sequence) VALUES($1,$2,'BANK_A','PAYMENT_ACCEPTED','SIM-123','SIM-123','ORD-123',250000,$3,$4,1) RETURNING id`, org, paymentInbox, base, runID).Scan(&paymentID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(`INSERT INTO observations(organization_id,inbox_event_id,provider_id,kind,transfer_reference,payment_reference,order_reference,amount_minor,occurred_at,scenario_run_id,source_sequence) VALUES($1,$2,'BANK_B','SETTLEMENT_POSTED','SIM-123','SIM-123','ORD-123',250000,$3,$4,1) RETURNING id`, org, settlementInbox, base.Add(2*time.Hour), runID).Scan(&settlementID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO source_status_snapshots(organization_id,provider_id,source_status,complete_through,scenario_run_id) VALUES($1,'BANK_B','HEALTHY',$2,$3)`, org, base.Add(2*time.Hour), runID)
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().Add(time.Second)
	var wg sync.WaitGroup
	errorsSeen := make(chan error, 20)
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			summary, e := Run(ctx, db, org, runID, "same-job", base.Add(2*time.Hour), cutoff)
			if e != nil {
				errorsSeen <- e
				return
			}
			if summary.OrderPayment != 1 || summary.PaymentSettlement != 1 || summary.Unmatched != 0 {
				errorsSeen <- fmt.Errorf("bad summary: %+v", summary)
				return
			}
			ids <- summary.RunID
		}()
	}
	wg.Wait()
	close(errorsSeen)
	close(ids)
	for e := range errorsSeen {
		t.Error(e)
	}
	first := ""
	for id := range ids {
		if first != "" && id != first {
			t.Errorf("duplicate run IDs: %s vs %s", first, id)
		}
		first = id
	}
	if first == "" {
		t.Fatal("no successful run")
	}
	var runCount, edgeCount int
	if err := db.QueryRow(`SELECT count(*) FROM reconciliation_runs WHERE organization_id=$1 AND job_key='same-job'`, org).Scan(&runCount); err != nil || runCount != 1 {
		t.Fatalf("run count=%d err=%v", runCount, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM match_edges WHERE run_id=$1`, first).Scan(&edgeCount); err != nil || edgeCount != 2 {
		t.Fatalf("edge count=%d err=%v", edgeCount, err)
	}
}

func TestLatestObservationsUsesUnscopedEvidence(t *testing.T) {
	dsn := os.Getenv("PRODUCT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PRODUCT_TEST_DATABASE_URL required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const org = "11111111-1111-4111-8111-111111111111"
	now := time.Now().UTC()
	_, err = db.Exec(`INSERT INTO merchant_orders(organization_id,external_reference,amount_minor,created_at) VALUES($1,'IMPORT-ORDER-1',700, $2)`, org, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ provider, kind string }{{"BANK_A", "PAYMENT_ACCEPTED"}, {"BANK_B", "SETTLEMENT_POSTED"}} {
		var inbox string
		err = db.QueryRow(`INSERT INTO inbox_events(organization_id,provider_id,provider_event_id,source_sequence,payload,payload_sha256) VALUES($1,$2,gen_random_uuid(),1,'{}','test') RETURNING id`, org, item.provider).Scan(&inbox)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO observations(organization_id,inbox_event_id,provider_id,kind,transfer_reference,payment_reference,order_reference,amount_minor,occurred_at,source_sequence) VALUES($1,$2,$3,$4,'IMPORT-PAY-1','IMPORT-PAY-1','IMPORT-ORDER-1',700,$5,1)`, org, inbox, item.provider, item.kind, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	summary, err := Run(context.Background(), db, org, "", "latest-unscoped-test", now.Add(time.Hour), time.Now().Add(time.Second))
	if err != nil || summary.OrderPayment != 1 || summary.PaymentSettlement != 1 || summary.Unmatched != 0 {
		t.Fatalf("latest result=%+v err=%v", summary, err)
	}
	var scope string
	var scenario sql.NullString
	err = db.QueryRow(`SELECT scope,scenario_run_id FROM reconciliation_runs WHERE id=$1`, summary.RunID).Scan(&scope, &scenario)
	if err != nil || scope != "LATEST_OBSERVATIONS" || scenario.Valid {
		t.Fatalf("scope=%s scenario=%v err=%v", scope, scenario, err)
	}
}
