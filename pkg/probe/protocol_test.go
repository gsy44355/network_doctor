package probe

import (
	"context"
	"testing"
)

func TestProtocolProbe_HTTP(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "https", Host: "google.com", IP: "142.250.80.46", Port: 443}
	prev := map[string]*ProbeResult{
		"conn": {Name: "conn", Status: StatusOK},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status == StatusError {
		t.Skipf("HTTP request failed (network): %s", result.Message)
	}
	if result.Protocol == nil {
		t.Fatal("Protocol details should be populated")
	}
	if result.Protocol.Type != "http" {
		t.Errorf("type = %q, want http", result.Protocol.Type)
	}
}

func TestProtocolProbe_SkipsWhenTCPFailed(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "mysql", Host: "db", Port: 3306}
	prev := map[string]*ProbeResult{
		"conn": {Name: "conn", Status: StatusError},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status != StatusSkipped {
		t.Errorf("expected Skipped, got %v", result.Status)
	}
}

func TestProtocolDetails_ProxyRelayFields(t *testing.T) {
	details := &ProtocolDetails{
		Type:             "http",
		ProxyRelayFailed: true,
		ProxyChain:       []string{"Proxy", "HK-Node"},
	}
	if !details.ProxyRelayFailed {
		t.Error("ProxyRelayFailed should be true")
	}
	if len(details.ProxyChain) != 2 || details.ProxyChain[0] != "Proxy" {
		t.Error("ProxyChain should contain [Proxy, HK-Node]")
	}
}

func TestProtocolProbe_GenericTCP(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "tcp", Host: "example.com", Port: 8080}
	prev := map[string]*ProbeResult{
		"conn": {Name: "conn", Status: StatusOK},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status == StatusError {
		t.Skipf("TCP connection failed (network): %s", result.Message)
	}
	if result.Protocol == nil {
		t.Fatal("Protocol details should be populated")
	}
	if result.Protocol.Type != "tcp" {
		t.Errorf("type = %q, want tcp", result.Protocol.Type)
	}
}
