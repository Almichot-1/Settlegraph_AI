package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var eventUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type bankEvent struct {
	EventID           string `json:"event_id"`
	ProviderID        string `json:"provider_id"`
	ScenarioRunID     string `json:"scenario_run_id"`
	Sequence          int64  `json:"sequence"`
	EventType         string `json:"event_type"`
	OccurredAt        string `json:"occurred_at"`
	TransferReference string `json:"transfer_reference"`
	OrderReference    string `json:"order_reference"`
	PaymentReference  string `json:"payment_reference"`
	Amount            struct {
		MinorUnits string `json:"minor_units"`
		Currency   string `json:"currency"`
	} `json:"amount"`
	PayloadVersion int `json:"payload_version"`
}

func (a API) ingestEvent(w http.ResponseWriter, r *http.Request) {
	const maxBody = 256 << 10
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 413, "PAYLOAD_TOO_LARGE", "Event body is too large", reqID(r))
		return
	}
	id := r.Header.Get("X-BankSim-Event-Id")
	stamp := r.Header.Get("X-BankSim-Delivery-Timestamp")
	signature := r.Header.Get("X-BankSim-Signature")
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if !eventUUID.MatchString(id) || err != nil || len(signature) != 67 || !strings.HasPrefix(signature, "v1=") {
		writeError(w, 400, "BAD_REQUEST", "Invalid webhook headers", reqID(r))
		return
	}
	delta := a.now().Unix() - seconds
	if delta > 300 || delta < -300 {
		writeError(w, 401, "UNAUTHORIZED", "Expired webhook delivery", reqID(r))
		return
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "v1="))
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Invalid webhook signature", reqID(r))
		return
	}
	mac := hmac.New(sha256.New, []byte(a.Config.BankSimWebhookSecret))
	_, _ = mac.Write([]byte(stamp + "." + id + "."))
	_, _ = mac.Write(raw)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		writeError(w, 401, "UNAUTHORIZED", "Invalid webhook signature", reqID(r))
		return
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var ev bankEvent
	if d.Decode(&ev) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, "BAD_REQUEST", "Invalid event JSON", reqID(r))
		return
	}
	amount, err := strconv.ParseInt(ev.Amount.MinorUnits, 10, 64)
	if ev.EventID != id || !eventUUID.MatchString(ev.ScenarioRunID) || ev.Sequence < 1 || ev.PayloadVersion != 1 || amount <= 0 || err != nil || ev.Amount.Currency != "ETB" || ev.TransferReference == "" || ev.OrderReference == "" || ev.PaymentReference == "" {
		writeError(w, 400, "BAD_REQUEST", "Invalid event fields", reqID(r))
		return
	}
	if ev.ProviderID != "BANK_A" && ev.ProviderID != "BANK_B" {
		writeError(w, 400, "BAD_REQUEST", "Unknown provider", reqID(r))
		return
	}
	switch ev.EventType {
	case "PAYMENT_ACCEPTED", "SETTLEMENT_POSTED", "REFUND_POSTED", "TRANSFER_FAILED":
	default:
		writeError(w, 400, "BAD_REQUEST", "Unsupported event type", reqID(r))
		return
	}
	occurred, err := time.Parse(time.RFC3339, ev.OccurredAt)
	if err != nil {
		writeError(w, 400, "BAD_REQUEST", "Invalid event time", reqID(r))
		return
	}
	hash := sha256.Sum256(raw)
	tx, err := a.DB.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Ingest database unavailable", reqID(r))
		return
	}
	defer tx.Rollback()
	var org, runID string
	err = tx.QueryRowContext(r.Context(), "SELECT organization_id,id FROM scenario_runs WHERE banksim_run_id=$1", ev.ScenarioRunID).Scan(&org, &runID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 400, "BAD_REQUEST", "Unknown scenario run", reqID(r))
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Ingest database unavailable", reqID(r))
		return
	}
	var inboxID string
	err = tx.QueryRowContext(r.Context(), `INSERT INTO inbox_events(organization_id,provider_id,provider_event_id,source_sequence,payload,payload_sha256)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (organization_id,provider_id,provider_event_id) DO NOTHING RETURNING id`, org, ev.ProviderID, ev.EventID, ev.Sequence, raw, hex.EncodeToString(hash[:])).Scan(&inboxID)
	if errors.Is(err, sql.ErrNoRows) {
		var storedHash string
		if err = tx.QueryRowContext(r.Context(), "SELECT payload_sha256 FROM inbox_events WHERE organization_id=$1 AND provider_id=$2 AND provider_event_id=$3", org, ev.ProviderID, ev.EventID).Scan(&storedHash); err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Ingest persistence failed", reqID(r))
			return
		}
		if storedHash != hex.EncodeToString(hash[:]) {
			writeError(w, 409, "IDEMPOTENCY_CONFLICT", "Event ID reused with different content", reqID(r))
			return
		}
		writeJSON(w, 202, map[string]any{"accepted": true, "duplicate": true})
		return
	}
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Ingest persistence failed", reqID(r))
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO observations(organization_id,inbox_event_id,provider_id,kind,transfer_reference,payment_reference,order_reference,amount_minor,currency,occurred_at,scenario_run_id,source_sequence)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,'ETB',$9,$10,$11)`, org, inboxID, ev.ProviderID, ev.EventType, ev.TransferReference, ev.PaymentReference, ev.OrderReference, amount, occurred.UTC(), runID, ev.Sequence)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Observation persistence failed", reqID(r))
		return
	}
	_, err = tx.ExecContext(r.Context(), "INSERT INTO outbox_events(organization_id,event_type,aggregate_ref,payload) VALUES($1,'OBSERVATION_RECEIVED',$2,$3)", org, inboxID, raw)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Outbox persistence failed", reqID(r))
		return
	}
	if tx.Commit() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Ingest commit failed", reqID(r))
		return
	}
	writeJSON(w, 202, map[string]any{"accepted": true, "duplicate": false})
}
