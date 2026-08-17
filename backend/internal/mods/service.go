package mods

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"factorymate/internal/frm"
)

const (
	defaultCacheTTL   = 15 * time.Minute
	settingProfileName = "mods.smm_profile_name"
	defaultProfileName = "FactoryMate Server"
)

// FRMClientFactory builds an FRM client from current settings.
type FRMClientFactory func(ctx context.Context) (*frm.Client, error)

// Service caches FRM getModList results (§8.5).
type Service struct {
	DB           *sql.DB
	FRMClient    FRMClientFactory
	Now          func() time.Time
	CacheTTL     time.Duration
	FicsitClient FicsitResolver

	mu    sync.Mutex
	cache *cachedList
}

type cachedList struct {
	response    ListResponse
	rawMods     []frm.Mod
	smmProfile  []byte
	smmFilename string
	at          time.Time
}

// NewService constructs a mod list service.
func NewService(db *sql.DB, frmFactory FRMClientFactory) *Service {
	return &Service{
		DB:        db,
		FRMClient: frmFactory,
		Now:       time.Now,
		CacheTTL:  defaultCacheTTL,
		FicsitClient: &HTTPFicsitClient{
			Endpoint: "https://api.ficsit.app/v2/query",
			HTTPClient: &httpClientWithTimeout{timeout: 15 * time.Second},
		},
	}
}

// List returns cached mod list, refreshing when stale.
func (s *Service) List(ctx context.Context) (ListResponse, error) {
	s.mu.Lock()
	if s.cache != nil && s.Now().Sub(s.cache.at) < s.CacheTTL {
		resp := s.cache.response
		s.mu.Unlock()
		return resp, nil
	}
	s.mu.Unlock()

	return s.Refresh(ctx)
}

// Refresh fetches mod list from FRM and busts SMM profile cache.
func (s *Service) Refresh(ctx context.Context) (ListResponse, error) {
	client, err := s.FRMClient(ctx)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.cache != nil {
			resp := s.cache.response
			resp.FRMReachable = false
			resp.Mods = []Mod{}
			return resp, nil
		}
		return ListResponse{Mods: []Mod{}, FRMReachable: false}, nil
	}

	rawMods, err := client.GetModList(ctx)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.cache != nil {
			resp := s.cache.response
			resp.FRMReachable = false
			resp.Mods = []Mod{}
			return resp, nil
		}
		return ListResponse{Mods: []Mod{}, FRMReachable: false}, nil
	}

	resp := buildListResponse(rawMods, s.Now)
	s.mu.Lock()
	s.cache = &cachedList{
		response: resp,
		rawMods:  rawMods,
		at:       s.Now(),
	}
	s.mu.Unlock()
	return resp, nil
}

// RawMods returns the cached FRM mod rows for SMM export.
func (s *Service) RawMods(ctx context.Context) ([]frm.Mod, error) {
	_, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		return nil, fmt.Errorf("mod list not available")
	}
	return append([]frm.Mod(nil), s.cache.rawMods...), nil
}

// ProfileName returns the configured SMM profile name.
func (s *Service) ProfileName(ctx context.Context) (string, error) {
	var name string
	err := s.DB.QueryRowContext(ctx, `
		SELECT value FROM app_setting_kv WHERE key = ?`, settingProfileName,
	).Scan(&name)
	if err == sql.ErrNoRows || strings.TrimSpace(name) == "" {
		return defaultProfileName, nil
	}
	if err != nil {
		return defaultProfileName, fmt.Errorf("get profile name: %w", err)
	}
	return strings.TrimSpace(name), nil
}

func buildListResponse(raw []frm.Mod, now func() time.Time) ListResponse {
	mods := make([]Mod, 0, len(raw))
	var gameBuild, smlVersion string
	for _, m := range raw {
		mods = append(mods, fromFRMMod(m))
		switch m.SMRName {
		case "FactoryGame":
			gameBuild = m.Version
		case "SML":
			smlVersion = m.Version
		}
	}
	return ListResponse{
		GameBuild:    gameBuild,
		SMLVersion:   smlVersion,
		Mods:         mods,
		CachedAt:     now().UTC().Format(time.RFC3339),
		FRMReachable:  true,
	}
}

func fromFRMMod(m frm.Mod) Mod {
	return Mod{
		Name:               m.Name,
		SMRName:            m.SMRName,
		Version:            m.Version,
		Description:        m.Description,
		DocsURL:            m.DocsURL,
		SupportURL:         m.SupportURL,
		CreatedBy:          m.CreatedBy,
		RemoteVersionRange: m.RemoteVersionRange,
		RequiredOnRemote:   m.RequiredOnRemote,
	}
}
