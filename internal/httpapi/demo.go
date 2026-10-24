package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var demoUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type simRun struct {
	ID             string    `json:"id"`
	ScenarioKey    string    `json:"scenario_key"`
	Status         string    `json:"status"`
	Step           int       `json:"step"`
	TotalSteps     int       `json:"total_steps"`
	VirtualNow     time.Time `json:"virtual_now"`
	OrganizationID string    `json:"organization_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func (a API) bankCall(r *http.Request, method, path, key string, payload any) (simRun, int, error) {
	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, strings.TrimRight(a.Config.BankSimAdminURL, "/")+path, body)
	if err != nil {
		return simRun{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Config.BankSimAdminToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return simRun{}, 0, err
	}
	defer resp.Body.Close()
	var run simRun
	if resp.StatusCode >= 300 {
		return run, resp.StatusCode, nil
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&run)
	return run, resp.StatusCode, err
}

func (a API) listDemoScenarios(w http.ResponseWriter, r *http.Request) {
	if a.Config.AppMode != "demo" {
		writeError(w, 404, "NOT_FOUND", "Resource not found", reqID(r))
		return
	}
	if _, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER"); !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"items": []map[string]any{
		{"key": "normal_flow", "title": "Normal synthetic payment", "description": "Payment and settlement are observed", "steps": 2},
		{"key": "delayed_settlement", "title": "Delayed synthetic settlement", "description": "Settlement follows the virtual grace period", "steps": 3},
		{"key": "duplicate_webhook", "title": "Duplicate notification replay", "description": "A notification is delivered twice", "steps": 2},
		{"key": "provider_outage", "title": "Provider outage and recovery", "description": "Settlement visibility is delayed until recovery", "steps": 3},
	}})
}
func (a API) createDemoScenario(w http.ResponseWriter, r *http.Request) {
	if a.Config.AppMode != "demo" {
		writeError(w, 404, "NOT_FOUND", "Resource not found", reqID(r))
		return
	}
	org, ok := a.tenant(w, r, "OWNER")
	if !ok {
		return
	}
	key := r.PathValue("scenarioKey")
	if key != "normal_flow" && key != "delayed_settlement" && key != "duplicate_webhook" && key != "provider_outage" {
		writeError(w, 404, "NOT_FOUND", "Scenario not found", reqID(r))
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if len(idem) < 8 || len(idem) > 128 {
		writeError(w, 400, "BAD_REQUEST", "Idempotency-Key must be 8 to 128 characters", reqID(r))
		return
	}
	var body struct {
		Seed *int `json:"seed"`
	}
	if !strictJSON(w, r, &body) {
		return
	}
	if body.Seed == nil || *body.Seed < 0 {
		writeError(w, 400, "BAD_REQUEST", "seed is required and must be nonnegative", reqID(r))
		return
	}
	bank, status, err := a.bankCall(r, "POST", "/admin/v1/scenarios/"+key+"/runs", idem, map[string]any{"seed": *body.Seed, "organization_id": org})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "BankSim admin unavailable", reqID(r))
		return
	}
	if status >= 300 {
		writeError(w, status, "BANKSIM_ERROR", "BankSim rejected scenario request", reqID(r))
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Product database unavailable", reqID(r))
		return
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(r.Context(), `INSERT INTO scenario_runs(organization_id,banksim_run_id,scenario_key,seed,status,current_step,total_steps,virtual_now)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (banksim_run_id) DO UPDATE SET banksim_run_id=EXCLUDED.banksim_run_id RETURNING id`, org, bank.ID, key, *body.Seed, bank.Status, bank.Step, bank.TotalSteps, bank.VirtualNow).Scan(&id)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Scenario persistence failed", reqID(r))
		return
	}
	orderRef := "ORD-" + itoa(*body.Seed)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO merchant_orders(organization_id,scenario_run_id,external_reference,amount_minor,currency,created_at)
	 VALUES($1,$2,$3,250000,'ETB',$4) ON CONFLICT DO NOTHING`, org, id, orderRef, bank.VirtualNow)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Order seed failed", reqID(r))
		return
	}
	if tx.Commit() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Scenario commit failed", reqID(r))
		return
	}
	bank.ID = id
	bank.OrganizationID = org
	writeJSON(w, 201, bank)
}
func (a API) getDemoScenario(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	id := r.PathValue("scenarioRunId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed run ID", reqID(r))
		return
	}
	var run simRun
	err := a.DB.QueryRowContext(r.Context(), `SELECT id,scenario_key,status,current_step,total_steps,virtual_now,organization_id,created_at FROM scenario_runs WHERE id=$1 AND organization_id=$2`, id, org).
		Scan(&run.ID, &run.ScenarioKey, &run.Status, &run.Step, &run.TotalSteps, &run.VirtualNow, &run.OrganizationID, &run.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Run not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, run)
}
func (a API) advanceDemoScenario(w http.ResponseWriter, r *http.Request) {
	if a.Config.AppMode != "demo" {
		writeError(w, 404, "NOT_FOUND", "Resource not found", reqID(r))
		return
	}
	org, ok := a.tenant(w, r, "OWNER")
	if !ok {
		return
	}
	id := r.PathValue("scenarioRunId")
	if !demoUUID.MatchString(id) {
		writeError(w, 400, "BAD_REQUEST", "Malformed run ID", reqID(r))
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if len(idem) < 8 || len(idem) > 128 {
		writeError(w, 400, "BAD_REQUEST", "Idempotency-Key must be 8 to 128 characters", reqID(r))
		return
	}
	var body struct {
		ExpectedStep *int `json:"expected_step"`
	}
	if !strictJSON(w, r, &body) {
		return
	}
	if body.ExpectedStep == nil || *body.ExpectedStep < 0 {
		writeError(w, 400, "BAD_REQUEST", "expected_step is required", reqID(r))
		return
	}
	var bankID string
	err := a.DB.QueryRowContext(r.Context(), "SELECT banksim_run_id FROM scenario_runs WHERE id=$1 AND organization_id=$2", id, org).Scan(&bankID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "NOT_FOUND", "Run not found", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run unavailable", reqID(r))
		return
	}
	bank, status, err := a.bankCall(r, "POST", "/admin/v1/runs/"+bankID+"/advance", idem, map[string]any{"expected_step": *body.ExpectedStep})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "BankSim admin unavailable", reqID(r))
		return
	}
	if status >= 300 {
		code := "BANKSIM_ERROR"
		if status == 412 {
			code = "PRECONDITION_FAILED"
		}
		writeError(w, status, code, "BankSim rejected advance request", reqID(r))
		return
	}
	_, err = a.DB.ExecContext(r.Context(), "UPDATE scenario_runs SET current_step=$3,status=$4,virtual_now=$5 WHERE id=$1 AND organization_id=$2", id, org, bank.Step, bank.Status, bank.VirtualNow)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Run update failed", reqID(r))
		return
	}
	for _, provider := range []string{"BANK_A", "BANK_B"} {
		if syncErr := a.syncCoverage(r, org, bankID, provider, bank.VirtualNow); syncErr != nil {
			slog.Warn("provider coverage is incomplete", "run_id", id, "provider", provider, "error", syncErr)
			if _, err = a.DB.ExecContext(r.Context(), "INSERT INTO source_status_snapshots(organization_id,provider_id,source_status,scenario_run_id) VALUES($1,$2,'STALE',$3)", org, provider, id); err != nil {
				writeError(w, 503, "SERVICE_UNAVAILABLE", "Coverage status unavailable", reqID(r))
				return
			}
			if _, err = a.DB.ExecContext(r.Context(), "UPDATE provider_sources SET sync_status='STALE' WHERE organization_id=$1 AND id=$2", org, provider); err != nil {
				writeError(w, 503, "SERVICE_UNAVAILABLE", "Coverage status unavailable", reqID(r))
				return
			}
		}
	}
	var cutoff time.Time
	if err = a.DB.QueryRowContext(r.Context(), "SELECT clock_timestamp()").Scan(&cutoff); err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Snapshot clock unavailable", reqID(r))
		return
	}
	jobPayload, _ := json.Marshal(map[string]any{"scenario_run_id": id, "virtual_now": bank.VirtualNow, "cutoff": cutoff})
	_, err = a.DB.ExecContext(r.Context(), `INSERT INTO jobs(organization_id,job_kind,job_key,payload) VALUES($1,'RECONCILE',$2,$3) ON CONFLICT (organization_id,job_kind,job_key) DO NOTHING`, org, id+":"+strconv.Itoa(bank.Step), jobPayload)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Reconciliation queue unavailable", reqID(r))
		return
	}
	bank.ID = id
	bank.OrganizationID = org
	writeJSON(w, 200, bank)
}

func (a API) syncCoverage(r *http.Request, org, bankRun, provider string, virtualNow time.Time) error {
	baseURL := strings.TrimRight(a.Config.BankSimAdminURL, "/") + "/provider/v1/institutions/" + provider + "/events?scenario_run_id=" + bankRun
	var page struct {
		Items []struct {
			EventID       string `json:"event_id"`
			ScenarioRunID string `json:"scenario_run_id"`
			Sequence      int64  `json:"sequence"`
		} `json:"items"`
		NextCursor      *string    `json:"next_cursor"`
		SourceStatus    string     `json:"source_status"`
		CompleteThrough *time.Time `json:"complete_through"`
	}
	var productRun string
	if err := a.DB.QueryRowContext(r.Context(), "SELECT id FROM scenario_runs WHERE organization_id=$1 AND banksim_run_id=$2", org, bankRun).Scan(&productRun); err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Second}
	cursor := ""
	var lastSequence int64
	for pages := 0; pages < 100; pages++ {
		requestURL := baseURL
		if cursor != "" {
			requestURL += "&cursor=" + cursor
		}
		req, err := http.NewRequestWithContext(r.Context(), "GET", requestURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+a.Config.BankSimProviderToken)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return errors.New("provider unavailable")
		}
		page = struct {
			Items []struct {
				EventID       string `json:"event_id"`
				ScenarioRunID string `json:"scenario_run_id"`
				Sequence      int64  `json:"sequence"`
			} `json:"items"`
			NextCursor      *string    `json:"next_cursor"`
			SourceStatus    string     `json:"source_status"`
			CompleteThrough *time.Time `json:"complete_through"`
		}{}
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if page.SourceStatus == "OUTAGE" {
			_, err = a.DB.ExecContext(r.Context(), "INSERT INTO source_status_snapshots(organization_id,provider_id,source_status,complete_through,scenario_run_id) VALUES($1,$2,'OUTAGE',$3,$4)", org, provider, page.CompleteThrough, productRun)
			if err == nil {
				_, err = a.DB.ExecContext(r.Context(), "UPDATE provider_sources SET sync_status='OUTAGE' WHERE organization_id=$1 AND id=$2", org, provider)
			}
			return err
		}
		if page.SourceStatus != "HEALTHY" || page.CompleteThrough == nil {
			return errors.New("provider coverage incomplete")
		}
		for _, event := range page.Items {
			if event.ScenarioRunID != bankRun {
				continue
			}
			if event.Sequence > lastSequence {
				lastSequence = event.Sequence
			}
			var exists bool
			if err = a.DB.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM inbox_events WHERE organization_id=$1 AND provider_id=$2 AND provider_event_id=$3)", org, provider, event.EventID).Scan(&exists); err != nil || !exists {
				return errors.New("provider event not ingested")
			}
		}
		if page.NextCursor == nil {
			break
		}
		if *page.NextCursor == "" || *page.NextCursor == cursor {
			return errors.New("provider cursor did not advance")
		}
		cursor = *page.NextCursor
		if pages == 99 {
			return errors.New("provider pagination limit exceeded")
		}
	}
	through := *page.CompleteThrough
	if virtualNow.Before(through) {
		through = virtualNow
	}
	_, err := a.DB.ExecContext(r.Context(), "INSERT INTO source_status_snapshots(organization_id,provider_id,source_status,complete_through,scenario_run_id) VALUES($1,$2,'HEALTHY',$3,$4)", org, provider, through, productRun)
	if err == nil {
		_, err = a.DB.ExecContext(r.Context(), `UPDATE provider_sources SET sync_status='HEALTHY',last_observed_at=now(),
 cursor_value=CASE WHEN $3::bigint>COALESCE(NULLIF(cursor_value,'')::bigint,0) THEN $3::text ELSE cursor_value END
 WHERE organization_id=$1 AND id=$2`, org, provider, lastSequence)
	}
	return err
}
