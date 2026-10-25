//go:build integration

package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/settlegraph/settlegraph/internal/config"
)

func integrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PRODUCT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PRODUCT_TEST_DATABASE_URL required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSharedIdentityFiltersAndHistory(t *testing.T) {
	db := integrationDB(t)
	secret := "integration-secret-with-at-least-thirty-two-bytes"
	cfg := config.Config{AppMode: "demo", DemoJWTSecret: secret, DemoAccessCode: "owner-access-code", DemoRoleCodes: map[string]string{"demo-reviewer": "reviewer-access-code", "demo-analyst": "analyst-access-code", "demo-viewer": "viewer-access-code"}}
	h := (API{DB: db, Config: cfg}).Handler()
	call := func(method, path, principal, body, code, version string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if principal != "" {
			token, _ := mintToken(secret, principal, time.Now())
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Organization-Id", demoOrg)
		r.Header.Set("X-Demo-Access-Code", code)
		r.Header.Set("Idempotency-Key", "integration-"+method+"-"+principal+"-"+version)
		if version != "" {
			r.Header.Set("If-Match", `"`+version+`"`)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		var data map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	get := func(path string) map[string]any { return call("GET", path, "demo-owner", "", "", "", 200) }
	owner := call("POST", "/v1/demo/sessions", "", `{"display_name":"Owner input"}`, cfg.DemoAccessCode, "", 201)
	reviewer := call("POST", "/v1/demo/sessions", "", `{"display_name":"Reviewer input"}`, cfg.DemoRoleCodes["demo-reviewer"], "", 201)
	if owner["user"].(map[string]any)["id"] == reviewer["user"].(map[string]any)["id"] {
		t.Fatal("roles share one identity")
	}
	if reviewer["user"].(map[string]any)["display_name"] != "Demo Reviewer" {
		t.Fatal("client renamed server-owned principal")
	}
	call("POST", "/v1/demo/sessions", "", `{"display_name":"OWNER","role":"OWNER"}`, cfg.DemoRoleCodes["demo-reviewer"], "", 400)
	call("POST", "/v1/demo/sessions", "", `{"display_name":"x"}`, "bad-code", "", 401)
	call("GET", "/v1/members", "demo-reviewer", "", "", "", 403)
	call("GET", "/v1/members", "demo-viewer", "", "", "", 403)
	members := get("/v1/members?limit=2")
	if len(members["items"].([]any)) != 2 || members["next_cursor"] == nil {
		t.Fatal("members not paged")
	}
	next := get("/v1/members?limit=2&cursor=" + url.QueryEscape(members["next_cursor"].(string)))
	if len(next["items"].([]any)) != 2 || next["next_cursor"] != nil {
		t.Fatal("members last page wrong")
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	rowID := func(q string, args ...any) string {
		t.Helper()
		var id string
		if err := db.QueryRow(q, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	scenario := rowID(`INSERT INTO scenario_runs(organization_id,scenario_key,seed,total_steps,virtual_now) VALUES($1,'normal_flow',999,2,now()) RETURNING id`, demoOrg)
	run := rowID(`INSERT INTO reconciliation_runs(organization_id,scope,rules_version,status,snapshot_cutoff,job_key) VALUES($1,'LATEST_OBSERVATIONS','v1','COMPLETED',now(),'test-matches') RETURNING id`, demoOrg)
	for i := 0; i < 4; i++ {
		provider := "BANK_A"
		var scope any = scenario
		if i == 2 {
			provider = "BANK_B"
		}
		if i == 3 {
			scope = nil
		}
		inbox := rowID(`INSERT INTO inbox_events(organization_id,provider_id,provider_event_id,source_sequence,payload,payload_sha256) VALUES($1,$2,gen_random_uuid(),$3,'{}','hash') RETURNING id`, demoOrg, provider, i+1)
		observation := rowID(`INSERT INTO observations(organization_id,inbox_event_id,provider_id,kind,transfer_reference,amount_minor,occurred_at,source_sequence,scenario_run_id) VALUES($1,$2,$3,'PAYMENT_ACCEPTED',$4,100,'2026-10-24T09:00:00Z',$5,$6) RETURNING id`, demoOrg, inbox, provider, fmt.Sprint("PAY-", i), i+1, scope)
		order := rowID(`INSERT INTO merchant_orders(organization_id,external_reference,amount_minor,created_at) VALUES($1,$2,100,now()) RETURNING id`, demoOrg, fmt.Sprint("ORDER-", i))
		exec(`INSERT INTO match_edges(organization_id,run_id,stage,order_id,payment_observation_id,allocated_minor,rule,proof) VALUES($1,$2,'ORDER_PAYMENT',$3,$4,100,'test','{}')`, demoOrg, run, order, observation)
	}
	base := "/v1/transactions?limit=1&provider_id=BANK_A&scenario_run_id=" + scenario
	page := get(base)
	if page["next_cursor"] == nil {
		t.Fatal("filtered transactions not paged")
	}
	final := get(base + "&cursor=" + url.QueryEscape(page["next_cursor"].(string)))
	if len(final["items"].([]any)) != 1 || final["next_cursor"] != nil {
		t.Fatalf("filters ignored: %v", final)
	}
	for _, p := range []map[string]any{page, final} {
		v := p["items"].([]any)[0].(map[string]any)
		if v["provider_id"] != "BANK_A" || v["scenario_run_id"] != scenario {
			t.Fatal("filter mismatch")
		}
	}
	call("GET", "/v1/transactions?scenario_run_id=bad", "demo-owner", "", "", "", 400)
	call("GET", "/v1/transactions?provider_id=BANK_B&cursor="+url.QueryEscape(page["next_cursor"].(string)), "demo-owner", "", "", "", 400)
	series := get("/v1/dashboard/series?from=2026-10-24T09:00:00Z&to=2026-10-24T10:00:00Z&granularity=HOUR")
	if len(series["points"].([]any)) != 1 || series["points"].([]any)[0].(map[string]any)["payments_accepted"].(map[string]any)["minor_units"] != "400" {
		t.Fatalf("series range wrong: %v", series)
	}
	outside := get("/v1/dashboard/series?from=2026-10-24T08:00:00Z&to=2026-10-24T09:00:00Z&granularity=HOUR")
	if outside["points"].([]any)[0].(map[string]any)["payments_accepted"].(map[string]any)["minor_units"] != "0" {
		t.Fatal("to boundary overlaps")
	}
	call("GET", "/v1/dashboard/series?from=2020-01-01T00:00:00Z&to=2026-01-01T00:00:00Z", "demo-owner", "", "", "", 400)
	caseID := rowID(`INSERT INTO investigation_cases(organization_id,kind,fingerprint,title,summary,severity,version) VALUES($1,'SETTLEMENT_NOT_OBSERVED','history-test','Test','Test','MEDIUM',110) RETURNING id`, demoOrg)
	for i := 1; i <= 110; i++ {
		exec(`INSERT INTO case_events(organization_id,case_id,version,action,actor,message) VALUES($1,$2,$3,'EVIDENCE_ADDED','test','test')`, demoOrg, caseID, i)
		exec(`INSERT INTO audit_events(organization_id,actor,action,resource_type,resource_id,occurred_at) VALUES($1,'test','OBSERVED','CASE',$2,'2026-10-24T09:00:00Z'::timestamptz+($3*interval '1 second'))`, demoOrg, caseID, i)
	}
	for path, want := range map[string]int{"/v1/cases/" + caseID + "/events": 110, "/v1/audit-events": 110, "/v1/reconciliation/runs/" + run + "/matches": 4} {
		seen := map[string]bool{}
		cursor := ""
		lastVersion := 0
		for pages := 0; pages < 10; pages++ {
			p := get(path + "?limit=17&cursor=" + url.QueryEscape(cursor))
			if len(p["items"].([]any)) > 17 {
				t.Fatal("limit ignored")
			}
			for _, item := range p["items"].([]any) {
				v := item.(map[string]any)
				id := v["id"].(string)
				if seen[id] {
					t.Fatal("duplicate page item")
				}
				seen[id] = true
				if version, ok := v["version"].(float64); ok {
					if int(version) <= lastVersion {
						t.Fatal("case events reordered")
					}
					lastVersion = int(version)
				}
			}
			if p["next_cursor"] == nil {
				break
			}
			cursor = p["next_cursor"].(string)
		}
		if len(seen) != want {
			t.Fatalf("%s lost history: got %d want %d", path, len(seen), want)
		}
	}
	call(http.MethodPost, "/v1/cases/"+caseID+"/decisions", "demo-analyst", `{"decision":"RESOLVE","reason":"Reviewed evidence"}`, "", "110", 403)
	call(http.MethodPost, "/v1/cases/"+caseID+"/decisions", "demo-reviewer", `{"decision":"RESOLVE","reason":"Reviewed evidence"}`, "", "109", 412)
	result := call(http.MethodPost, "/v1/cases/"+caseID+"/decisions", "demo-reviewer", `{"decision":"RESOLVE","reason":"Reviewed evidence"}`, "", "110", 200)
	retry := call(http.MethodPost, "/v1/cases/"+caseID+"/decisions", "demo-reviewer", `{"decision":"RESOLVE","reason":"Reviewed evidence"}`, "", "110", 200)
	if result["version"] != retry["version"] || result["status"] != "RESOLVED" {
		t.Fatal("decision retry duplicated")
	}
}
