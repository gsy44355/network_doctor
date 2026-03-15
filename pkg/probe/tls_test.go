package probe

import (
	"context"
	"testing"
)

func TestTLSProbe_SkipsNonTLS(t *testing.T) {
	p := &TLSProbe{}
	target := &Target{Scheme: "mysql", Host: "db.host", Port: 3306}
	result := p.Run(context.Background(), target, nil)
	if result.Status != StatusSkipped {
		t.Errorf("expected Skipped for non-TLS, got %v", result.Status)
	}
}

func TestTLSProbe_SkipsWhenTCPFailed(t *testing.T) {
	p := &TLSProbe{}
	target := &Target{Scheme: "https", Host: "example.com", Port: 443}
	prev := map[string]*ProbeResult{
		"conn": {Name: "conn", Status: StatusError},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status != StatusSkipped {
		t.Errorf("expected Skipped when TCP failed, got %v", result.Status)
	}
}

func TestTLSProbe_SuccessfulHandshake(t *testing.T) {
	p := &TLSProbe{Verbose: true}
	target := &Target{Scheme: "https", Host: "google.com", IP: "142.250.80.46", Port: 443}
	prev := map[string]*ProbeResult{
		"conn": {Name: "conn", Status: StatusOK},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status == StatusError {
		t.Skipf("TLS handshake failed (network issue): %s", result.Message)
	}
	if result.TLS == nil {
		t.Fatal("TLS details should be populated")
	}
	if result.TLS.Version == "" {
		t.Error("TLS version should be set")
	}
	if !result.TLS.SNIMatch {
		t.Error("SNI should match for google.com")
	}
}
