//go:build integration

package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/settlegraph/settlegraph/internal/config"
)

func TestDemoIdentityRoleAndTenantBoundary(t *testing.T) {
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
	const other = "33333333-3333-4333-8333-333333333333"
	_, err = db.Exec(`INSERT INTO app_users(id,auth_subject,display_name) VALUES('44444444-4444-4444-8444-444444444444','demo-viewer','Viewer')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO memberships(organization_id,user_id,role) VALUES($1,'44444444-4444-4444-8444-444444444444','VIEWER')`, org)
	if err != nil {
		t.Fatal(err)
	}
	secret := "test-demo-jwt-secret-with-at-least-32-characters"
	owner, _ := mintToken(secret, "demo-owner", time.Now().UTC())
	viewer, _ := mintToken(secret, "demo-viewer", time.Now().UTC())
	api := API{DB: db, Config: config.Config{AppMode: "demo", DemoJWTSecret: secret, DemoAccessCode: "test-demo-access-code"}}
	handler := api.Handler()
	cases := []struct {
		name, method, path, body, token, organization string
		want                                          int
	}{
		{"missing token", "GET", "/v1/providers", "", "", org, 401},
		{"invalid token", "GET", "/v1/providers", "", "invalid", org, 401},
		{"foreign organization", "GET", "/v1/providers", "", owner, other, 404},
		{"viewer read", "GET", "/v1/providers", "", viewer, org, 200},
		{"viewer cannot advance", "POST", "/v1/demo/scenarios/normal_flow/runs", `{"seed":1}`, viewer, org, 403},
		{"malformed JSON", "POST", "/v1/demo/scenarios/normal_flow/runs", `{"seed":`, owner, org, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			req.Header.Set("X-Organization-Id", tc.organization)
			if tc.method == http.MethodPost {
				req.Header.Set("Idempotency-Key", "test-idempotency-key")
			}
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", out.Code, tc.want, out.Body.String())
			}
		})
	}
	first := httptest.NewRequest("GET", "/v1/providers?limit=1", nil)
	first.Header.Set("Authorization", "Bearer "+viewer)
	first.Header.Set("X-Organization-Id", org)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, first)
	var body struct {
		NextCursor string `json:"next_cursor"`
	}
	if page.Code != 200 || json.Unmarshal(page.Body.Bytes(), &body) != nil || body.NextCursor == "" {
		t.Fatalf("provider cursor missing: %d %s", page.Code, page.Body.String())
	}
	for _, path := range []string{
		"/v1/providers?limit=1&cursor=" + body.NextCursor,
		"/v1/providers?limit=1&provider_id=BANK_A&cursor=" + body.NextCursor,
		"/v1/orders?limit=1&cursor=" + body.NextCursor,
	} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+viewer)
		req.Header.Set("X-Organization-Id", org)
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		want := 400
		if path == "/v1/providers?limit=1&cursor="+body.NextCursor {
			want = 200
		}
		if out.Code != want {
			t.Fatalf("cursor scope path=%s status=%d want=%d", path, out.Code, want)
		}
	}
	accountsReq := httptest.NewRequest("GET", "/v1/accounts?limit=1", nil)
	accountsReq.Header.Set("Authorization", "Bearer "+viewer)
	accountsReq.Header.Set("X-Organization-Id", org)
	accountsPage := httptest.NewRecorder()
	handler.ServeHTTP(accountsPage, accountsReq)
	var accounts struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor"`
	}
	if accountsPage.Code != 200 || json.Unmarshal(accountsPage.Body.Bytes(), &accounts) != nil || len(accounts.Items) != 1 || accounts.NextCursor == "" {
		t.Fatalf("known account page invalid: %d %s", accountsPage.Code, accountsPage.Body.String())
	}
	if accounts.Items[0]["observed_balance"] != nil || accounts.Items[0]["last_observed_at"] != nil {
		t.Fatalf("unobserved balance was presented as known: %v", accounts.Items[0])
	}
	secondReq := httptest.NewRequest("GET", "/v1/accounts?limit=1&cursor="+accounts.NextCursor, nil)
	secondReq.Header.Set("Authorization", "Bearer "+viewer)
	secondReq.Header.Set("X-Organization-Id", org)
	secondPage := httptest.NewRecorder()
	handler.ServeHTTP(secondPage, secondReq)
	if secondPage.Code != 200 {
		t.Fatalf("second account page status=%d body=%s", secondPage.Code, secondPage.Body.String())
	}
}
