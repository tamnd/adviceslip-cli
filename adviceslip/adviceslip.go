// Package adviceslip is the library behind the adviceslip command line:
// the HTTP client, request shaping, and typed data models for api.adviceslip.com.
//
// The Client sets a real User-Agent, paces requests, and retries transient
// failures (429 and 5xx) with exponential backoff. Two operations are provided:
// get a random advice slip and search for slips by keyword.
package adviceslip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "api.adviceslip.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Config holds tunable knobs for the HTTP client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible defaults for production use.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: "adviceslip-cli/0.1.0 (github.com/tamnd/adviceslip-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to api.adviceslip.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Slip is a single piece of advice returned by the API.
type Slip struct {
	ID     int    `json:"id"`
	Advice string `json:"advice"`
}

// internal response shapes

type randomResponse struct {
	Slip *Slip `json:"slip"`
}

type searchResponse struct {
	Slips   []Slip         `json:"slips"`
	Message *searchMessage `json:"message"`
}

type searchMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Random returns a single random advice slip.
func (c *Client) Random(ctx context.Context) (*Slip, error) {
	b, err := c.get(ctx, c.cfg.BaseURL+"/advice")
	if err != nil {
		return nil, err
	}
	var resp randomResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("decode random advice: %w", err)
	}
	if resp.Slip == nil {
		return nil, fmt.Errorf("empty slip in response")
	}
	return resp.Slip, nil
}

// Search returns advice slips matching the query. Returns an empty slice (not
// an error) when the API reports no results. Limit is applied client-side.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Slip, error) {
	rawURL := c.cfg.BaseURL + "/advice/search/" + url.PathEscape(query)
	b, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var resp searchResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	// API returns {"message":{...}} when no results found
	if resp.Message != nil {
		return []Slip{}, nil
	}
	slips := resp.Slips
	if limit > 0 && len(slips) > limit {
		slips = slips[:limit]
	}
	return slips, nil
}

// get fetches url and returns the response body. It paces and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
