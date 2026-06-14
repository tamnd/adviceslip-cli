package adviceslip_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/adviceslip-cli/adviceslip"
)

func newTestClient(ts *httptest.Server) *adviceslip.Client {
	cfg := adviceslip.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return adviceslip.NewClient(cfg)
}

const mockRandomResponse = `{"slip":{"id":42,"advice":"Do not take life too seriously. You will never get out of it alive."}}`

const mockGetResponse = `{"slip":{"id":91,"advice":"Drink a glass of water before meals."}}`

const mockSearchResponse = `{"slips":[{"id":1,"advice":"Remember to be kind."},{"id":2,"advice":"Stay positive always."}]}`

const mockNoResultsResponse = `{"message":{"type":"warning","text":"No advice slips found matching that search term."}}`

func TestRandomSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockRandomResponse)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Random(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestRandomParsesSlip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API sends text/html even though it's JSON — must still decode correctly
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockRandomResponse)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	slip, err := c.Random(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if slip.ID != 42 {
		t.Errorf("slip.ID = %d, want 42", slip.ID)
	}
	if slip.Advice != "Do not take life too seriously. You will never get out of it alive." {
		t.Errorf("slip.Advice = %q, unexpected", slip.Advice)
	}
}

func TestGetByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/advice/91" {
			t.Errorf("path = %q, want /advice/91", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockGetResponse)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	slip, err := c.Get(context.Background(), 91)
	if err != nil {
		t.Fatal(err)
	}
	if slip.ID != 91 {
		t.Errorf("slip.ID = %d, want 91", slip.ID)
	}
	if slip.Advice != "Drink a glass of water before meals." {
		t.Errorf("slip.Advice = %q, unexpected", slip.Advice)
	}
}

func TestSearchParsesSlips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockSearchResponse)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	slips, err := c.Search(context.Background(), "positive", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(slips) != 2 {
		t.Fatalf("len(slips) = %d, want 2", len(slips))
	}
	if slips[0].ID != 1 {
		t.Errorf("slips[0].ID = %d, want 1", slips[0].ID)
	}
	if slips[1].Advice != "Stay positive always." {
		t.Errorf("slips[1].Advice = %q, unexpected", slips[1].Advice)
	}
}

func TestSearchNoResultsReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockNoResultsResponse)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	slips, err := c.Search(context.Background(), "zzznomatch", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slips) != 0 {
		t.Errorf("len(slips) = %d, want 0 (no-results must return empty slice)", len(slips))
	}
}

func TestSearchLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockSearchResponse)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	// Ask for only 1 result from a 2-item response
	slips, err := c.Search(context.Background(), "positive", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(slips) != 1 {
		t.Errorf("len(slips) = %d, want 1 (limit enforced)", len(slips))
	}
}

func TestRetryOn503(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, mockRandomResponse)
	}))
	defer srv.Close()

	cfg := adviceslip.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := adviceslip.NewClient(cfg)

	start := time.Now()
	_, err := c.Random(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}
