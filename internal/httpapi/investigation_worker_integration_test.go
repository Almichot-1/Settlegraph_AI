//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/settlegraph/settlegraph/internal/config"
	"github.com/settlegraph/settlegraph/internal/investigation"
	"github.com/settlegraph/settlegraph/internal/jobs"
)

type testAnalyzer struct {
	bad, fail bool
	calls     int
	onCall    func()
}

func (*testAnalyzer) Provider() string { return "OFFLINE_STUB" }
func (a *testAnalyzer) Analyze(_ context.Context, p investigation.Packet) (investigation.Output, error) {
	a.calls++
	if a.onCall != nil {
		a.onCall()
	}
	if a.fail {
		return investigation.Output{}, investigation.ErrProvider
	}
	r := investigation.Offline(p.CaseKind, p.IDs())
	if a.bad {
		r.EvidenceIDs = []string{"foreign-id"}
	}
	return investigation.Output{Result: r, ModelID: "test-offline"}, nil
}
func TestDurableInvestigationJobsAndFailureIsolation(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	secret := "investigation-test-secret-at-least-thirty-two-bytes"
	h := (API{DB: db, Config: config.Config{AppMode: "demo", DemoJWTSecret: secret}}).Handler()
	token, _ := mintToken(secret, "demo-owner", time.Now())
	var caseID, order string
	if err := db.QueryRow(`INSERT INTO merchant_orders(organization_id,external_reference,amount_minor,created_at) VALUES($1,'AI-ORDER',250000,now()) RETURNING id`, demoOrg).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO investigation_cases(organization_id,kind,fingerprint,title,summary,severity) VALUES($1,'SETTLEMENT_NOT_OBSERVED','AI-TEST','Test','Test','MEDIUM') RETURNING id`, demoOrg).Scan(&caseID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO case_evidence(organization_id,case_id,evidence_kind,evidence_id) VALUES($1,$2,'ORDER',$3)`, demoOrg, caseID, order); err != nil {
		t.Fatal(err)
	}
	create := func(key string) map[string]any {
		t.Helper()
		r := httptest.NewRequest("POST", "/v1/cases/"+caseID+"/investigations", strings.NewReader(`{"mode":"EVIDENCE_ANALYSIS"}`))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Organization-Id", demoOrg)
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 202 {
			t.Fatalf("create %d %s", w.Code, w.Body.String())
		}
		var body map[string]any
		json.Unmarshal(w.Body.Bytes(), &body)
		return body
	}
	status := func(id string) string {
		var s string
		if err := db.QueryRow("SELECT status FROM ai_investigations WHERE id=$1", id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	events := func() int {
		var count int
		db.QueryRow("SELECT count(*) FROM case_events WHERE case_id=$1", caseID).Scan(&count)
		return count
	}
	queued := create("investigate-valid")
	if queued["status"] != "QUEUED" || queued["result"] != nil {
		t.Fatal("API computed the result synchronously")
	}
	if create("investigate-valid")["id"] != queued["id"] {
		t.Fatal("same-key job duplicated")
	}
	good := &testAnalyzer{}
	if err := jobs.ProcessOne(ctx, db, good); err != nil {
		t.Fatal(err)
	}
	if status(queued["id"].(string)) != "COMPLETED" || events() != 1 {
		t.Fatal("investigation not completed once")
	}
	// A crash between domain completion and job acknowledgement replays safely.
	if _, err := db.Exec(`UPDATE jobs SET status='RUNNING',lease_expires_at=now()-interval '1 second' WHERE job_key=$1`, "investigation:"+queued["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if err := jobs.ProcessOne(ctx, db, good); err != nil {
		t.Fatal(err)
	}
	if events() != 1 || good.calls != 1 {
		t.Fatal("restart repeated completed analysis")
	}
	if _, err := db.Exec(`UPDATE ai_investigations SET evidence_snapshot_hash='changed' WHERE id=$1`, queued["id"]); err == nil {
		t.Fatal("immutable snapshot changed")
	}
	rejected := create("investigate-rejected")
	bad := &testAnalyzer{bad: true}
	if err := jobs.ProcessOne(ctx, db, bad); err != nil {
		t.Fatal(err)
	}
	if status(rejected["id"].(string)) != "REJECTED" || events() != 1 {
		t.Fatal("rejected output mutated case")
	}
	failed := create("investigate-failed")
	unavailable := &testAnalyzer{fail: true}
	for i := 0; i < 5; i++ {
		if err := jobs.ProcessOne(ctx, db, unavailable); err == nil {
			t.Fatal("provider failure disappeared")
		}
		if _, err := db.Exec(`UPDATE jobs SET next_attempt_at=now() WHERE job_key=$1`, "investigation:"+failed["id"].(string)); err != nil {
			t.Fatal(err)
		}
	}
	if status(failed["id"].(string)) != "FAILED" || events() != 1 || unavailable.calls != 5 {
		t.Fatal("retry budget or failure isolation violated")
	}
	if err := jobs.ProcessOne(ctx, db, unavailable); err != nil {
		t.Fatal(err)
	}
	if unavailable.calls != 5 {
		t.Fatal("failed job is in a hot retry loop")
	}
	// Losing the lease while the model is running must prevent a stale worker write.
	leased := create("investigate-lost-lease")
	lose := &testAnalyzer{onCall: func() {
		if _, err := db.Exec(`UPDATE jobs SET lease_owner='replacement' WHERE job_key=$1`, "investigation:"+leased["id"].(string)); err != nil {
			t.Fatal(err)
		}
	}}
	if err := jobs.ProcessOne(ctx, db, lose); !errors.Is(err, investigation.ErrLease) {
		t.Fatalf("stale lease: %v", err)
	}
	if status(leased["id"].(string)) != "RUNNING" || events() != 1 {
		t.Fatal("stale worker committed")
	}
	db.Exec(`UPDATE jobs SET lease_expires_at=now()-interval '1 second' WHERE job_key=$1`, "investigation:"+leased["id"].(string))
	if err := jobs.ProcessOne(ctx, db, good); err != nil {
		t.Fatal(err)
	}
	if status(leased["id"].(string)) != "COMPLETED" || events() != 2 {
		t.Fatal("expired lease did not recover")
	}
	// A permanently malformed queue item must not loop or call the provider.
	if _, err := db.Exec(`INSERT INTO jobs(organization_id,job_kind,job_key,payload) VALUES($1,'INVESTIGATE','invalid-analysis-job','{"investigation_id":"malformed"}')`, demoOrg); err != nil {
		t.Fatal(err)
	}
	calls := good.calls
	if err := jobs.ProcessOne(ctx, db, good); err == nil {
		t.Fatal("malformed investigation payload was accepted")
	}
	var malformedStatus string
	var malformedAttempts int
	if err := db.QueryRow(`SELECT status,attempts FROM jobs WHERE job_key='invalid-analysis-job'`).Scan(&malformedStatus, &malformedAttempts); err != nil {
		t.Fatal(err)
	}
	if malformedStatus != "FAILED" || malformedAttempts != 1 || good.calls != calls {
		t.Fatal("malformed queue item was retried or invoked the provider")
	}
}
