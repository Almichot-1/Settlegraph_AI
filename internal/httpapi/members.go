package httpapi

import "net/http"

// members exposes only active tenant members to users allowed to assign cases.
func (a API) members(w http.ResponseWriter, r *http.Request) {
	org, ok := a.tenant(w, r, "OWNER", "ANALYST")
	if !ok {
		return
	}
	limit, cursor, ok := a.limitCursor(w, r, org)
	if !ok {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT u.id,u.display_name,m.role
 FROM memberships m JOIN app_users u ON u.id=m.user_id
 WHERE m.organization_id=$1 AND m.active AND ($2='' OR u.id::text>$2)
 ORDER BY u.id LIMIT $3`, org, cursor, limit+1)
	if err != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Members unavailable", reqID(r))
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var id, name, role string
		if rows.Scan(&id, &name, &role) != nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Members unavailable", reqID(r))
			return
		}
		if len(items) == limit {
			next = a.nextCursor(r, org, items[len(items)-1]["id"].(string))
			break
		}
		items = append(items, map[string]any{"id": id, "display_name": name, "role": role})
	}
	if rows.Err() != nil {
		writeError(w, 503, "SERVICE_UNAVAILABLE", "Members unavailable", reqID(r))
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}
