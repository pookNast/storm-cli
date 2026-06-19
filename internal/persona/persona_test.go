package persona

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pookNast/storm-cli/internal/client"
)

// repairMock returns prose on first-attempt calls and valid JSON on repair calls.
// It distinguishes them by checking whether the user prompt contains the repairSuffix,
// which avoids any race-condition flakiness from concurrent goroutine scheduling.
type repairMock struct {
	calls atomic.Int64
}

func (m *repairMock) Chat(_ context.Context, _, _, user string) (string, error) {
	m.calls.Add(1)
	if strings.Contains(user, "Your previous reply was not valid JSON") {
		// This is a repair call — return valid JSON.
		return `{"position":"repaired","evidence":"ev","insight":"ins"}`, nil
	}
	// First attempt — return prose to trigger repair.
	return "Here is my answer in plain prose, not JSON.", nil
}

func TestFanOut_RepairPath(t *testing.T) {
	mock := &repairMock{}
	results, err := FanOut(context.Background(), mock, "test-model", "AI safety", "analyst")
	if err != nil {
		t.Fatalf("FanOut repair path error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Position != "repaired" {
			t.Errorf("results[%d].Position = %q, want %q", i, r.Position, "repaired")
		}
	}
	// 5 goroutines × 2 calls each = 10 total
	if got := mock.calls.Load(); got != 10 {
		t.Errorf("total Chat calls = %d, want 10 (5 prose + 5 repair)", got)
	}
}

func personaJSON(name string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": `{"position":"pos-` + name + `","evidence":"ev","insight":"ins"}`}},
		},
	})
	return string(b)
}

func TestFanOut_Returns5Results(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(personaJSON("x"))) //nolint
	}))
	defer srv.Close()

	cl := client.New(srv.URL, "", 5*time.Second)
	results, err := FanOut(context.Background(), cl, "test-model", "AI safety", "analyst")
	if err != nil {
		t.Fatalf("FanOut error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	// Verify order matches locked5
	for i, p := range locked5 {
		if results[i].Persona != p.Name {
			t.Errorf("result[%d].Persona = %q, want %q", i, results[i].Persona, p.Name)
		}
	}
}

// TestFanOutParallel proves that all 5 persona calls are in-flight concurrently.
// The test server blocks until it has seen N concurrent connections, with a timeout.
func TestFanOutParallel(t *testing.T) {
	const wantConcurrent = 5
	var inFlight atomic.Int32
	var peak atomic.Int32
	ready := make(chan struct{})
	var readyOnce atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)

		// track peak concurrency
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}

		// signal once we have wantConcurrent in-flight
		if cur >= wantConcurrent && readyOnce.CompareAndSwap(false, true) {
			close(ready)
		}

		// wait for signal or short timeout so the server unblocks
		select {
		case <-ready:
		case <-time.After(2 * time.Second):
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(personaJSON("parallel"))) //nolint
	}))
	defer srv.Close()

	cl := client.New(srv.URL, "", 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := FanOut(ctx, cl, "test-model", "test topic", "analyst")
	if err != nil {
		t.Fatalf("FanOut error: %v", err)
	}

	if peak.Load() < wantConcurrent {
		t.Errorf("peak concurrent requests = %d, want >= %d (fan-out not parallel)", peak.Load(), wantConcurrent)
	}
}

func TestPersonas_Count(t *testing.T) {
	ps := Personas()
	if len(ps) != 5 {
		t.Fatalf("expected 5 personas, got %d", len(ps))
	}
	names := map[string]bool{}
	for _, p := range ps {
		names[p.Name] = true
	}
	for _, want := range []string{"Practitioner", "Academic", "Skeptic", "Economist", "Historian"} {
		if !names[want] {
			t.Errorf("missing persona %q", want)
		}
	}
}
