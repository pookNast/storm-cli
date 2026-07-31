// Package client provides an OpenAI-compatible chat completion client.
// It deliberately does NOT import internal/config to keep Wave 0 items independent.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client issues POST /chat/completions against an OpenAI-compatible backend.
type Client struct {
	base    string
	key     string
	timeout time.Duration
	http    *http.Client
}

// New creates a Client. key may be empty (no auth header sent when empty).
func New(base, key string, timeout time.Duration) *Client {
	return &Client{
		base:    base,
		key:     key,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat sends a single system+user turn and returns the assistant's reply.
// It retries transient failures — 5xx, 429 rate limits, connection errors, and
// timeouts — with a bounded exponential backoff (2 s, 4 s). Under concurrent
// load the gateway can transiently stall past the client deadline or rate-limit
// a burst, and a retry after backoff typically clears it. A parent-context
// cancellation is still honoured: the backoff select returns immediately if ctx
// is already done. A 429 Retry-After header (delta-seconds) overrides the
// computed backoff when it asks for a longer wait.
func (c *Client) Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	const maxAttempts = 3 // initial + 2 retries — absorbs a transient stall or rate burst
	var (
		result string
		err    error
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second // 2 s, then 4 s
			var re *retryableError
			if errors.As(err, &re) && re.retryAfter > backoff {
				backoff = re.retryAfter
			}
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("client: context cancelled before retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
		result, err = c.doChat(ctx, model, systemPrompt, userPrompt)
		if err == nil {
			return result, nil
		}
		if !isRetryable(err) {
			return "", err
		}
	}
	return "", err
}

func (c *Client) doChat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("client: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.base, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Retry transport errors including timeouts: under concurrent load the
		// gateway can transiently stall past the http.Client deadline on a single
		// request, and a retry usually clears it. A parent-context cancellation
		// short-circuits the retry loop in Chat() via its ctx.Done() guard.
		return "", &retryableError{cause: fmt.Errorf("client: http do: %w", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("client: read body: %w", err)
	}

	// 429 (rate limit) and 5xx are transient — retry with backoff. A 429 may
	// carry a Retry-After hint that Chat() honours when it exceeds the backoff.
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return "", &retryableError{
			cause:      fmt.Errorf("client: server status %d: %s", resp.StatusCode, raw),
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("client: unexpected status %d: %s", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("client: decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("client: empty choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}

// retryableError marks errors eligible for retry. retryAfter, when non-zero,
// carries a 429 Retry-After hint (delta-seconds) that overrides the computed
// exponential backoff.
type retryableError struct {
	cause      error
	retryAfter time.Duration
}

func (e *retryableError) Error() string { return e.cause.Error() }
func (e *retryableError) Unwrap() error { return e.cause }

func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}

// parseRetryAfter parses a Retry-After header in delta-seconds form into a
// Duration, capped at 30 s so a huge value can't stall the run. Returns 0 on any
// parse failure (the HTTP-date form is intentionally not supported).
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	const cap30 = 30 * time.Second
	d := time.Duration(n) * time.Second
	if d > cap30 {
		return cap30
	}
	return d
}
