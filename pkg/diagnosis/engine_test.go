package diagnosis

import (
	"strings"
	"testing"

	"github.com/network-doctor/network-doctor/pkg/probe"
)

func boolPtr(b bool) *bool { return &b }

func TestDiagnose_AllOK(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"tls":      {Name: "tls", Status: probe.StatusOK},
		"protocol": {Name: "protocol", Status: probe.StatusOK},
	}
	d := Diagnose(results)
	if !d.Reachable {
		t.Error("expected reachable")
	}
	if d.Summary != "目标可达" {
		t.Errorf("summary = %q, want 目标可达", d.Summary)
	}
}

func TestDiagnose_DNSFailed(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system": {Name: "system", Status: probe.StatusOK},
		"dns":    {Name: "dns", Status: probe.StatusError},
	}
	d := Diagnose(results)
	if d.Reachable {
		t.Error("should not be reachable when DNS fails")
	}
	if d.Summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestDiagnose_TCPTimeout(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system": {Name: "system", Status: probe.StatusOK},
		"dns":    {Name: "dns", Status: probe.StatusOK},
		"conn":   {Name: "conn", Status: probe.StatusError, Conn: &probe.ConnDetails{Port: 443, ErrorType: "timeout"}},
	}
	d := Diagnose(results)
	if d.Reachable {
		t.Error("should not be reachable")
	}
	if d.Suggestion == "" {
		t.Error("should have a suggestion")
	}
}

func TestDiagnose_TCPRefused(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system": {Name: "system", Status: probe.StatusOK},
		"dns":    {Name: "dns", Status: probe.StatusOK},
		"conn":   {Name: "conn", Status: probe.StatusError, Conn: &probe.ConnDetails{Port: 3306, ErrorType: "refused"}},
	}
	d := Diagnose(results)
	if d.Reachable {
		t.Error("should not be reachable")
	}
}

func TestDiagnose_MITMDetected(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"tls":      {Name: "tls", Status: probe.StatusWarning, TLS: &probe.TLSDetails{MITM: true, MITMDetail: "企业代理证书 (Zscaler)"}},
		"protocol": {Name: "protocol", Status: probe.StatusOK},
	}
	d := Diagnose(results)
	if len(d.Warnings) == 0 {
		t.Error("should have warnings for MITM")
	}
}

func TestDiagnose_TUNDetected(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK, System: &probe.SystemDetails{TUN: "utun3 (Clash)", TUNName: "utun3"}},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"protocol": {Name: "protocol", Status: probe.StatusOK},
	}
	d := Diagnose(results)
	if len(d.Warnings) == 0 {
		t.Error("should have warning for TUN")
	}
}

func TestDiagnose_InternalDomain(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system": {Name: "system", Status: probe.StatusOK},
		"dns":    {Name: "dns", Status: probe.StatusOK, DNS: &probe.DNSDetails{InternalDomain: true}},
		"conn":   {Name: "conn", Status: probe.StatusOK},
	}
	d := Diagnose(results)
	found := false
	for _, w := range d.Warnings {
		if w == "内部域名，仅在当前 DNS 可解析" {
			found = true
		}
	}
	if !found {
		t.Error("should warn about internal domain")
	}
}

func TestDiagnose_PublicDNSErrorDoesNotWarnAsInternalDomain(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system": {Name: "system", Status: probe.StatusOK},
		"dns":    {Name: "dns", Status: probe.StatusOK, DNS: &probe.DNSDetails{PublicDNSError: "i/o timeout"}},
		"conn":   {Name: "conn", Status: probe.StatusOK},
	}
	d := Diagnose(results)
	for _, w := range d.Warnings {
		if strings.Contains(w, "内部域名") {
			t.Fatalf("should not warn internal domain when public DNS transport failed: %v", d.Warnings)
		}
	}
	if len(d.Warnings) == 0 {
		t.Fatal("should warn that public DNS consistency could not be checked")
	}
}

func TestDiagnose_TLSVerifyWarning(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"tls":      {Name: "tls", Status: probe.StatusWarning, TLS: &probe.TLSDetails{SNIMatch: true, ValidChain: false, VerifyError: "certificate expired", Expired: true}},
		"protocol": {Name: "protocol", Status: probe.StatusOK},
	}
	d := Diagnose(results)
	found := false
	for _, w := range d.Warnings {
		if strings.Contains(w, "证书已过期") {
			found = true
		}
	}
	if !found {
		t.Fatalf("should warn expired certificate, got %v", d.Warnings)
	}
}

func TestDiagnose_HTTPWarningRemainsReachableWithWarning(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"tls":      {Name: "tls", Status: probe.StatusOK},
		"protocol": {Name: "protocol", Status: probe.StatusWarning, Protocol: &probe.ProtocolDetails{Type: "http", StatusCode: 403}},
	}
	d := Diagnose(results)
	if !d.Reachable {
		t.Fatal("4xx HTTP response should remain network-reachable")
	}
	found := false
	for _, w := range d.Warnings {
		if strings.Contains(w, "HTTP 403") {
			found = true
		}
	}
	if !found {
		t.Fatalf("should warn HTTP status code, got %v", d.Warnings)
	}
}

func TestDiagnose_HTTPServerErrorRemainsReachableWithWarning(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"tls":      {Name: "tls", Status: probe.StatusOK},
		"protocol": {Name: "protocol", Status: probe.StatusWarning, Protocol: &probe.ProtocolDetails{Type: "http", StatusCode: 502}},
	}
	d := Diagnose(results)
	if !d.Reachable {
		t.Fatal("5xx HTTP response should remain network-reachable")
	}
	found := false
	for _, w := range d.Warnings {
		if strings.Contains(w, "HTTP 502") {
			found = true
		}
	}
	if !found {
		t.Fatalf("should warn HTTP 502, got %v", d.Warnings)
	}
}

func TestDiagnose_ProxyRelayFailed_WithChain(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK, System: &probe.SystemDetails{TUN: "utun3 (Clash)", TUNName: "utun3"}},
		"clash":    {Name: "clash", Status: probe.StatusOK, Clash: &probe.ClashDetails{Available: true, APIAddr: "127.0.0.1:9090"}},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK, Conn: &probe.ConnDetails{Port: 8888}},
		"protocol": {Name: "protocol", Status: probe.StatusError, Protocol: &probe.ProtocolDetails{Type: "http", ProxyRelayFailed: true, ProxyChain: []string{"Proxy", "HK-Node"}}},
	}
	d := Diagnose(results)
	if d.Reachable {
		t.Error("should not be reachable when proxy relay fails")
	}
	if !strings.Contains(d.Summary, "代理转发") {
		t.Errorf("summary should mention proxy relay, got: %s", d.Summary)
	}
	if !strings.Contains(d.Suggestion, "HK-Node") {
		t.Errorf("suggestion should mention proxy chain node, got: %s", d.Suggestion)
	}
}

func TestDiagnose_ProxyRelayFailed_NoChain(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK, System: &probe.SystemDetails{TUN: "utun3 (Clash)", TUNName: "utun3"}},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK, Conn: &probe.ConnDetails{Port: 443}},
		"protocol": {Name: "protocol", Status: probe.StatusError, Protocol: &probe.ProtocolDetails{Type: "http", ProxyRelayFailed: true}},
	}
	d := Diagnose(results)
	if d.Reachable {
		t.Error("should not be reachable when proxy relay fails")
	}
	if !strings.Contains(d.Summary, "代理转发") {
		t.Errorf("summary should mention proxy relay, got: %s", d.Summary)
	}
	if d.Suggestion == "" {
		t.Error("should have suggestion for proxy relay failure")
	}
}

func TestDiagnose_ProtocolError_NotProxyRelay(t *testing.T) {
	results := map[string]*probe.ProbeResult{
		"system":   {Name: "system", Status: probe.StatusOK},
		"dns":      {Name: "dns", Status: probe.StatusOK},
		"conn":     {Name: "conn", Status: probe.StatusOK},
		"protocol": {Name: "protocol", Status: probe.StatusError, Protocol: &probe.ProtocolDetails{Type: "http", ProxyRelayFailed: false}},
	}
	d := Diagnose(results)
	if d.Reachable {
		t.Error("should not be reachable")
	}
	if strings.Contains(d.Summary, "代理转发") {
		t.Error("summary should NOT mention proxy relay for non-proxy errors")
	}
}
