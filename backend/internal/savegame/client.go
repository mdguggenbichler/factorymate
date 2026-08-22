package savegame

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	metaTimeout     = 15 * time.Second
	downloadTimeout = 120 * time.Second
)

// Config holds Dedicated Server HTTPS API connection settings.
type Config struct {
	Host    string
	Port    int
	Token   string
	BaseURL string // optional override for tests (e.g. httptest server)
}

// Client calls the Satisfactory Dedicated Server HTTPS API (read-only).
type Client struct {
	cfg        Config
	metaHTTP   *http.Client
	downloadHTTP *http.Client
}

// NewClient constructs a client for the game API.
func NewClient(cfg Config) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed dedicated server cert
	}
	return &Client{
		cfg: cfg,
		metaHTTP: &http.Client{
			Timeout:   metaTimeout,
			Transport: transport,
		},
		downloadHTTP: &http.Client{
			Timeout:   downloadTimeout,
			Transport: transport,
		},
	}
}

type apiEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type apiErrorResponse struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (c *Client) baseURL() string {
	if strings.TrimSpace(c.cfg.BaseURL) != "" {
		return strings.TrimRight(c.cfg.BaseURL, "/")
	}
	return fmt.Sprintf("https://%s:%d/api/v1", strings.TrimSpace(c.cfg.Host), c.cfg.Port)
}

func (c *Client) postJSON(ctx context.Context, fn string, data any, httpClient *http.Client) ([]byte, string, error) {
	body := map[string]any{"function": fn}
	if data != nil {
		body["data"] = data
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL(), bytes.NewReader(raw))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(c.cfg.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") || (len(respBody) > 0 && respBody[0] == '{') {
		var errResp apiErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.ErrorCode != "" {
			return nil, "", fmt.Errorf("%s: %s", errResp.ErrorCode, errResp.ErrorMessage)
		}
		if resp.StatusCode >= 400 {
			return nil, "", fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
		}
	}

	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("http %d", resp.StatusCode)
	}

	return respBody, resp.Header.Get("Content-Disposition"), nil
}

// QueryServerState returns the current dedicated server state.
func (c *Client) QueryServerState(ctx context.Context) (ServerGameState, error) {
	raw, _, err := c.postJSON(ctx, "QueryServerState", nil, c.metaHTTP)
	if err != nil {
		return ServerGameState{}, err
	}
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ServerGameState{}, err
	}
	var data queryServerStateData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return ServerGameState{}, err
	}
	return data.ServerGameState, nil
}

// EnumerateSessions lists save sessions on the server.
func (c *Client) EnumerateSessions(ctx context.Context) ([]Session, error) {
	raw, _, err := c.postJSON(ctx, "EnumerateSessions", nil, c.metaHTTP)
	if err != nil {
		return nil, err
	}
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	var data enumerateSessionsData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	return data.Sessions, nil
}

// ResolveLatestSave finds the latest autosave for the active session without downloading.
func (c *Client) ResolveLatestSave(ctx context.Context) (LatestSaveInfo, error) {
	state, err := c.QueryServerState(ctx)
	if err != nil {
		return LatestSaveInfo{}, err
	}
	sessions, err := c.EnumerateSessions(ctx)
	if err != nil {
		return LatestSaveInfo{}, err
	}
	header, err := PickLatestAutosave(state.ActiveSessionName, sessions)
	if err != nil {
		return LatestSaveInfo{}, err
	}
	return LatestSaveInfo{
		ActiveSessionName: state.ActiveSessionName,
		SaveName:          header.SaveName,
		SaveDateTime:      header.SaveDateTime,
	}, nil
}

// DownloadSaveGame downloads a save file by name.
func (c *Client) DownloadSaveGame(ctx context.Context, saveName string) (DownloadResult, error) {
	raw, disposition, err := c.postJSON(ctx, "DownloadSaveGame", map[string]string{
		"SaveName": saveName,
	}, c.downloadHTTP)
	if err != nil {
		return DownloadResult{}, err
	}
	filename := parseContentDispositionFilename(disposition)
	if filename == "" {
		filename = saveName + ".sav"
	}
	return DownloadResult{
		Filename: filename,
		Size:     int64(len(raw)),
		Body:     raw,
	}, nil
}

func parseContentDispositionFilename(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	return params["filename"]
}
