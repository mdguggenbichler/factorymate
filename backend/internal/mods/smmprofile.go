package mods

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"factorymate/internal/frm"
)

const ficsitAPIBase = "https://api.ficsit.app"

var profileRequiredTargets = []string{"Windows", "LinuxServer"}

// FicsitResolver resolves mod version metadata from ficsit.app.
type FicsitResolver interface {
	ResolveModVersions(ctx context.Context, constraints []ModVersionConstraint) ([]ResolvedMod, error)
}

// ModVersionConstraint pins a mod reference to an exact version.
type ModVersionConstraint struct {
	ModIDOrReference string `json:"modIdOrReference"`
	Version          string `json:"version"`
}

// ResolvedMod is a ficsit.app resolveModVersions result entry.
type ResolvedMod struct {
	ModReference string
	Versions     []ResolvedVersion
}

// ResolvedVersion holds per-target download metadata.
type ResolvedVersion struct {
	ID      string
	Version string
	Targets []ResolvedTarget
}

// ResolvedTarget is a platform-specific download pin.
type ResolvedTarget struct {
	TargetName string
	Hash       string
	Link       string
}

// SMMProfile is the generated .smmprofile document (§8.5).
type SMMProfile struct {
	Profile  SMMProfileSection  `json:"profile"`
	Lockfile SMMProfileLockfile `json:"lockfile"`
	Metadata SMMProfileMetadata `json:"metadata"`
}

type SMMProfileSection struct {
	Name            string                       `json:"name"`
	RequiredTargets []string                     `json:"required_targets"`
	Mods            map[string]SMMProfileModSpec `json:"mods"`
}

type SMMProfileModSpec struct {
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

type SMMProfileLockfile struct {
	Version int                              `json:"version"`
	Mods    map[string]SMMProfileLockMod     `json:"mods"`
}

type SMMProfileLockMod struct {
	Version      string                            `json:"version"`
	Dependencies any                               `json:"dependencies"`
	Targets      map[string]SMMProfileLockTarget   `json:"targets"`
}

type SMMProfileLockTarget struct {
	Hash string `json:"hash"`
	Link string `json:"link"`
}

type SMMProfileMetadata struct {
	GameVersion int `json:"gameVersion"`
}

// GenerateSMMProfile builds a .smmprofile from live mod list + ficsit.app lockfile resolution.
func (s *Service) GenerateSMMProfile(ctx context.Context) ([]byte, string, error) {
	s.mu.Lock()
	if s.cache != nil && len(s.cache.smmProfile) > 0 && s.Now().Sub(s.cache.at) < s.CacheTTL {
		data := append([]byte(nil), s.cache.smmProfile...)
		filename := s.cache.smmFilename
		s.mu.Unlock()
		return data, filename, nil
	}
	s.mu.Unlock()

	rawMods, capturedGen, err := s.rawModsWithGeneration(ctx)
	if err != nil {
		return nil, "", err
	}
	profileName, err := s.ProfileName(ctx)
	if err != nil {
		return nil, "", err
	}

	profile, err := buildSMMProfile(ctx, s.FicsitClient, rawMods, profileName)
	if err != nil {
		return nil, "", err
	}

	raw, err := json.Marshal(profile)
	if err != nil {
		return nil, "", fmt.Errorf("marshal smm profile: %w", err)
	}
	filename := "factorymate-server.smmprofile"

	s.mu.Lock()
	if s.cache != nil && s.cache.generation == capturedGen {
		s.cache.smmProfile = append([]byte(nil), raw...)
		s.cache.smmFilename = filename
	}
	s.mu.Unlock()

	return raw, filename, nil
}

func (s *Service) rawModsWithGeneration(ctx context.Context) ([]frm.Mod, uint64, error) {
	_, err := s.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		return nil, 0, fmt.Errorf("mod list not available")
	}
	return append([]frm.Mod(nil), s.cache.rawMods...), s.cache.generation, nil
}

func buildSMMProfile(ctx context.Context, client FicsitResolver, rawMods []frm.Mod, profileName string) (SMMProfile, error) {
	var gameVersion int
	var hasFactoryGame bool
	constraints := make([]ModVersionConstraint, 0)
	for _, m := range rawMods {
		if m.SMRName == "FactoryGame" {
			var err error
			gameVersion, err = parseGameBuildInt(m.Version)
			if err != nil {
				return SMMProfile{}, err
			}
			hasFactoryGame = true
			continue
		}
		constraints = append(constraints, ModVersionConstraint{
			ModIDOrReference: m.SMRName,
			Version:          m.Version,
		})
	}
	if !hasFactoryGame {
		return SMMProfile{}, fmt.Errorf("FactoryGame mod not found in mod list")
	}

	resolved, err := client.ResolveModVersions(ctx, constraints)
	if err != nil {
		return SMMProfile{}, err
	}
	resolvedByRef := make(map[string]ResolvedMod, len(resolved))
	for _, r := range resolved {
		resolvedByRef[r.ModReference] = r
	}

	profileMods := make(map[string]SMMProfileModSpec)
	lockMods := make(map[string]SMMProfileLockMod)

	for _, m := range rawMods {
		if m.SMRName == "FactoryGame" {
			continue
		}
		res, ok := resolvedByRef[m.SMRName]
		if !ok || len(res.Versions) == 0 {
			return SMMProfile{}, fmt.Errorf("could not resolve mod %s version %s on ficsit.app", m.SMRName, m.Version)
		}
		ver := res.Versions[0]
		targets := make(map[string]SMMProfileLockTarget)
		for _, t := range ver.Targets {
			link := t.Link
			if strings.HasPrefix(link, "/") {
				link = ficsitAPIBase + link
			}
			targets[t.TargetName] = SMMProfileLockTarget{Hash: t.Hash, Link: link}
		}
		for _, required := range profileRequiredTargets {
			if _, ok := targets[required]; !ok {
				return SMMProfile{}, fmt.Errorf(
					"mod %s version %s has no %s target",
					m.SMRName, m.Version, required,
				)
			}
		}
		lockMods[m.SMRName] = SMMProfileLockMod{
			Version:      m.Version,
			Dependencies: nil,
			Targets:      targets,
		}
		if m.SMRName != "SML" {
			profileMods[m.SMRName] = SMMProfileModSpec{Version: ">=0.0.0", Enabled: true}
		}
	}

	return SMMProfile{
		Profile: SMMProfileSection{
			Name:            profileName,
			RequiredTargets: append([]string(nil), profileRequiredTargets...),
			Mods:            profileMods,
		},
		Lockfile: SMMProfileLockfile{
			Version: 1,
			Mods:    lockMods,
		},
		Metadata: SMMProfileMetadata{GameVersion: gameVersion},
	}, nil
}

func parseGameBuildInt(version string) (int, error) {
	part := strings.SplitN(version, ".", 2)[0]
	if part == "" {
		return 0, fmt.Errorf("invalid FactoryGame version %q", version)
	}
	n := 0
	for _, c := range part {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid FactoryGame version %q", version)
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return 0, fmt.Errorf("invalid FactoryGame version %q", version)
	}
	return n, nil
}

// HTTPFicsitClient calls the public ficsit.app GraphQL API.
type HTTPFicsitClient struct {
	Endpoint   string
	HTTPClient HTTPDoer
}

// HTTPDoer performs HTTP requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type httpClientWithTimeout struct {
	timeout time.Duration
}

func (c *httpClientWithTimeout) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{Timeout: c.timeout}
	return client.Do(req)
}

const resolveQuery = `query ResolveServerMods($filter: [ModVersionConstraint!]!) {
  resolveModVersions(filter: $filter) {
    mod_reference
    versions {
      id
      version
      targets { targetName hash link }
    }
  }
}`

func (c *HTTPFicsitClient) ResolveModVersions(ctx context.Context, constraints []ModVersionConstraint) ([]ResolvedMod, error) {
	if len(constraints) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{
		"query":     resolveQuery,
		"variables": map[string]any{"filter": constraints},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal graphql: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ficsit.app request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read ficsit.app response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ficsit.app HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Data struct {
			ResolveModVersions []struct {
				ModReference string `json:"mod_reference"`
				Versions     []struct {
					ID      string `json:"id"`
					Version string `json:"version"`
					Targets []struct {
						TargetName string `json:"targetName"`
						Hash       string `json:"hash"`
						Link       string `json:"link"`
					} `json:"targets"`
				} `json:"versions"`
			} `json:"resolveModVersions"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse ficsit.app response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("ficsit.app graphql: %s", parsed.Errors[0].Message)
	}

	out := make([]ResolvedMod, 0, len(parsed.Data.ResolveModVersions))
	for _, item := range parsed.Data.ResolveModVersions {
		rm := ResolvedMod{ModReference: item.ModReference}
		for _, v := range item.Versions {
			rv := ResolvedVersion{ID: v.ID, Version: v.Version}
			for _, t := range v.Targets {
				rv.Targets = append(rv.Targets, ResolvedTarget{
					TargetName: t.TargetName,
					Hash:       t.Hash,
					Link:       t.Link,
				})
			}
			rm.Versions = append(rm.Versions, rv)
		}
		out = append(out, rm)
	}
	return out, nil
}
