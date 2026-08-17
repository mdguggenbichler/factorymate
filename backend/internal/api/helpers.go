package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"factorymate/internal/template"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

type paginationParams struct {
	Limit  int
	Offset int
}

func parsePagination(r *http.Request) paginationParams {
	limit := defaultPageLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return paginationParams{Limit: limit, Offset: offset}
}

type timeRange struct {
	From *time.Time
	To   *time.Time
}

func parseTimeRange(r *http.Request) (timeRange, error) {
	var tr timeRange
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return tr, err
		}
		tr.From = &t
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return tr, err
		}
		tr.To = &t
	}
	return tr, nil
}

func parseIDParam(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func nullFloatPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	v := nf.Float64
	return &v
}

func parseJSONColumn(raw string, dest any) error {
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), dest)
}

func renderedToAPI(msg template.RenderedMessage) map[string]any {
	out := map[string]any{}
	if msg.Plain != "" {
		out["plain"] = msg.Plain
	}
	if msg.Embed != nil {
		fields := make([]map[string]any, 0, len(msg.Embed.Fields))
		for _, f := range msg.Embed.Fields {
			fields = append(fields, map[string]any{
				"name":   f.Name,
				"value":  f.Value,
				"inline": f.Inline,
			})
		}
		out["embed"] = map[string]any{
			"title":       msg.Embed.Title,
			"description": msg.Embed.Description,
			"color":       msg.Embed.Color,
			"fields":      fields,
		}
	}
	return out
}

func validationError(w http.ResponseWriter, r *http.Request, err error) {
	if ve, ok := err.(*template.ValidationError); ok {
		writeError(w, r, http.StatusBadRequest, ve.Message)
		return
	}
	writeError(w, r, http.StatusBadRequest, err.Error())
}

func timeNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
