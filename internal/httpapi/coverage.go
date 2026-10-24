package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// CoverageLoop repairs stale source windows after BankSim retries a webhook.
// A window becomes HEALTHY only after every provider event is in the durable inbox.
func (a API) CoverageLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := a.DB.QueryContext(ctx, `WITH latest AS (
 SELECT DISTINCT ON (organization_id,scenario_run_id,provider_id)
 organization_id,scenario_run_id,provider_id,source_status
 FROM source_status_snapshots WHERE scenario_run_id IS NOT NULL
 ORDER BY organization_id,scenario_run_id,provider_id,observed_at DESC,id DESC)
 SELECT l.organization_id,l.scenario_run_id,s.banksim_run_id,l.provider_id,s.virtual_now
 FROM latest l JOIN scenario_runs s ON s.organization_id=l.organization_id AND s.id=l.scenario_run_id
 WHERE l.source_status='STALE' ORDER BY s.created_at LIMIT 50`)
			if err != nil {
				slog.Warn("coverage repair scan failed", "error", err)
				continue
			}
			type pending struct {
				org, runID, bankRun, provider string
				virtualNow                    time.Time
			}
			items := []pending{}
			for rows.Next() {
				var item pending
				if err := rows.Scan(&item.org, &item.runID, &item.bankRun, &item.provider, &item.virtualNow); err != nil {
					slog.Warn("coverage repair scan failed", "error", err)
					break
				}
				items = append(items, item)
			}
			rows.Close()
			for _, item := range items {
				attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				req, _ := http.NewRequestWithContext(attemptCtx, http.MethodGet, "http://coverage.local/", nil)
				if err := a.syncCoverage(req, item.org, item.bankRun, item.provider, item.virtualNow); err != nil {
					slog.Warn("coverage repair deferred", "run_id", item.bankRun, "provider", item.provider, "error", err)
				} else {
					var status string
					err := a.DB.QueryRowContext(attemptCtx, `SELECT source_status FROM source_status_snapshots WHERE organization_id=$1 AND scenario_run_id=$2 AND provider_id=$3 ORDER BY observed_at DESC,id DESC LIMIT 1`, item.org, item.runID, item.provider).Scan(&status)
					if err == nil && status == "HEALTHY" {
						var cutoff time.Time
						err = a.DB.QueryRowContext(attemptCtx, "SELECT clock_timestamp()").Scan(&cutoff)
						if err == nil {
							payload, _ := json.Marshal(map[string]any{"scenario_run_id": item.runID, "virtual_now": item.virtualNow, "cutoff": cutoff})
							_, err = a.DB.ExecContext(attemptCtx, `INSERT INTO jobs(organization_id,job_kind,job_key,payload) VALUES($1,'RECONCILE',$2,$3) ON CONFLICT (organization_id,job_kind,job_key) DO NOTHING`, item.org, "coverage:"+item.runID+":"+item.provider+":"+item.virtualNow.Format(time.RFC3339Nano), payload)
						}
					}
					if err != nil {
						slog.Warn("coverage repair enqueue failed", "run_id", item.bankRun, "provider", item.provider, "error", err)
					}
				}
				cancel()
			}
		}
	}
}
