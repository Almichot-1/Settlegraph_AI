package httpapi

import (
	"database/sql"
	"net/http"
	"time"
)

func (a API) accounts(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,provider_id,account_label,account_type,last_observed_at,observed_balance_minor
 FROM business_accounts WHERE organization_id=$1 AND ($2='' OR id::text>$2) ORDER BY id LIMIT $3`, org, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Accounts unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var id, provider, label, kind string
		var observed sql.NullTime
		var balance sql.NullInt64
		if err := rows.Scan(&id, &provider, &label, &kind, &observed, &balance); err != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Accounts unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		var observedAt any
		if observed.Valid {
			observedAt = observed.Time.UTC().Format(time.RFC3339)
		}
		item := map[string]any{"id": id, "provider_id": provider, "account_label": label,
			"account_type": kind, "last_observed_at": observedAt}
		if balance.Valid {
			item["observed_balance"] = money(balance.Int64)
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Accounts unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}

func (a API) dashboardSeries(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER")
	if !ok {
		return
	}
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "DAY"
	}
	if granularity != "DAY" && granularity != "HOUR" {
		writeError(w, 400, "BAD_REQUEST", "granularity must be DAY or HOUR", reqID(r))
		return
	}
	trunc := "day"
	if granularity == "HOUR" {
		trunc = "hour"
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT date_trunc($2,occurred_at),COALESCE(sum(amount_minor) FILTER (WHERE kind='PAYMENT_ACCEPTED'),0),COALESCE(sum(amount_minor) FILTER (WHERE kind='SETTLEMENT_POSTED'),0) FROM observations WHERE organization_id=$1 GROUP BY 1 ORDER BY 1`, org, trunc)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Series unavailable", reqID(r))
		return
	}
	defer rows.Close()
	points := []map[string]any{}
	for rows.Next() {
		var bucket time.Time
		var payments, settlements int64
		if rows.Scan(&bucket, &payments, &settlements) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Series unavailable", reqID(r))
			return
		}
		points = append(points, map[string]any{"bucket_start": bucket.UTC().Format(time.RFC3339), "payments_accepted": money(payments), "settlements_observed": money(settlements)})
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Series unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"granularity": granularity, "points": points})
}

func (a API) listImports(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER"); !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"items": []any{}, "next_cursor": nil})
}
