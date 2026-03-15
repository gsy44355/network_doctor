package probe

import (
	"context"
	"testing"
)

func TestConnProbe_SuccessfulConnection(t *testing.T) {
	p := &ConnProbe{}
	target := &Target{Raw: "google.com:443", Host: "google.com", IP: "142.250.80.46", Port: 443, Scheme: "https"}
	result := p.Run(context.Background(), target, nil)
	if result.Status == StatusError {
		t.Skipf("Connection failed (network issue): %s", result.Message)
	}
	if result.Status != StatusOK {
		t.Errorf("expected OK, got %v: %s", result.Status, result.Message)
	}
	if result.Conn == nil {
		t.Fatal("Conn details should be populated")
	}
}

func TestConnProbe_SkipsWhenDNSFailed(t *testing.T) {
	p := &ConnProbe{}
	target := &Target{Raw: "nonexistent.invalid", Host: "nonexistent.invalid", Port: 443, Scheme: "https"}
	prev := map[string]*ProbeResult{
		"dns": {Name: "dns", Status: StatusError},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status != StatusSkipped {
		t.Errorf("expected Skipped when DNS failed, got %v", result.Status)
	}
}
