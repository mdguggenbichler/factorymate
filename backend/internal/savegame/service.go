package savegame

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Service coordinates save downloads with config, rate limits, and audit logging.
type Service struct {
	DB *sql.DB
}

// NewService constructs a savegame service.
func NewService(db *sql.DB) *Service {
	return &Service{DB: db}
}

// ConfigFromDB loads game API settings from app_settings.
func (s *Service) ConfigFromDB(ctx context.Context) (Config, error) {
	var host string
	var port int
	var token sql.NullString
	err := s.DB.QueryRowContext(ctx, `
		SELECT game_api_host, game_api_port, game_api_token
		FROM app_settings WHERE id = 1`,
	).Scan(&host, &port, &token)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Host: strings.TrimSpace(host),
		Port: port,
	}
	if token.Valid {
		cfg.Token = strings.TrimSpace(token.String)
	}
	return cfg, nil
}

// IsConfigured reports whether host and token are set for downloads.
func (s *Service) IsConfigured(ctx context.Context) (bool, error) {
	cfg, err := s.ConfigFromDB(ctx)
	if err != nil {
		return false, err
	}
	return cfg.Host != "" && cfg.Token != "", nil
}

// TestConnection probes the API and resolves the latest autosave name.
func (s *Service) TestConnection(ctx context.Context, cfg Config) (LatestSaveInfo, error) {
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port <= 0 || strings.TrimSpace(cfg.Token) == "" {
		return LatestSaveInfo{}, ErrNotConfigured
	}
	client := NewClient(cfg)
	return client.ResolveLatestSave(ctx)
}

// ResolveLatest uses stored configuration to resolve latest save metadata.
func (s *Service) ResolveLatest(ctx context.Context) (LatestSaveInfo, error) {
	cfg, err := s.loadReadyConfig(ctx)
	if err != nil {
		return LatestSaveInfo{}, err
	}
	return NewClient(cfg).ResolveLatestSave(ctx)
}

// DownloadLatest downloads the latest autosave for the active session.
func (s *Service) DownloadLatest(ctx context.Context, userID int64, channel string) (DownloadResult, error) {
	cfg, err := s.loadReadyConfig(ctx)
	if err != nil {
		return DownloadResult{}, err
	}
	return s.downloadLatestWithClient(ctx, userID, channel, NewClient(cfg))
}

func (s *Service) downloadLatestWithClient(ctx context.Context, userID int64, channel string, client *Client) (DownloadResult, error) {
	if err := s.checkRateLimit(ctx, userID); err != nil {
		return DownloadResult{}, err
	}

	info, err := client.ResolveLatestSave(ctx)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	result, err := client.DownloadSaveGame(ctx, info.SaveName)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	if err := s.recordDownload(ctx, userID, channel, info.SaveName, result.Size); err != nil {
		return DownloadResult{}, err
	}
	return result, nil
}

func (s *Service) loadReadyConfig(ctx context.Context) (Config, error) {
	cfg, err := s.ConfigFromDB(ctx)
	if err != nil {
		return Config{}, err
	}
	if cfg.Host == "" || cfg.Token == "" {
		return Config{}, ErrNotConfigured
	}
	if cfg.Port <= 0 {
		cfg.Port = 7777
	}
	return cfg, nil
}

func (s *Service) checkRateLimit(ctx context.Context, userID int64) error {
	cutoff := time.Now().UTC().Add(-rateLimitWindow * time.Minute).Format(time.RFC3339)
	var count int
	err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM savegame_download_log
		WHERE user_id = ? AND downloaded_at > ?`,
		userID, cutoff,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrRateLimited
	}
	return nil
}

func (s *Service) recordDownload(ctx context.Context, userID int64, channel, saveName string, bytes int64) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO savegame_download_log (user_id, channel, save_name, bytes, downloaded_at)
		VALUES (?, ?, ?, ?, ?)`,
		userID, channel, saveName, bytes, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
