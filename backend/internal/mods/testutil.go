package mods

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"

	"factorymate/internal/frm"
)

// MockFicsitResolver returns canned resolve results for tests.
type MockFicsitResolver struct {
	Mods []ResolvedMod
}

// MockFicsitFromFixture builds resolver data from an example .smmprofile fixture.
func MockFicsitFromFixture(fixture []byte) *MockFicsitResolver {
	var doc struct {
		Lockfile struct {
			Mods map[string]struct {
				Version string `json:"version"`
				Targets map[string]struct {
					Hash string `json:"hash"`
					Link string `json:"link"`
				} `json:"targets"`
			} `json:"mods"`
		} `json:"lockfile"`
	}
	if err := json.Unmarshal(fixture, &doc); err != nil {
		return &MockFicsitResolver{}
	}
	out := make([]ResolvedMod, 0, len(doc.Lockfile.Mods))
	for ref, mod := range doc.Lockfile.Mods {
		rv := ResolvedVersion{Version: mod.Version}
		for name, target := range mod.Targets {
			link := target.Link
			const prefix = "https://api.ficsit.app"
			if len(link) > len(prefix) && link[:len(prefix)] == prefix {
				link = link[len(prefix):]
			}
			rv.Targets = append(rv.Targets, ResolvedTarget{
				TargetName: name,
				Hash:       target.Hash,
				Link:       link,
			})
		}
		out = append(out, ResolvedMod{
			ModReference: ref,
			Versions:     []ResolvedVersion{rv},
		})
	}
	return &MockFicsitResolver{Mods: out}
}

func (m *MockFicsitResolver) ResolveModVersions(_ context.Context, constraints []ModVersionConstraint) ([]ResolvedMod, error) {
	if m == nil {
		return nil, fmt.Errorf("mock ficsit not configured")
	}
	byRef := make(map[string]ResolvedMod, len(m.Mods))
	for _, mod := range m.Mods {
		byRef[mod.ModReference] = mod
	}
	out := make([]ResolvedMod, 0, len(constraints))
	for _, c := range constraints {
		if mod, ok := byRef[c.ModIDOrReference]; ok {
			out = append(out, mod)
		}
	}
	return out, nil
}

// ModListTestServer hosts a mock getModList endpoint.
type ModListTestServer struct {
	Server *httptest.Server
	Host   string
	Port   int
}

// NewModListTestServer starts an httptest server returning mods JSON.
func NewModListTestServer(mods []frm.Mod) *ModListTestServer {
	body, _ := json.Marshal(mods)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getModList" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	host, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return &ModListTestServer{Server: srv, Host: host, Port: port}
}
