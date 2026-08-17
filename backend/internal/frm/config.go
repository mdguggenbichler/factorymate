package frm

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds FRM connection settings (typically from app_settings).
type Config struct {
	Host  string
	Port  int
	Token string // optional; sent as X-FRM-Authorization when non-empty
}

// BaseURL returns the FRM HTTP base URL without a trailing slash.
func (c Config) BaseURL() string {
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

// ConfigFromEnv reads FRM_TEST_HOST, FRM_TEST_PORT, and FRM_TEST_TOKEN for integration tests.
func ConfigFromEnv() Config {
	host := os.Getenv("FRM_TEST_HOST")
	port := 8080
	if p := os.Getenv("FRM_TEST_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	return Config{
		Host:  host,
		Port:  port,
		Token: os.Getenv("FRM_TEST_TOKEN"),
	}
}
