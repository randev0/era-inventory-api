package internal

import (
	"net/http"
	"strconv"
	"strings"
)

// listParams holds common query parameters for list endpoints
type listParams struct {
	orgID  int64
	limit  int
	offset int
	q      string
	sort   string
}

// parseListParams parses org_id, limit, offset, q, and sort from the request
// Defaults: org_id=1, limit=50 (max 200), offset=0
func parseListParams(r *http.Request) listParams {
	values := r.URL.Query()

	var orgID int64 = 1
	if s := strings.TrimSpace(values.Get("org_id")); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			orgID = v
		}
	}

	limit := 50
	if s := strings.TrimSpace(values.Get("limit")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			if v > 200 {
				v = 200
			}
			limit = v
		}
	}

	offset := 0
	if s := strings.TrimSpace(values.Get("offset")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 {
			offset = v
		}
	}

	return listParams{
		orgID:  orgID,
		limit:  limit,
		offset: offset,
		q:      strings.TrimSpace(values.Get("q")),
		sort:   strings.TrimSpace(values.Get("sort")),
	}
}

// buildOrderBy builds a safe ORDER BY clause using a whitelist of allowed keys.
// allowed maps incoming sort keys (e.g., "name") to actual column identifiers.
// Input sort is comma-separated; prefix with '-' for DESC.
// Returns a string starting with " ORDER BY ...". Defaults to " ORDER BY id ASC".
func buildOrderBy(sortParam string, allowed map[string]string) string {
	if sortParam == "" {
		if col, ok := allowed["id"]; ok {
			return " ORDER BY " + col + " ASC"
		}
		return " ORDER BY id ASC"
	}

	parts := strings.Split(sortParam, ",")
	clauses := make([]string, 0, len(parts))
	for _, raw := range parts {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		desc := false
		if strings.HasPrefix(s, "-") {
			desc = true
			s = strings.TrimPrefix(s, "-")
		}
		col, ok := allowed[s]
		if !ok {
			continue
		}
		if desc {
			clauses = append(clauses, col+" DESC")
		} else {
			clauses = append(clauses, col+" ASC")
		}
	}
	if len(clauses) == 0 {
		if col, ok := allowed["id"]; ok {
			return " ORDER BY " + col + " ASC"
		}
		return " ORDER BY id ASC"
	}
	return " ORDER BY " + strings.Join(clauses, ", ")
}

