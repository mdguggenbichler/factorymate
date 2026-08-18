package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"factorymate/internal/auth"
	"factorymate/internal/frm"
	"factorymate/internal/notify"
	"factorymate/internal/template"

	"github.com/go-chi/chi/v5"
)

type discordTargetConfig struct {
	ChannelID string `json:"channel_id"`
	ThreadID  string `json:"thread_id,omitempty"`
}

type notificationTargetRequest struct {
	Name         string              `json:"name"`
	ProviderType string              `json:"providerType"`
	Config       discordTargetConfig `json:"config"`
	Enabled      *bool               `json:"enabled"`
}

type enabledRequest struct {
	Enabled bool `json:"enabled"`
}

type templatePreviewRequest struct {
	Variant  string            `json:"variant"`
	Template template.Template   `json:"template"`
}

type targetIDsRequest struct {
	TargetIDs []int64 `json:"targetIds"`
}

type settingsResponse struct {
	ServerName                        string `json:"serverName"`
	FRMHost                           string `json:"frmHost"`
	FRMPort                           int    `json:"frmPort"`
	FRMAuthToken                      string `json:"frmAuthToken"`
	PollIntervalSeconds               int    `json:"pollIntervalSeconds"`
	ProductionSnapshotIntervalSeconds int    `json:"productionSnapshotIntervalSeconds"`
	ProductionSnapshotRetentionDays   int    `json:"productionSnapshotRetentionDays"`
}

type updateUserRequest struct {
	Role         *auth.Role      `json:"role"`
	Password     *string         `json:"password"`
	PlayerIDJSON json.RawMessage `json:"playerId"`
}

type frmTestRequest struct {
	FRMHost      string `json:"frmHost"`
	FRMPort      int    `json:"frmPort"`
	FRMAuthToken string `json:"frmAuthToken"`
}

func (h *Handler) ListNotificationTargets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, provider_type, config_json, enabled, created_at
		FROM notification_targets ORDER BY name`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	targets := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var name, providerType, configJSON, createdAt string
		var enabled bool
		if err := rows.Scan(&id, &name, &providerType, &configJSON, &enabled, &createdAt); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var config map[string]any
		_ = parseJSONColumn(configJSON, &config)
		targets = append(targets, map[string]any{
			"id":           id,
			"name":         name,
			"providerType": providerType,
			"config":       config,
			"enabled":      enabled,
			"createdAt":    createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": targets})
}

func (h *Handler) CreateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	var req notificationTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required")
		return
	}
	if req.ProviderType != "discord" {
		writeError(w, r, http.StatusBadRequest, "unsupported provider type")
		return
	}
	if strings.TrimSpace(req.Config.ChannelID) == "" {
		writeError(w, r, http.StatusBadRequest, "channel_id is required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if err := h.validateDiscordTargetChannel(r.Context(), string(configJSON)); err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	now := timeNowRFC3339()
	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO notification_targets (name, provider_type, config_json, enabled, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		req.Name, req.ProviderType, string(configJSON), enabled, now,
	)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	id, _ := res.LastInsertId()

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           id,
		"name":         req.Name,
		"providerType": req.ProviderType,
		"config":       req.Config,
		"enabled":      enabled,
		"createdAt":    now,
	})
}

func (h *Handler) UpdateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	var req notificationTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	var name, providerType, configJSON string
	var enabled bool
	err = h.db.QueryRowContext(r.Context(), `
		SELECT name, provider_type, config_json, enabled FROM notification_targets WHERE id = ?`, id,
	).Scan(&name, &providerType, &configJSON, &enabled)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	if strings.TrimSpace(req.Name) != "" {
		name = strings.TrimSpace(req.Name)
	}
	if req.ProviderType != "" && req.ProviderType != "discord" {
		writeError(w, r, http.StatusBadRequest, "unsupported provider type")
		return
	}
	if req.ProviderType != "" {
		providerType = req.ProviderType
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var existing discordTargetConfig
	_ = json.Unmarshal([]byte(configJSON), &existing)
	if strings.TrimSpace(req.Config.ChannelID) != "" {
		existing.ChannelID = req.Config.ChannelID
	}
	if req.Config.ThreadID != "" {
		existing.ThreadID = req.Config.ThreadID
	}
	newConfigJSON, err := json.Marshal(existing)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if err := h.validateDiscordTargetChannel(r.Context(), string(newConfigJSON)); err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	_, err = h.db.ExecContext(r.Context(), `
		UPDATE notification_targets SET name = ?, provider_type = ?, config_json = ?, enabled = ?
		WHERE id = ?`, name, providerType, string(newConfigJSON), enabled, id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var config map[string]any
	_ = parseJSONColumn(string(newConfigJSON), &config)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           id,
		"name":         name,
		"providerType": providerType,
		"config":       config,
		"enabled":      enabled,
	})
}

func (h *Handler) DeleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}
	res, err := h.db.ExecContext(r.Context(), `DELETE FROM notification_targets WHERE id = ?`, id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	var name, providerType, configJSON string
	var enabled bool
	err = h.db.QueryRowContext(r.Context(), `
		SELECT name, provider_type, config_json, enabled FROM notification_targets WHERE id = ?`, id,
	).Scan(&name, &providerType, &configJSON, &enabled)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	target := notify.NotificationTarget{
		ID:           id,
		Name:         name,
		ProviderType: providerType,
		ConfigJSON:   configJSON,
		Enabled:      enabled,
	}
	if err := h.validateDiscordTargetChannel(r.Context(), configJSON); err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.dispatcher.DeliverToTarget(r.Context(), "target_test", target, notify.SampleTemplateRenderedMessage()); err != nil {
		writeError(w, r, http.StatusBadRequest, notify.FormatDiscordSendError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMessageTypes(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT key, label, category, enabled, default_template_json, variables_json
		FROM message_types ORDER BY category, label`)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	type messageTypeRow struct {
		key, label, category, defaultJSON, variablesJSON string
		enabled                                          bool
	}
	rawRows := make([]messageTypeRow, 0)
	for rows.Next() {
		var row messageTypeRow
		if err := rows.Scan(&row.key, &row.label, &row.category, &row.enabled, &row.defaultJSON, &row.variablesJSON); err != nil {
			rows.Close()
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		rawRows = append(rawRows, row)
	}
	if err := rows.Close(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	types := make([]map[string]any, 0, len(rawRows))
	for _, row := range rawRows {
		effective, err := h.loadEffectiveTemplate(r.Context(), row.key, row.defaultJSON)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		targetIDs, err := h.loadMessageTypeTargetIDs(r.Context(), row.key)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		var variables []string
		_ = parseJSONColumn(row.variablesJSON, &variables)

		types = append(types, map[string]any{
			"key":       row.key,
			"label":     row.label,
			"category":  row.category,
			"enabled":   row.enabled,
			"variables": variables,
			"template":  effective,
			"targetIds": targetIDs,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"messageTypes": types})
}

func (h *Handler) loadEffectiveTemplate(ctx context.Context, key, defaultJSON string) (template.Template, error) {
	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		return template.Template{}, err
	}
	var overrideJSON sql.NullString
	err := h.db.QueryRowContext(ctx,
		`SELECT template_json FROM message_templates WHERE message_type_key = ?`, key,
	).Scan(&overrideJSON)
	if err != nil && err != sql.ErrNoRows {
		return template.Template{}, err
	}
	if overrideJSON.Valid {
		return template.Merge(defaults, []byte(overrideJSON.String))
	}
	return defaults, nil
}

func (h *Handler) loadMessageTypeTargetIDs(ctx context.Context, key string) ([]int64, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT target_id FROM message_type_targets WHERE message_type_key = ? ORDER BY target_id`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (h *Handler) UpdateMessageTypeEnabled(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req enabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE message_types SET enabled = ? WHERE key = ?`, req.Enabled, key)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}

func (h *Handler) UpdateMessageTypeTemplate(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var partial template.Template
	if err := json.NewDecoder(r.Body).Decode(&partial); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	var defaultJSON string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT default_template_json FROM message_types WHERE key = ?`, key,
	).Scan(&defaultJSON)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	existingOverride, _ := h.loadOverrideJSON(r.Context(), key)
	mergedOverride, err := mergeTemplateOverride(existingOverride, partial)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid template override")
		return
	}

	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	effective, err := template.Merge(defaults, mergedOverride)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if err := template.ValidateMessageType(key, effective); err != nil {
		validationError(w, r, err)
		return
	}

	user, _ := auth.UserFromContext(r.Context())
	if err := h.saveTemplateOverride(r.Context(), key, mergedOverride, user.ID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, effective)
}

func (h *Handler) loadOverrideJSON(ctx context.Context, key string) ([]byte, error) {
	var overrideJSON sql.NullString
	err := h.db.QueryRowContext(ctx,
		`SELECT template_json FROM message_templates WHERE message_type_key = ?`, key,
	).Scan(&overrideJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !overrideJSON.Valid {
		return nil, nil
	}
	return []byte(overrideJSON.String), nil
}

func mergeTemplateOverride(existing []byte, partial template.Template) ([]byte, error) {
	raw := make(map[string]json.RawMessage)
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &raw); err != nil {
			return nil, err
		}
	}
	if partial.Plain != "" {
		b, err := json.Marshal(partial.Plain)
		if err != nil {
			return nil, err
		}
		raw["plain"] = b
	}
	if partial.Embed != nil {
		b, err := json.Marshal(partial.Embed)
		if err != nil {
			return nil, err
		}
		raw["embed"] = b
	}
	if len(raw) == 0 {
		return nil, nil
	}
	return json.Marshal(raw)
}

func (h *Handler) saveTemplateOverride(ctx context.Context, key string, overrideJSON []byte, userID int64) error {
	now := timeNowRFC3339()
	if len(overrideJSON) == 0 {
		_, err := h.db.ExecContext(ctx,
			`DELETE FROM message_templates WHERE message_type_key = ?`, key)
		return err
	}
	_, err := h.db.ExecContext(ctx, `
		INSERT INTO message_templates (message_type_key, template_json, updated_by, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(message_type_key) DO UPDATE SET
			template_json = excluded.template_json,
			updated_by = excluded.updated_by,
			updated_at = excluded.updated_at`,
		key, string(overrideJSON), userID, now,
	)
	return err
}

func (h *Handler) ResetMessageTypeTemplate(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	variant := strings.TrimSpace(r.URL.Query().Get("variant"))
	if variant == "" {
		writeError(w, r, http.StatusBadRequest, "variant is required")
		return
	}
	if variant != "plain" && variant != "embed" && variant != "all" {
		writeError(w, r, http.StatusBadRequest, "invalid variant")
		return
	}

	var defaultJSON string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT default_template_json FROM message_types WHERE key = ?`, key,
	).Scan(&defaultJSON)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	existing, _ := h.loadOverrideJSON(r.Context(), key)
	raw := make(map[string]json.RawMessage)
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &raw)
	}
	switch variant {
	case "plain":
		delete(raw, "plain")
	case "embed":
		delete(raw, "embed")
	case "all":
		raw = nil
	}

	var newOverride []byte
	if len(raw) > 0 {
		newOverride, err = json.Marshal(raw)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
	}

	user, _ := auth.UserFromContext(r.Context())
	if err := h.saveTemplateOverride(r.Context(), key, newOverride, user.ID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	effective, err := h.loadEffectiveTemplate(r.Context(), key, defaultJSON)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, effective)
}

func (h *Handler) PreviewMessageTypeTemplate(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req templatePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	var defaultJSON string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT default_template_json FROM message_types WHERE key = ?`, key,
	).Scan(&defaultJSON)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	effective := template.MergeTemplates(defaults, &req.Template)
	if err := template.ValidateMessageType(key, effective); err != nil {
		validationError(w, r, err)
		return
	}

	rendered := template.Render(effective, template.SampleVariables(key))
	writeJSON(w, http.StatusOK, renderedToAPI(rendered))
}

func (h *Handler) TestMessageTypeTemplate(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req templatePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	var defaultJSON string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT default_template_json FROM message_types WHERE key = ?`, key,
	).Scan(&defaultJSON)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	var defaults template.Template
	if err := json.Unmarshal([]byte(defaultJSON), &defaults); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	effective := template.MergeTemplates(defaults, &req.Template)
	if err := template.ValidateMessageType(key, effective); err != nil {
		validationError(w, r, err)
		return
	}

	rendered := template.Render(effective, template.SampleVariables(key))
	if err := h.dispatcher.SendRenderedTest(r.Context(), key, rendered); err != nil {
		if errors.Is(err, notify.ErrNoTargets) {
			writeError(w, r, http.StatusBadRequest, "no targets assigned")
			return
		}
		writeError(w, r, http.StatusBadRequest, notify.FormatDiscordSendError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateMessageTypeTargets(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req targetIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	var exists int
	err := h.db.QueryRowContext(r.Context(),
		`SELECT 1 FROM message_types WHERE key = ?`, key,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		writeError(w, r, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(),
		`DELETE FROM message_type_targets WHERE message_type_key = ?`, key); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	for _, targetID := range req.TargetIDs {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO message_type_targets (message_type_key, target_id) VALUES (?, ?)`,
			key, targetID); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid target id")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"targetIds": req.TargetIDs})
}

func (h *Handler) GetNotificationLog(w http.ResponseWriter, r *http.Request) {
	p := parsePagination(r)
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	targetFilter := strings.TrimSpace(r.URL.Query().Get("target"))
	deliveryFilter := strings.TrimSpace(r.URL.Query().Get("delivery"))

	where := "WHERE 1=1"
	args := make([]any, 0, 5)
	if typeFilter != "" {
		where += " AND nl.message_type_key = ?"
		args = append(args, typeFilter)
	}
	if targetFilter != "" {
		where += " AND nl.target_id = ?"
		args = append(args, targetFilter)
	}
	if deliveryFilter != "" {
		where += " AND nl.delivery_mode = ?"
		args = append(args, deliveryFilter)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM notification_log nl ` + where
	if err := h.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	query := `
		SELECT nl.id, nl.message_type_key, nl.target_id, nt.name,
			nl.rendered_preview, nl.success, nl.error, nl.sent_at,
			nl.delivery_mode, nl.recipient_external_user_id
		FROM notification_log nl
		LEFT JOIN notification_targets nt ON nt.id = nl.target_id
		` + where + `
		ORDER BY nl.sent_at DESC
		LIMIT ? OFFSET ?`
	listArgs := append(args, p.Limit, p.Offset)
	rows, err := h.db.QueryContext(r.Context(), query, listArgs...)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var messageTypeKey, preview, sentAt, deliveryMode string
		var targetID sql.NullInt64
		var targetName sql.NullString
		var recipientExternalUserID sql.NullString
		var success bool
		var errText sql.NullString
		if err := rows.Scan(
			&id, &messageTypeKey, &targetID, &targetName,
			&preview, &success, &errText, &sentAt,
			&deliveryMode, &recipientExternalUserID,
		); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal error")
			return
		}
		item := map[string]any{
			"id":              id,
			"messageTypeKey":  messageTypeKey,
			"renderedPreview": preview,
			"success":         success,
			"error":           nullStringPtr(errText),
			"sentAt":          sentAt,
			"deliveryMode":    deliveryMode,
		}
		if targetID.Valid {
			item["targetId"] = targetID.Int64
		} else {
			item["targetId"] = nil
		}
		item["targetName"] = nullStringPtr(targetName)
		item["recipientExternalUserId"] = nullStringPtr(recipientExternalUserID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	var s settingsResponse
	var authToken sql.NullString
	err := h.db.QueryRowContext(r.Context(), `
		SELECT server_name, frm_host, frm_port, frm_auth_token,
			poll_interval_seconds, production_snapshot_interval_seconds, production_snapshot_retention_days
		FROM app_settings WHERE id = 1`,
	).Scan(&s.ServerName, &s.FRMHost, &s.FRMPort, &authToken,
		&s.PollIntervalSeconds, &s.ProductionSnapshotIntervalSeconds, &s.ProductionSnapshotRetentionDays)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	if authToken.Valid {
		s.FRMAuthToken = authToken.String
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	current, err := h.getSettingsRow(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	if raw, ok := req["frmHost"]; ok {
		_ = json.Unmarshal(raw, &current.FRMHost)
	}
	if raw, ok := req["frmPort"]; ok {
		_ = json.Unmarshal(raw, &current.FRMPort)
	}
	if raw, ok := req["frmAuthToken"]; ok {
		_ = json.Unmarshal(raw, &current.FRMAuthToken)
	}
	if raw, ok := req["pollIntervalSeconds"]; ok {
		_ = json.Unmarshal(raw, &current.PollIntervalSeconds)
	}
	if raw, ok := req["productionSnapshotIntervalSeconds"]; ok {
		_ = json.Unmarshal(raw, &current.ProductionSnapshotIntervalSeconds)
	}
	if raw, ok := req["productionSnapshotRetentionDays"]; ok {
		_ = json.Unmarshal(raw, &current.ProductionSnapshotRetentionDays)
	}

	if current.PollIntervalSeconds <= 0 || current.ProductionSnapshotIntervalSeconds <= 0 || current.ProductionSnapshotRetentionDays <= 0 {
		writeError(w, r, http.StatusBadRequest, "interval and retention values must be positive")
		return
	}

	if strings.TrimSpace(current.FRMHost) != "" {
		sessionName, err := fetchFRMSessionName(r.Context(), current.FRMHost, current.FRMPort, current.FRMAuthToken)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "could not reach FRM server or resolve session name")
			return
		}
		current.ServerName = sessionName
	}

	_, err = h.db.ExecContext(r.Context(), `
		UPDATE app_settings SET
			server_name = ?, frm_host = ?, frm_port = ?, frm_auth_token = ?,
			poll_interval_seconds = ?, production_snapshot_interval_seconds = ?,
			production_snapshot_retention_days = ?
		WHERE id = 1`,
		current.ServerName, current.FRMHost, current.FRMPort, nullIfEmpty(current.FRMAuthToken),
		current.PollIntervalSeconds, current.ProductionSnapshotIntervalSeconds,
		current.ProductionSnapshotRetentionDays,
	)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, current)
}

func (h *Handler) getSettingsRow(ctx context.Context) (settingsResponse, error) {
	var s settingsResponse
	var authToken sql.NullString
	err := h.db.QueryRowContext(ctx, `
		SELECT server_name, frm_host, frm_port, frm_auth_token,
			poll_interval_seconds, production_snapshot_interval_seconds, production_snapshot_retention_days
		FROM app_settings WHERE id = 1`,
	).Scan(&s.ServerName, &s.FRMHost, &s.FRMPort, &authToken,
		&s.PollIntervalSeconds, &s.ProductionSnapshotIntervalSeconds, &s.ProductionSnapshotRetentionDays)
	if err != nil {
		return s, err
	}
	if authToken.Valid {
		s.FRMAuthToken = authToken.String
	}
	return s, nil
}

func (h *Handler) TestFRMConnection(w http.ResponseWriter, r *http.Request) {
	var req frmTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.FRMHost) == "" || req.FRMPort <= 0 {
		writeError(w, r, http.StatusBadRequest, "frmHost and frmPort are required")
		return
	}

	sessionName, err := fetchFRMSessionName(r.Context(), req.FRMHost, req.FRMPort, req.FRMAuthToken)
	if err != nil {
		writeError(w, r, http.StatusBadGateway, "could not reach FRM server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionName": sessionName,
		"reachable":   true,
	})
}

func fetchFRMSessionName(ctx context.Context, host string, port int, token string) (string, error) {
	client := frm.NewClient(frm.Config{
		Host:  strings.TrimSpace(host),
		Port:  port,
		Token: strings.TrimSpace(token),
	})
	info, err := client.GetSessionInfo(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(info.SessionName) == "" {
		return "", fmt.Errorf("empty session name")
	}
	return info.SessionName, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.auth.ListUsers(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != nil {
		if *req.Role != auth.RoleAdmin && *req.Role != auth.RoleViewer {
			writeError(w, r, http.StatusBadRequest, "invalid role")
			return
		}
	}

	playerIDUpdate, err := auth.ParseOptionalPlayerID(req.PlayerIDJSON)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid playerId")
		return
	}

	user, err := h.auth.UpdateUser(r.Context(), id, auth.UserUpdate{
		Role:     req.Role,
		Password: req.Password,
		PlayerID: playerIDUpdate,
	})
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, auth.ErrLastAdmin) {
			writeError(w, r, http.StatusBadRequest, "cannot demote the last admin")
			return
		}
		if errors.Is(err, auth.ErrWeakPassword) {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(err.Error(), "player not found") {
			writeError(w, r, http.StatusBadRequest, "player not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.auth.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, r, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, auth.ErrLastAdmin) {
			writeError(w, r, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
