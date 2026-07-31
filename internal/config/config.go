// Package config loads STORM environment configuration with validated defaults.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the STORM CLI.
// The API key is never printed by String() or any log call.
type Config struct {
	APIBase    string
	APIKey     string // never logged; use [REDACTED] in diagnostics
	Model      string
	SynthModel string
	Timeout    time.Duration
}

// Load reads configuration from environment variables, applying documented defaults.
// Returns an error if STORM_API_BASE is set but not a valid URL.
func Load() (Config, error) {
	base := envOr("STORM_API_BASE", "http://127.0.0.1:8400/v1")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("config: STORM_API_BASE must be http/https with a host, got %q", base)
	}

	// Default tuned for the local HiveMind gateway (glm-5.2) under concurrent load.
	timeoutSec := 120
	if raw := os.Getenv("STORM_TIMEOUT_SEC"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("config: invalid STORM_TIMEOUT_SEC %q: must be positive integer", raw)
		}
		if n > 3600 {
			return Config{}, fmt.Errorf("config: STORM_TIMEOUT_SEC too large (max 3600), got %q", raw)
		}
		timeoutSec = n
	}

	return Config{
		APIBase:    base,
		APIKey:     os.Getenv("STORM_API_KEY"),
		// glm-5.2 is the HiveMind lead + lab R10 floor. The former defaults
		// (glm-4.5-air, glm-5.1) are unsuitable for STORM's 5-way concurrent
		// persona fan-out: glm-4.5-air is a slower reasoning model that bursts
		// past the gateway rate limit / 120 s deadline, and glm-5.1 is silently
		// aliased to glm-5.2 on :8400 and trips tight timeouts.
		Model:      envOr("STORM_MODEL", "glm-5.2"),
		SynthModel: envOr("STORM_SYNTH_MODEL", "glm-5.2"),
		Timeout:    time.Duration(timeoutSec) * time.Second,
	}, nil
}

// String returns a safe diagnostic representation — key is always redacted,
// and any userinfo embedded in APIBase is stripped.
func (c Config) String() string {
	key := "[empty]"
	if c.APIKey != "" {
		key = "[REDACTED]"
	}
	base := c.APIBase
	if u, err := url.Parse(base); err == nil && u.User != nil {
		u.User = nil
		base = u.String()
	}
	return fmt.Sprintf("Config{base=%s model=%s synth=%s timeout=%s key=%s}",
		base, c.Model, c.SynthModel, c.Timeout, key)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
