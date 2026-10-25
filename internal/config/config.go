package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Role names the independently deployed process and its one allowed database.
type Role string

const (
	API     Role = "api"
	Worker  Role = "worker"
	BankSim Role = "banksim"
)

type Config struct {
	InvestigationProvider string
	BedrockModelID        string
	AWSRegion             string
	Role                  Role
	ListenAddr            string
	DatabaseURL           string
	AppMode               string
	DemoAccessCode        string
	DemoRoleCodes         map[string]string
	AuthRateLimit         int
	ExpensiveRateLimit    int
	DemoJWTSecret         string
	BankSimAdminToken     string
	BankSimProviderToken  string
	BankSimEvaluatorToken string
	BankSimWebhookSecret  string
	BankSimAdminURL       string
	ProductWebhookURL     string
}

func Load(role Role) (Config, error) {
	var key, defaultAddr string
	switch role {
	case API:
		key, defaultAddr = "PRODUCT_DATABASE_URL", ":8080"
	case Worker:
		key, defaultAddr = "PRODUCT_DATABASE_URL", ":8081"
	case BankSim:
		key, defaultAddr = "BANKSIM_DATABASE_URL", ":8082"
	default:
		return Config{}, errors.New("unknown service role")
	}

	dsn := strings.TrimSpace(os.Getenv(key))
	if dsn == "" {
		return Config{}, fmt.Errorf("%s is required", key)
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Path == "" || u.Path == "/" {
		return Config{}, fmt.Errorf("%s must be a PostgreSQL URL with host and database", key)
	}
	// Never include the URL in an error; it may contain a password.
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "" {
		return Config{}, errors.New("LISTEN_ADDR must be host:port")
	}
	mode := os.Getenv("APP_MODE")
	if mode == "" {
		mode = "production"
	}
	if mode != "demo" && mode != "production" {
		return Config{}, errors.New("APP_MODE must be demo or production")
	}
	cfg := Config{Role: role, ListenAddr: addr, DatabaseURL: dsn, AppMode: mode}
	if role == API || role == Worker {
		cfg.InvestigationProvider = os.Getenv("INVESTIGATION_PROVIDER")
		if cfg.InvestigationProvider == "" {
			cfg.InvestigationProvider = "OFFLINE_STUB"
		}
		if cfg.InvestigationProvider != "OFFLINE_STUB" && cfg.InvestigationProvider != "BEDROCK" {
			return Config{}, errors.New("INVESTIGATION_PROVIDER must be OFFLINE_STUB or BEDROCK")
		}
		cfg.BedrockModelID = os.Getenv("BEDROCK_MODEL_ID")
		cfg.AWSRegion = os.Getenv("AWS_REGION")
		if cfg.InvestigationProvider == "BEDROCK" && (cfg.BedrockModelID == "" || cfg.AWSRegion == "") {
			return Config{}, errors.New("explicit Bedrock mode requires BEDROCK_MODEL_ID and AWS_REGION")
		}
	}
	if role == API {
		for key, dest := range map[string]*int{"DEMO_AUTH_RATE_LIMIT": &cfg.AuthRateLimit, "EXPENSIVE_RATE_LIMIT": &cfg.ExpensiveRateLimit} {
			if raw := os.Getenv(key); raw != "" {
				n, err := strconv.Atoi(raw)
				if err != nil || n < 1 || n > 10000 {
					return Config{}, errors.New("invalid request rate limit")
				}
				*dest = n
			}
		}
	}
	if role == API && mode == "production" {
		return Config{}, errors.New("production API authentication is not configured")
	}
	if role == API && mode == "demo" {
		cfg.DemoAccessCode = os.Getenv("DEMO_ACCESS_CODE")
		cfg.DemoRoleCodes = map[string]string{}
		seen := map[string]bool{cfg.DemoAccessCode: true}
		for _, role := range []string{"reviewer", "analyst", "viewer"} {
			code := os.Getenv("DEMO_" + strings.ToUpper(role) + "_ACCESS_CODE")
			if code != "" {
				if len(code) < 16 || seen[code] {
					return Config{}, errors.New("demo role codes must be distinct and at least 16 characters")
				}
				seen[code] = true
				cfg.DemoRoleCodes["demo-"+role] = code
			}
		}
		cfg.DemoJWTSecret = os.Getenv("DEMO_JWT_SECRET")
		cfg.BankSimAdminToken = os.Getenv("BANKSIM_ADMIN_TOKEN")
		cfg.BankSimProviderToken = os.Getenv("BANKSIM_PROVIDER_TOKEN")
		cfg.BankSimWebhookSecret = os.Getenv("BANKSIM_WEBHOOK_SECRET")
		cfg.BankSimAdminURL = os.Getenv("BANKSIM_ADMIN_URL")
		if len(cfg.DemoAccessCode) < 16 || len(cfg.DemoJWTSecret) < 32 || len(cfg.BankSimAdminToken) < 32 || len(cfg.BankSimProviderToken) < 32 || len(cfg.BankSimWebhookSecret) < 32 {
			return Config{}, errors.New("demo API credentials are required and must be at least 32 characters except access code (16)")
		}
	}
	if role == BankSim {
		cfg.BankSimAdminToken = os.Getenv("BANKSIM_ADMIN_TOKEN")
		cfg.BankSimProviderToken = os.Getenv("BANKSIM_PROVIDER_TOKEN")
		cfg.BankSimEvaluatorToken = os.Getenv("BANKSIM_EVALUATOR_TOKEN")
		cfg.BankSimWebhookSecret = os.Getenv("BANKSIM_WEBHOOK_SECRET")
		cfg.ProductWebhookURL = os.Getenv("PRODUCT_WEBHOOK_URL")
		if len(cfg.BankSimAdminToken) < 32 || len(cfg.BankSimProviderToken) < 32 || len(cfg.BankSimEvaluatorToken) < 32 || len(cfg.BankSimWebhookSecret) < 32 {
			return Config{}, errors.New("BankSim demo tokens and webhook secret (32+ chars) are required")
		}
	}
	return cfg, nil
}
