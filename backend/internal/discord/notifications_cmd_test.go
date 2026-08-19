package discord

import "testing"

func TestCategoryStateLabel(t *testing.T) {
	if got := categoryStateLabel(0, 2); got != "off" {
		t.Fatalf("got %q", got)
	}
	if got := categoryStateLabel(2, 2); got != "on" {
		t.Fatalf("got %q", got)
	}
	if got := categoryStateLabel(1, 2); got != "mixed (1/2)" {
		t.Fatalf("got %q", got)
	}
}

func TestDashboardPrefsFooter(t *testing.T) {
	if got := dashboardPrefsFooter(""); got != "Dashboard URL is not configured (set FACTORYMATE_PUBLIC_URL)." {
		t.Fatalf("empty = %q", got)
	}
	if got := dashboardPrefsFooter("https://fm.example/account/notifications"); got != "Per-type checkboxes: https://fm.example/account/notifications" {
		t.Fatalf("url = %q", got)
	}
}

func TestNotificationsDashboardURL(t *testing.T) {
	t.Setenv("FACTORYMATE_PUBLIC_URL", "https://fm.example/")
	if got := NotificationsDashboardURL(); got != "https://fm.example/account/notifications" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("FACTORYMATE_PUBLIC_URL", "")
	if got := NotificationsDashboardURL(); got != "" {
		t.Fatalf("empty env = %q", got)
	}
}
