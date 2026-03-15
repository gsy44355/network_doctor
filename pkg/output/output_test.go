package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/network-doctor/network-doctor/pkg/diagnosis"
	"github.com/network-doctor/network-doctor/pkg/probe"
)

func makeTestResults() ([]*probe.ProbeResult, *diagnosis.Diagnosis) {
	results := []*probe.ProbeResult{
		{Name: "system", Status: probe.StatusOK, StatusStr: "ok", Message: "代理: 无 | TUN: 无 | 出口: en0",
			System: &probe.SystemDetails{Interface: "en0"}},
		{Name: "dns", Status: probe.StatusOK, StatusStr: "ok", Message: "93.184.216.34 (12ms) | AAAA: 无 | 一致性: ✅",
			Duration: 12 * time.Millisecond, DurationMs: 12,
			DNS: &probe.DNSDetails{IPv4: []string{"93.184.216.34"}}},
		{Name: "conn", Status: probe.StatusOK, StatusStr: "ok", Message: "TCP:443: ✅ 42ms",
			Duration: 42 * time.Millisecond, DurationMs: 42,
			Conn: &probe.ConnDetails{Port: 443}},
	}
	diag := &diagnosis.Diagnosis{Reachable: true, Summary: "目标可达"}
	return results, diag
}

func TestJSONRenderer(t *testing.T) {
	results, diag := makeTestResults()
	var buf bytes.Buffer
	r := &JSONRenderer{}
	err := r.Render(&buf, "https://example.com", results, diag, false)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if out["reachable"] != true {
		t.Error("expected reachable=true")
	}
	if out["diagnosis"] != "目标可达" {
		t.Errorf("diagnosis = %v", out["diagnosis"])
	}
}

func TestTextRenderer(t *testing.T) {
	results, diag := makeTestResults()
	var buf bytes.Buffer
	r := &TextRenderer{NoColor: true}
	err := r.Render(&buf, "https://example.com", results, diag, false)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("[系统]")) {
		t.Error("expected [系统] section")
	}
	if !bytes.Contains([]byte(output), []byte("目标可达")) {
		t.Error("expected 目标可达")
	}
}
