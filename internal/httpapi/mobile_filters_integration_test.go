//go:build integration

package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/settlegraph/settlegraph/internal/config"
)

func TestMobileCaseFiltersAndInvestigationPagination(t *testing.T) {
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
	const run = "99999999-9999-4999-8999-999999999999"
	const other = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	var owner string
	if err = db.QueryRow("SELECT id FROM app_users WHERE auth_subject='demo-owner'").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO scenario_runs(id,organization_id,scenario_key,seed,total_steps,virtual_now) VALUES($1,$2,'delayed_settlement',102,3,now())`, run, org); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO organizations(id,name,mode) VALUES($1,'Other tenant','DEMO')`, other); err != nil {
		t.Fatal(err)
	}
	ids := []string{"10000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000002", "10000000-0000-4000-8000-000000000003", "10000000-0000-4000-8000-000000000004", "10000000-0000-4000-8000-000000000005", "10000000-0000-4000-8000-000000000006"}
	for i, id := range ids {
		status := "OPEN"
		var assignee, scenario any = owner, run
		tenant := org
		switch i {
		case 0:
			status = "RESOLVED"
		case 2:
			assignee = nil
		case 4:
			scenario = nil
		case 5:
			tenant = other
			assignee = nil
			scenario = nil
		}
		_, err = db.Exec(`INSERT INTO investigation_cases(id,organization_id,scenario_run_id,kind,fingerprint,title,summary,status,severity,assigned_to) VALUES($1,$2,$3,'SETTLEMENT_NOT_OBSERVED',$6,'Filter test','Synthetic filter test',$4,'MEDIUM',$5)`, id, tenant, scenario, status, assignee, id)
		if err != nil {
			t.Fatal(err)
		}
	}
	secret := "mobile-filter-test-secret-at-least-32-characters"
	token, _ := mintToken(secret, "demo-owner", time.Now().UTC())
	handler := (API{DB: db, Config: config.Config{AppMode: "demo", DemoJWTSecret: secret}}).Handler()
	get := func(path string, want int) map[string]any {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-Id", org)
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("%s: status=%d want=%d body=%s", path, out.Code, want, out.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	base := "/v1/cases?limit=1&status=OPEN&assigned_to=" + owner + "&scenario_run_id=" + run
	page := get(base, 200)
	items := page["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != ids[1] {
		t.Fatalf("filter applied after pagination or ignored: %v", page)
	}
	cursor, ok := page["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("next cursor missing: %v", page)
	}
	next := get(base+"&cursor="+url.QueryEscape(cursor), 200)
	if next["next_cursor"] != nil || len(next["items"].([]any)) != 1 || next["items"].([]any)[0].(map[string]any)["id"] != ids[3] {
		t.Fatalf("wrong second page: %v", next)
	}
	get("/v1/cases?status=RESOLVED&cursor="+url.QueryEscape(cursor), 400)
	get("/v1/cases?status=UNKNOWN", 400)
	get("/v1/cases?assigned_to=bad", 400)
	get("/v1/cases?scenario_run_id=bad", 400)
	get("/v1/cases/"+ids[5], 404)
	get("/v1/cases/"+ids[5]+"/investigations", 404)
	filtered := get("/v1/cases?assigned_to="+owner+"&status=RESOLVED", 200)
	if len(filtered["items"].([]any)) != 1 {
		t.Fatalf("status/assignee filters failed: %v", filtered)
	}
	for _, id := range []string{"20000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000002", "20000000-0000-4000-8000-000000000003"} {
		_, err = db.Exec(`INSERT INTO ai_investigations(id,organization_id,case_id,status,provider,evidence_snapshot_hash,result,completed_at) VALUES($1,$2,$3,'COMPLETED','OFFLINE_STUB','snapshot','{"summary":"Synthetic","hypotheses":[],"recommended_actions":[],"evidence_ids":[],"limitations":[]}',now())`, id, org, ids[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	path := "/v1/cases/" + ids[1] + "/investigations?limit=2"
	first := get(path, 200)
	if len(first["items"].([]any)) != 2 || first["next_cursor"] == nil {
		t.Fatalf("investigation pagination ignored: %v", first)
	}
	last := get(path+"&cursor="+url.QueryEscape(first["next_cursor"].(string)), 200)
	if len(last["items"].([]any)) != 1 || last["next_cursor"] != nil {
		t.Fatalf("last investigation page incorrect: %v", last)
	}
	get("/v1/cases/"+ids[3]+"/investigations?cursor="+url.QueryEscape(first["next_cursor"].(string)), 400)
}
