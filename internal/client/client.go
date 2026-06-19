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
// It retries once on 5xx or timeout errors with a short backoff.
// ponytail: single fixed backoff — upgrade: exponential backoff if API becomes flaky.
func (c *Client) Chat(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	var (
		result string
		err    error
	)
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			// ponytail: fixed 1 s backoff on retry — upgrade: exponential if latency variance grows.
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("client: context cancelled before retry: %w", ctx.Err())
			case <-time.After(time.Second):
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
		// A timeout means the server is slow — retrying only doubles the wall time,
		// so surface it directly. Connection-level errors are retried once.
		if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
			return "", fmt.Errorf("client: http do (timeout): %w", err)
		}
		return "", &retryableError{fmt.Errorf("client: http do: %w", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("client: read body: %w", err)
	}

	if resp.StatusCode >= 500 {
		return "", &retryableError{fmt.Errorf("client: server error %d: %s", resp.StatusCode, raw)}
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

// retryableError marks errors eligible for the single retry.
type retryableError struct{ cause error }

func (e *retryableError) Error() string { return e.cause.Error() }
func (e *retryableError) Unwrap() error { return e.cause }

func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}
