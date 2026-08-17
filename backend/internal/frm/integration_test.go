//go:build integration

package frm

import (
	"context"
	"os"
	"strconv"
	"testing"
)

func TestLiveFRM_allEndpoints(t *testing.T) {
	cfg := ConfigFromEnv()
	if cfg.Host == "" {
		t.Skip("FRM_TEST_HOST not set")
	}
	if p := os.Getenv("FRM_TEST_PORT"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			cfg.Port = port
		}
	}

	client := NewClient(cfg)
	ctx := context.Background()

	fast := client.GetFast(ctx)
	for ep, err := range fast.Errors {
		t.Errorf("fast poll %s: %v", ep, err)
	}
	if !fast.Reachable() {
		t.Fatal("fast poll unreachable")
	}

	slow := client.GetSlow(ctx)
	for ep, err := range slow.Errors {
		t.Errorf("slow poll %s: %v", ep, err)
	}
	if len(slow.Errors) > 0 {
		t.Fatalf("slow poll had %d errors", len(slow.Errors))
	}
}
