package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.APIBase != "http://127.0.0.1:8400/v1" {
		t.Errorf("APIBase = %q, want default", cfg.APIBase)
	}
	if cfg.Model != "glm-5.2" {
		t.Errorf("Model = %q, want default", cfg.Model)
	}
	if cfg.SynthModel != "glm-5.2" {
		t.Errorf("SynthModel = %q, want default", cfg.SynthModel)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", cfg.Timeout)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey should be empty by default")
	}
}

func TestLoad_InvalidBase(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_API_BASE", "not-a-url")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid STORM_API_BASE")
	}
}

func TestLoad_InvalidTimeout(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_TIMEOUT_SEC", "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-integer STORM_TIMEOUT_SEC")
	}
}

func TestLoad_ZeroTimeout(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_TIMEOUT_SEC", "0")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero STORM_TIMEOUT_SEC")
	}
}

func TestString_RedactsKey(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_API_KEY", "super-secret-key")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	s := cfg.String()
	if strings.Contains(s, "super-secret-key") {
		t.Error("String() leaked API key")
	}
	if !strings.Contains(s, "[REDACTED]") {
		t.Error("String() should show [REDACTED] when key is set")
	}
}

func TestLoad_BaseWithoutScheme(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_API_BASE", "localhost:8400/v1")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for STORM_API_BASE without scheme")
	}
}

func TestString_RedactsUserinfoInBase(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_API_BASE", "http://u:s3cr3tpass@somehost/v1")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	s := cfg.String()
	if strings.Contains(s, "s3cr3tpass") {
		t.Errorf("String() leaked password in base URL: %s", s)
	}
	if strings.Contains(s, "u:") {
		t.Errorf("String() leaked userinfo in base URL: %s", s)
	}
}

func TestLoad_TimeoutTooLarge(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORM_TIMEOUT_SEC", "999999")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for STORM_TIMEOUT_SEC > 3600")
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"STORM_API_BASE", "STORM_API_KEY", "STORM_MODEL",
		"STORM_SYNTH_MODEL", "STORM_TIMEOUT_SEC",
	} {
		old := os.Getenv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if old != "" {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
	}
}
