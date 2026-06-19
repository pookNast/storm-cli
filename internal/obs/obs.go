// Package obs centralizes observability: slog JSON handler to stderr, secret redaction.
package obs

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
)

// Setup initialises the global slog handler.
// verbose=true sets DEBUG level; false sets INFO level.
// All output goes to STDERR (stdout is reserved for piping).
func Setup(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

// TopicHash returns a short hex digest of the topic string safe for INFO-level logging.
// The raw topic is never logged at INFO.
func TopicHash(topic string) string {
	sum := sha256.Sum256([]byte(topic))
	return fmt.Sprintf("%x", sum[:8])
}

// RedactKey returns "[REDACTED]" if key is non-empty, otherwise "[empty]".
// Use this whenever displaying key status in logs or diagnostics.
func RedactKey(key string) string {
	if key != "" {
		return "[REDACTED]"
	}
	return "[empty]"
}
