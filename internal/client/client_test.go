package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func okServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint
	}))
}

func TestChat_Success(t *testing.T) {
	srv := okServer(t, "hello world")
	defer srv.Close()

	c := New(srv.URL, "", 5*time.Second)
	got, err := c.Chat(context.Background(), "test-model", "sys", "user")
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestChat_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer srv.Close()

	const testKey = "my-secret-key"
	c := New(srv.URL, testKey, 5*time.Second)
	_, err := c.Chat(context.Background(), "m", "s", "u")
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if want := "Bearer " + testKey; gotAuth != want {
		t.Errorf("auth header = %q, want %q", gotAuth, want)
	}
}

func TestChat_NoAuthWhenKeyEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer srv.Close()

	c := New(srv.URL, "", 5*time.Second)
	_, err := c.Chat(context.Background(), "m", "s", "u")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no auth header, got %q", gotAuth)
	}
}

func TestChat_Non2xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "", 5*time.Second)
	_, err := c.Chat(context.Background(), "m", "s", "u")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestChat_5xxRetries(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error")) //nolint
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "recovered"}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer srv.Close()

	c := New(srv.URL, "", 5*time.Second)
	got, err := c.Chat(context.Background(), "m", "s", "u")
	if err != nil {
		t.Fatalf("Chat() error after retry: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if got != "recovered" {
		t.Errorf("got %q after retry", got)
	}
}

func TestChat_TrailingSlashBase(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer srv.Close()

	// base URL has trailing slash
	c := New(srv.URL+"/", "", 5*time.Second)
	_, err := c.Chat(context.Background(), "m", "s", "u")
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("URL path = %q, want %q", gotPath, "/chat/completions")
	}
}

func TestChat_ContextCancellation(t *testing.T) {
	// Use a channel to signal to the handler when the test is done
	// so the goroutine can exit cleanly before srv.Close().
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer func() {
		close(done)
		srv.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	c := New(srv.URL, "", 5*time.Second)
	_, err := c.Chat(ctx, "m", "s", "u")
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}
