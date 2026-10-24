package config

import (
	"strings"
	"testing"
)

func TestLoadRoleIsolation(t *testing.T) {
	t.Setenv("APP_MODE", "demo")
	t.Setenv("DEMO_ACCESS_CODE", "demo-test-access-code")
	t.Setenv("DEMO_JWT_SECRET", "demo-test-signing-secret-at-least-thirty-two-bytes")
	t.Setenv("BANKSIM_ADMIN_TOKEN", "admin-test-token-at-least-thirty-two-bytes")
	t.Setenv("BANKSIM_PROVIDER_TOKEN", "provider-test-token-at-least-thirty-two-bytes")
	t.Setenv("BANKSIM_EVALUATOR_TOKEN", "evaluator-test-token-at-least-thirty-two-bytes")
	t.Setenv("BANKSIM_WEBHOOK_SECRET", "webhook-test-secret-at-least-thirty-two-bytes")
	t.Setenv("PRODUCT_DATABASE_URL", "postgres://product-db/product")
	t.Setenv("BANKSIM_DATABASE_URL", "postgres://banksim-db/banksim")
	t.Setenv("LISTEN_ADDR", "")
	for _, tc := range []struct {
		role Role
		want string
		addr string
	}{
		{API, "postgres://product-db/product", ":8080"},
		{Worker, "postgres://product-db/product", ":8081"},
		{BankSim, "postgres://banksim-db/banksim", ":8082"},
	} {
		got, err := Load(tc.role)
		if err != nil || got.DatabaseURL != tc.want || got.ListenAddr != tc.addr {
			t.Fatalf("Load(%s) = %+v, %v", tc.role, got, err)
		}
	}
}

func TestLoadRejectsMissingAndMalformedDatabase(t *testing.T) {
	t.Setenv("PRODUCT_DATABASE_URL", "")
	if _, err := Load(API); err == nil || !strings.Contains(err.Error(), "PRODUCT_DATABASE_URL") {
		t.Fatalf("missing URL error = %v", err)
	}
	secret := "do-not-log-me"
	t.Setenv("PRODUCT_DATABASE_URL", "postgres://user:"+secret+"@/product")
	if _, err := Load(API); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("malformed URL error exposed secret or was nil: %v", err)
	}
}

func TestProductionAPIFailsClosedWithoutIdentityProvider(t *testing.T) {
	t.Setenv("APP_MODE", "production")
	t.Setenv("PRODUCT_DATABASE_URL", "postgres://product-db/product")
	if _, err := Load(API); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("production API must fail closed: %v", err)
	}
}
