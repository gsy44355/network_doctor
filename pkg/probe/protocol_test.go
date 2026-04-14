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

func TestParseClashConnections_FindsMatch(t *testing.T) {
	jsonData := []byte(`{
		"connections": [
			{
				"metadata": {
					"host": "example.com",
					"destinationPort": "443",
					"destinationIP": "93.184.216.34"
				},
				"chains": ["Proxy", "HK-Node"],
				"start": "2026-04-14T10:00:00Z"
			},
			{
				"metadata": {
					"host": "other.com",
					"destinationPort": "80",
					"destinationIP": "1.2.3.4"
				},
				"chains": ["DIRECT"],
				"start": "2026-04-14T10:00:00Z"
			}
		]
	}`)

	chain, found := parseClashConnections(jsonData, "example.com", 443)
	if !found {
		t.Fatal("should find matching connection")
	}
	if len(chain) != 2 || chain[0] != "Proxy" || chain[1] != "HK-Node" {
		t.Errorf("chain = %v, want [Proxy, HK-Node]", chain)
	}
}

func TestParseClashConnections_MatchByIP(t *testing.T) {
	jsonData := []byte(`{
		"connections": [
			{
				"metadata": {
					"host": "",
					"destinationPort": "3306",
					"destinationIP": "10.0.0.5"
				},
				"chains": ["Proxy", "SG-Node"],
				"start": "2026-04-14T10:00:00Z"
			}
		]
	}`)

	chain, found := parseClashConnections(jsonData, "10.0.0.5", 3306)
	if !found {
		t.Fatal("should find matching connection by IP")
	}
	if len(chain) != 2 || chain[1] != "SG-Node" {
		t.Errorf("chain = %v, want [Proxy, SG-Node]", chain)
	}
}

func TestParseClashConnections_NoMatch(t *testing.T) {
	jsonData := []byte(`{
		"connections": [
			{
				"metadata": {
					"host": "other.com",
					"destinationPort": "80",
					"destinationIP": "1.2.3.4"
				},
				"chains": ["DIRECT"],
				"start": "2026-04-14T10:00:00Z"
			}
		]
	}`)

	_, found := parseClashConnections(jsonData, "example.com", 443)
	if found {
		t.Error("should not find matching connection")
	}
}

func TestParseClashConnections_InvalidJSON(t *testing.T) {
	_, found := parseClashConnections([]byte(`invalid`), "example.com", 443)
	if found {
		t.Error("should not find match for invalid JSON")
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
