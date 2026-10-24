package banksim

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// DeliverLoop retries durable provider events after an API outage or a
// BankSim restart. Product-side event ID deduplication makes retries safe.
func (h HTTP) DeliverLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := h.Ledger.DB.QueryContext(ctx, "SELECT DISTINCT scenario_id FROM sim_events WHERE delivery_status='PENDING' LIMIT 50")
			if err != nil {
				slog.Warn("provider delivery scan failed", "error", err)
				continue
			}
			ids := []string{}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					slog.Warn("provider delivery scan failed", "error", err)
					break
				}
				ids = append(ids, id)
			}
			rows.Close()
			for _, id := range ids {
				attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if err := h.DeliverPending(attemptCtx, id); err != nil {
					slog.Warn("provider event delivery deferred", "run_id", id, "error", err)
				}
				cancel()
			}
		}
	}
}

// DeliverPending sends stored provider observations after the ledger transaction
// commits. A failed delivery remains pending for a later retry or provider backfill.
func (h HTTP) DeliverPending(ctx context.Context, scenarioID string) error {
	if h.Config.ProductWebhookURL == "" {
		return nil
	}
	rows, err := h.Ledger.DB.QueryContext(ctx, `SELECT id,payload,provider_id FROM sim_events WHERE scenario_id=$1 AND delivery_status='PENDING' ORDER BY occurred_at,sequence`, scenarioID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pending struct {
		id       string
		payload  []byte
		provider string
	}
	var events []pending
	for rows.Next() {
		var p pending
		if err = rows.Scan(&p.id, &p.payload, &p.provider); err != nil {
			return err
		}
		events = append(events, p)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	var scenarioKey string
	if err = h.Ledger.DB.QueryRowContext(ctx, "SELECT scenario_key FROM sim_scenarios WHERE id=$1", scenarioID).Scan(&scenarioKey); err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Second}
	for _, event := range events {
		attempts := 1
		if scenarioKey == "duplicate_webhook" && event.provider == "BANK_A" {
			attempts = 2
		}
		for i := 0; i < attempts; i++ {
			stamp := strconv.FormatInt(time.Now().Unix(), 10)
			mac := hmac.New(sha256.New, []byte(h.Config.BankSimWebhookSecret))
			_, _ = mac.Write([]byte(stamp + "." + event.id + "."))
			_, _ = mac.Write(event.payload)
			req, err := http.NewRequestWithContext(ctx, "POST", h.Config.ProductWebhookURL, bytes.NewReader(event.payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-BankSim-Event-Id", event.id)
			req.Header.Set("X-BankSim-Delivery-Timestamp", stamp)
			req.Header.Set("X-BankSim-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != 202 {
				return fmt.Errorf("webhook delivery returned %d", resp.StatusCode)
			}
		}
		result, err := h.Ledger.DB.ExecContext(ctx, "UPDATE sim_events SET delivery_status='DELIVERED',delivered_at=now(),attempts=attempts+$2 WHERE id=$1", event.id, attempts)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return errors.New("event disappeared during delivery")
		}
	}
	return nil
}
