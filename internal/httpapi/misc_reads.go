package httpapi

import (
	"net/http"
	"time"
)

func (a API) accounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.tenant(w, r, "OWNER", "ANALYST", "REVIEWER", "VIEWER"); !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"items": []any{}, "next_cursor": nil})
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
