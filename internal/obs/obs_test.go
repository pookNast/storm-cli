package obs

import (
	"testing"
)

func TestTopicHash_Stable(t *testing.T) {
	h1 := TopicHash("AI safety")
	h2 := TopicHash("AI safety")
	if h1 != h2 {
		t.Error("TopicHash should be deterministic")
	}
	if len(h1) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("TopicHash length = %d, want 16", len(h1))
	}
}

func TestTopicHash_Different(t *testing.T) {
	h1 := TopicHash("topic A")
	h2 := TopicHash("topic B")
	if h1 == h2 {
		t.Error("different topics should have different hashes")
	}
}

func TestRedactKey_NonEmpty(t *testing.T) {
	got := RedactKey("secret123")
	if got != "[REDACTED]" {
		t.Errorf("RedactKey(%q) = %q, want [REDACTED]", "secret123", got)
	}
}

func TestRedactKey_Empty(t *testing.T) {
	got := RedactKey("")
	if got != "[empty]" {
		t.Errorf("RedactKey empty = %q, want [empty]", got)
	}
}

func TestSetup_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Setup panicked: %v", r)
		}
	}()
	Setup(false)
	Setup(true)
}
