package hamnut

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Station-Manager/logging"
	"github.com/Station-Manager/types"
)

// helper to create a Service with minimal initialization and injected test HTTP client and config
func newTestService(baseURL string) *Service {
	return &Service{
		LoggerService: &logging.Service{}, // safe no-op logger
		Config: &types.LookupConfig{
			Name:      ServiceName,
			Enabled:   true,
			URL:       baseURL,
			UserAgent: "test-agent",
		},
		client: http.DefaultClient,
	}
}

func TestLookup_NotInitialized(t *testing.T) {
	lc := &Service{}
	_, err := lc.Lookup("K1")
	if err == nil || err != ErrClientNotInitialized {
		t.Fatalf("expected ErrClientNotInitialized, got %v", err)
	}
}

func TestLookup_Disabled(t *testing.T) {
	lc := &Service{LoggerService: &logging.Service{}}
	lc.initialized.Store(true)
	lc.Config = &types.LookupConfig{Enabled: false}
	c, err := lc.Lookup("K1")
	if err != nil {
		t.Fatalf("expected nil error when disabled, got %v", err)
	}
	if (c != types.Country{}) {
		t.Fatalf("expected zero-value country when disabled, got %+v", c)
	}
}

func TestLookup_EmptyCallsign(t *testing.T) {
	lc := &Service{LoggerService: &logging.Service{}}
	lc.initialized.Store(true)
	lc.Config = &types.LookupConfig{Enabled: true}
	_, err := lc.Lookup("   ")
	if err == nil || err != ErrEmptyCallsign {
		t.Fatalf("expected ErrEmptyCallsign, got %v", err)
	}
}

func TestLookup_HTTPErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer ts.Close()

	lc := newTestService(ts.URL)
	lc.initialized.Store(true)

	_, err := lc.Lookup("K1")
	if err == nil || !strings.Contains(err.Error(), "unexpected status 400") {
		t.Fatalf("expected unexpected status error, got %v", err)
	}
}

func TestLookup_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer ts.Close()

	lc := newTestService(ts.URL)
	lc.initialized.Store(true)

	_, err := lc.Lookup("K1")
	if err == nil || err != ErrPrefixNotFound {
		t.Fatalf("expected ErrPrefixNotFound, got %v", err)
	}
}

func TestLookup_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid json"))
	}))
	defer ts.Close()

	lc := newTestService(ts.URL)
	lc.initialized.Store(true)

	_, err := lc.Lookup("K1")
	if err == nil || !strings.Contains(err.Error(), "unmarshalling JSON") {
		t.Fatalf("expected JSON unmarshal error, got %v", err)
	}
}

func TestLookup_Success(t *testing.T) {
	// Build a minimal JSON body matching hamnut tags on types.Country
	body := `{"localTime":"2025-09-09T12:34:00-05:00","countryName":"United States","prefix":"K","countryCode":"US","continent":"NA","cqZone":"5","ituZone":"8","primaryDXCCPrefix":"291"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("prefix"); got != "K1" {
			t.Fatalf("expected prefix query K1, got %s", got)
		}
		if ua := r.Header.Get("User-Agent"); ua != "test-agent" {
			t.Fatalf("expected User-Agent header 'test-agent', got %s", ua)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	lc := newTestService(ts.URL)
	lc.initialized.Store(true)

	country, err := lc.Lookup("K1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if country.Name != "United States" || country.Prefix != "K" || country.Ccode != "US" || country.Continent != "NA" || country.CQZone != "5" || country.ITUZone != "8" || country.DXCC != "291" || country.TimeOffset != "-05:00" {
		t.Fatalf("unexpected mapping: %+v", country)
	}
}
