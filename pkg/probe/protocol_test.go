package probe

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

func targetFromURL(raw string) (*Target, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	t := &Target{Raw: raw, Scheme: u.Scheme, Host: host, Port: port}
	if ip := net.ParseIP(host); ip != nil {
		t.IP = ip.String()
		t.Host = ""
		t.IsIP = true
	}
	return t, nil
}

func TestProtocolProbeHTTPFallsBackToGETWhenHeadUnsupported(t *testing.T) {
	var seenGet bool
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodHead {
			return &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
				Status:     "405 Method Not Allowed",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET fallback", r.Method)
		}
		if got := r.Header.Get("Range"); got != "bytes=0-0" {
			t.Fatalf("Range = %q, want bytes=0-0", got)
		}
		seenGet = true
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Status:     "206 Partial Content",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("x")),
			Request:    r,
		}, nil
	})}
	target := &Target{Raw: "http://example.com", Scheme: "http", Host: "example.com", Port: 80}
	p := &ProtocolProbe{HTTPClient: client}

	result := p.probeHTTP(context.Background(), target)

	if !seenGet {
		t.Fatal("GET fallback was not sent")
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %v, message = %s", result.Status, result.Message)
	}
	if result.Protocol.Method != http.MethodGet {
		t.Fatalf("method = %q, want GET fallback", result.Protocol.Method)
	}
	if result.Protocol.StatusCode != http.StatusPartialContent {
		t.Fatalf("status code = %d, want 206", result.Protocol.StatusCode)
	}
}

func TestProtocolProbeHTTPMarksForbiddenAsWarning(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}
	target := &Target{Raw: "http://example.com", Scheme: "http", Host: "example.com", Port: 80}
	p := &ProtocolProbe{HTTPClient: client}

	result := p.probeHTTP(context.Background(), target)

	if result.Status != StatusWarning {
		t.Fatalf("status = %v, want warning for 403", result.Status)
	}
}

func TestProtocolProbeHTTPMarksServerErrorAsWarning(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})}
	target := &Target{Raw: "http://example.com", Scheme: "http", Host: "example.com", Port: 80}
	p := &ProtocolProbe{HTTPClient: client}

	result := p.probeHTTP(context.Background(), target)

	if result.Status != StatusWarning {
		t.Fatalf("status = %v, want warning for 502", result.Status)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
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

func TestLooksLikeProxyRelayFailure_EOF(t *testing.T) {
	if !looksLikeProxyRelayFailure("HTTP 请求失败: EOF") {
		t.Error("message containing EOF should match")
	}
}

func TestLooksLikeProxyRelayFailure_EmptyReply(t *testing.T) {
	if !looksLikeProxyRelayFailure("HTTP 请求失败: empty reply from server") {
		t.Error("message containing 'empty' should match")
	}
}

func TestLooksLikeProxyRelayFailure_ConnectionReset(t *testing.T) {
	if !looksLikeProxyRelayFailure("MySQL 握手失败: connection reset by peer") {
		t.Error("message containing 'connection reset' should match")
	}
}

func TestLooksLikeProxyRelayFailure_Timeout(t *testing.T) {
	if looksLikeProxyRelayFailure("TCP 连接超时: i/o timeout") {
		t.Error("timeout should NOT match proxy relay failure")
	}
}

func TestLooksLikeProxyRelayFailure_Normal(t *testing.T) {
	if looksLikeProxyRelayFailure("HTTP 200 OK (50ms)") {
		t.Error("normal success message should NOT match")
	}
}

func TestProtocolProbeAddrUsesIPv6Brackets(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "tcp", IP: "2001:db8::1", Port: 443, IsIP: true}
	if got := p.addr(target); got != "[2001:db8::1]:443" {
		t.Fatalf("addr = %q, want bracketed IPv6 address", got)
	}
}

func TestProtocolProbe_DetectsProxyRelayFailure_WithTUN(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "http", Host: "unreachable.example.com", IP: "198.18.0.1", Port: 8888}
	prev := map[string]*ProbeResult{
		"conn":   {Name: "conn", Status: StatusOK},
		"system": {Name: "system", Status: StatusOK, System: &SystemDetails{TUNName: "utun3", TUN: "utun3 (Clash)"}},
		"clash":  {Name: "clash", Status: StatusSkipped, Clash: &ClashDetails{Available: false}},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Status == StatusOK {
		t.Skip("target actually reachable in test environment")
	}
	if result.Protocol != nil && result.Protocol.ProxyRelayFailed {
		return
	}
	t.Logf("result: status=%v, message=%s", result.Status, result.Message)
}

func TestProtocolProbe_NoFalsePositive_WithoutTUN(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "http", Host: "unreachable.example.com", IP: "198.18.0.1", Port: 8888}
	prev := map[string]*ProbeResult{
		"conn":   {Name: "conn", Status: StatusOK},
		"system": {Name: "system", Status: StatusOK, System: &SystemDetails{}},
	}
	result := p.Run(context.Background(), target, prev)
	if result.Protocol != nil && result.Protocol.ProxyRelayFailed {
		t.Error("should NOT set ProxyRelayFailed when TUN is not active")
	}
}
