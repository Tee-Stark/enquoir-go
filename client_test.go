package enquoir

import (
	"net/http"
	"testing"
	"time"
)

func TestNewConfig_Defaults(t *testing.T) {
	cfg := newConfig("enq_sk_test", nil)
	if cfg.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, defaultBaseURL)
	}
	if cfg.APIKey != "enq_sk_test" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "enq_sk_test")
	}
}

func TestNewConfig_AppliesOptions(t *testing.T) {
	customClient := &http.Client{}
	cfg := newConfig("k", []Option{
		WithBaseURL("https://staging.example.com"),
		WithHTTPClient(customClient),
		WithTimeout(5 * time.Second),
	})
	if cfg.BaseURL != "https://staging.example.com" {
		t.Errorf("BaseURL = %q, want override", cfg.BaseURL)
	}
	if cfg.HTTPClient != customClient {
		t.Error("HTTPClient was not overridden")
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
}

func TestNewTreasuryClient_AndNewMerchantClient(t *testing.T) {
	if c := NewTreasuryClient("enq_sk_test"); c == nil {
		t.Error("NewTreasuryClient returned nil")
	}
	if c := NewMerchantClient("enq_mk_test"); c == nil {
		t.Error("NewMerchantClient returned nil")
	}
}
