package diagnosis

import (
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
