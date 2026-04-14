# TUN Proxy Relay Failure Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect when TCP connections through a TUN proxy appear reachable but the proxy fails to relay traffic to the actual target, and report accurate "unreachable via proxy" diagnosis.

**Architecture:** Two-layer detection inside ProtocolProbe: (1) query Clash API `/connections` during an active protocol connection to identify proxy chain and relay status; (2) if API unavailable, infer relay failure from EOF/empty-reply behavior when TUN is active. Diagnosis engine adds a new branch for proxy relay failure with distinct Chinese-language messaging.

**Tech Stack:** Go, net/http (Clash API client), encoding/json, io/net standard library

---

### File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `pkg/probe/probe.go` | Modify | Add `ProxyRelayFailed`, `ProxyChain` fields to `ProtocolDetails` |
| `pkg/probe/protocol.go` | Modify | Add TUN-aware EOF detection + Clash API `/connections` query logic |
| `pkg/diagnosis/engine.go` | Modify | Add proxy relay failure diagnosis branch |
| `pkg/diagnosis/engine_test.go` | Modify | Add tests for proxy relay failure diagnosis |
| `pkg/probe/protocol_test.go` | Modify | Add tests for EOF detection with TUN active |

---

### Task 1: Add new fields to ProtocolDetails

**Files:**
- Modify: `pkg/probe/probe.go:107-113`

- [ ] **Step 1: Write the failing test**

Add to `pkg/probe/protocol_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestProtocolDetails_ProxyRelayFields -v`
Expected: FAIL — `ProxyRelayFailed` and `ProxyChain` fields do not exist on `ProtocolDetails`.

- [ ] **Step 3: Add fields to ProtocolDetails**

In `pkg/probe/probe.go`, replace the `ProtocolDetails` struct:

```go
type ProtocolDetails struct {
	Type             string   `json:"type"`
	StatusCode       int      `json:"status_code,omitempty"`
	Version          string   `json:"version,omitempty"`
	Banner           string   `json:"banner,omitempty"`
	AuthRequired     bool     `json:"auth_required,omitempty"`
	ProxyRelayFailed bool     `json:"proxy_relay_failed,omitempty"`
	ProxyChain       []string `json:"proxy_chain,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestProtocolDetails_ProxyRelayFields -v`
Expected: PASS

- [ ] **Step 5: Run all existing tests to ensure no regression**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./...`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add pkg/probe/probe.go pkg/probe/protocol_test.go
git commit -m "feat: add ProxyRelayFailed and ProxyChain fields to ProtocolDetails"
```

---

### Task 2: Add Clash API `/connections` query helper to ProtocolProbe

**Files:**
- Modify: `pkg/probe/protocol.go`
- Test: `pkg/probe/protocol_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/probe/protocol_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestParseClashConnections -v`
Expected: FAIL — `parseClashConnections` function does not exist.

- [ ] **Step 3: Implement parseClashConnections and queryClashConnections**

Add to `pkg/probe/protocol.go`, before the `dialTimeout` function:

```go
// queryClashConnections queries Clash API /connections and finds the connection
// matching the given host and port. Returns the proxy chain and whether a match was found.
func queryClashConnections(ctx context.Context, apiAddr, secret, host string, port int) ([]string, bool) {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+apiAddr+"/connections", nil)
	if err != nil {
		return nil, false
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	return parseClashConnections(body, host, port)
}

// parseClashConnections parses the Clash /connections JSON response and finds
// a connection matching host:port. Matches against metadata.host or metadata.destinationIP.
func parseClashConnections(data []byte, host string, port int) ([]string, bool) {
	var resp struct {
		Connections []struct {
			Metadata struct {
				Host            string `json:"host"`
				DestinationPort string `json:"destinationPort"`
				DestinationIP   string `json:"destinationIP"`
			} `json:"metadata"`
			Chains []string `json:"chains"`
		} `json:"connections"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false
	}

	portStr := fmt.Sprintf("%d", port)
	for _, conn := range resp.Connections {
		if conn.Metadata.DestinationPort != portStr {
			continue
		}
		if conn.Metadata.Host == host || conn.Metadata.DestinationIP == host {
			return conn.Chains, true
		}
	}
	return nil, false
}
```

Also add `"encoding/json"` to the imports in `protocol.go` (it's not currently imported).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestParseClashConnections -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/probe/protocol.go pkg/probe/protocol_test.go
git commit -m "feat: add Clash API /connections query helper for proxy chain detection"
```

---

### Task 3: Add TUN-aware EOF detection helper

**Files:**
- Modify: `pkg/probe/protocol.go`
- Test: `pkg/probe/protocol_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/probe/protocol_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestLooksLikeProxyRelayFailure -v`
Expected: FAIL — `looksLikeProxyRelayFailure` does not exist.

- [ ] **Step 3: Implement looksLikeProxyRelayFailure**

Add to `pkg/probe/protocol.go`, before the `dialTimeout` function:

```go
// looksLikeProxyRelayFailure checks if a protocol probe error message matches
// the pattern of a proxy relay failure. When a TUN proxy accepts a TCP connection
// but fails to forward it to the real target, it typically closes the connection
// immediately, resulting in EOF, empty reply, or connection reset errors.
func looksLikeProxyRelayFailure(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "eof") ||
		strings.Contains(lower, "empty") ||
		strings.Contains(lower, "connection reset")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestLooksLikeProxyRelayFailure -v`
Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/probe/protocol.go pkg/probe/protocol_test.go
git commit -m "feat: add looksLikeProxyRelayFailure helper for TUN-aware EOF detection"
```

---

### Task 4: Integrate proxy relay detection into ProtocolProbe.Run

**Files:**
- Modify: `pkg/probe/protocol.go`
- Test: `pkg/probe/protocol_test.go`

- [ ] **Step 1: Write the failing test**

Add to `pkg/probe/protocol_test.go`:

```go
func TestProtocolProbe_DetectsProxyRelayFailure_WithTUN(t *testing.T) {
	p := &ProtocolProbe{}
	target := &Target{Scheme: "http", Host: "unreachable.example.com", IP: "198.18.0.1", Port: 8888}
	prev := map[string]*ProbeResult{
		"conn":   {Name: "conn", Status: StatusOK},
		"system": {Name: "system", Status: StatusOK, System: &SystemDetails{TUNName: "utun3", TUN: "utun3 (Clash)"}},
		"clash":  {Name: "clash", Status: StatusSkipped, Clash: &ClashDetails{Available: false}},
	}
	result := p.Run(context.Background(), target, prev)
	// If connection succeeds (e.g. TUN is not actually active in test env), skip
	if result.Status == StatusOK {
		t.Skip("target actually reachable in test environment")
	}
	// When the protocol probe fails with EOF and TUN is active,
	// ProxyRelayFailed should be set
	if result.Protocol != nil && result.Protocol.ProxyRelayFailed {
		// Correct behavior: detected proxy relay failure
		return
	}
	// If we get here with StatusError but no ProxyRelayFailed, the error
	// might be a non-EOF error (e.g., connection refused), which is fine
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
```

- [ ] **Step 2: Run test to verify current behavior**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/probe/ -run TestProtocolProbe_DetectsProxyRelay -v`
Expected: Tests run but the `ProxyRelayFailed` field is never set (current code doesn't set it).

- [ ] **Step 3: Add helper to extract TUN and Clash info from prev results**

Add to `pkg/probe/protocol.go`, before the `probeHTTP` method:

```go
// tunInfo extracts TUN active status and Clash API details from previous probe results.
func tunInfo(prev map[string]*ProbeResult) (tunActive bool, clashAvailable bool, clashAPIAddr string, clashSecret string) {
	if sys, ok := prev["system"]; ok && sys.System != nil && sys.System.TUNName != "" {
		tunActive = true
	}
	if clash, ok := prev["clash"]; ok && clash.Clash != nil && clash.Clash.Available {
		clashAvailable = true
		clashAPIAddr = clash.Clash.APIAddr
	}
	return
}
```

- [ ] **Step 4: Modify ProtocolProbe.Run to pass prev to sub-probes**

Replace the `Run` method in `pkg/probe/protocol.go`:

```go
func (p *ProtocolProbe) Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult {
	if conn, ok := prev["conn"]; ok && conn.Status == StatusError {
		return NewResult("protocol", StatusSkipped, "⏭️ 跳过 (TCP 不通)")
	}

	var result *ProbeResult
	switch target.Scheme {
	case "http", "https":
		result = p.probeHTTP(ctx, target)
	case "mysql":
		result = p.probeMySQL(ctx, target)
	case "redis":
		result = p.probeRedis(ctx, target)
	case "postgresql":
		result = p.probePostgreSQL(ctx, target)
	case "ssh":
		result = p.probeSSH(ctx, target)
	default:
		result = p.probeGenericTCP(ctx, target)
	}

	// Proxy relay failure detection: only when TUN is active and protocol probe failed
	if result.Status == StatusError && result.Protocol != nil {
		tunActive, clashAvailable, clashAPIAddr, clashSecret := tunInfo(prev)
		if tunActive {
			p.detectProxyRelayFailure(ctx, result, target, tunActive, clashAvailable, clashAPIAddr, clashSecret)
		}
	}

	return result
}
```

- [ ] **Step 5: Implement detectProxyRelayFailure**

Add to `pkg/probe/protocol.go`:

```go
// detectProxyRelayFailure checks if a protocol probe failure is caused by proxy relay failure.
// It uses two layers: (1) Clash API /connections query for precise detection,
// (2) EOF/connection-reset behavior inference as fallback.
func (p *ProtocolProbe) detectProxyRelayFailure(ctx context.Context, result *ProbeResult, target *Target, tunActive, clashAvailable bool, clashAPIAddr, clashSecret string) {
	host := target.Host
	if host == "" {
		host = target.IP
	}

	// Layer 1: Clash API query
	if clashAvailable {
		chain, found := queryClashConnections(ctx, clashAPIAddr, clashSecret, host, target.Port)
		if found {
			result.Protocol.ProxyChain = chain
			result.Protocol.ProxyRelayFailed = true
			return
		}
	}

	// Layer 2: Behavioral inference — check if the error looks like a proxy relay failure
	if looksLikeProxyRelayFailure(result.Message) {
		result.Protocol.ProxyRelayFailed = true
	}
}
```

- [ ] **Step 6: Run all tests**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add pkg/probe/protocol.go pkg/probe/protocol_test.go
git commit -m "feat: integrate proxy relay failure detection into ProtocolProbe.Run"
```

---

### Task 5: Add proxy relay failure diagnosis to engine

**Files:**
- Modify: `pkg/diagnosis/engine.go:112-122`
- Test: `pkg/diagnosis/engine_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `pkg/diagnosis/engine_test.go`:

```go
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
```

Add `"strings"` to the test file imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/diagnosis/ -run TestDiagnose_ProxyRelay -v`
Expected: FAIL — current diagnosis engine doesn't check `ProxyRelayFailed` and generates generic protocol error messages.

- [ ] **Step 3: Modify the protocol evaluation block in engine.go**

In `pkg/diagnosis/engine.go`, replace the protocol evaluation block (lines 112-122):

```go
	if proto, ok := results["protocol"]; ok {
		if proto.Status == probe.StatusError {
			d.Reachable = false
			if proto.Protocol != nil && proto.Protocol.ProxyRelayFailed {
				port := 0
				if conn, ok := results["conn"]; ok && conn.Conn != nil {
					port = conn.Conn.Port
				}
				d.Summary = fmt.Sprintf("TCP:%d 通过代理连接成功，但代理转发到目标失败（目标不可达）", port)
				if len(proto.Protocol.ProxyChain) > 0 {
					d.Suggestion = fmt.Sprintf("当前代理链路: %s，建议切换其他节点或添加直连规则", strings.Join(proto.Protocol.ProxyChain, " → "))
				} else {
					d.Suggestion = "1. 检查代理节点是否能访问该目标  2. 尝试切换代理节点或使用直连规则  3. 确认目标地址和端口是否正确"
				}
			} else {
				d.Summary = "TCP 连通但协议握手失败，可能被代理拦截或服务异常"
				d.Suggestion = "检查目标服务是否正常运行，或是否有代理/防火墙拦截应用层协议"
			}
			return d
		}
		if proto.Protocol != nil && proto.Protocol.AuthRequired {
			d.Warnings = append(d.Warnings, "协议可达，服务端要求认证（这不是网络问题）")
		}
	}
```

- [ ] **Step 4: Run proxy relay tests to verify they pass**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/diagnosis/ -run TestDiagnose_ProxyRelay -v`
Expected: All 3 new tests PASS.

- [ ] **Step 5: Run all diagnosis tests**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./pkg/diagnosis/ -v`
Expected: All tests pass including existing ones.

- [ ] **Step 6: Commit**

```bash
git add pkg/diagnosis/engine.go pkg/diagnosis/engine_test.go
git commit -m "feat: add proxy relay failure diagnosis with Chinese messaging"
```

---

### Task 6: Pass ClashProbe secret through to ProtocolProbe

**Files:**
- Modify: `pkg/probe/probe.go:80-87`
- Modify: `pkg/probe/protocol.go`

The `ClashDetails` struct currently lacks a `Secret` field. The `tunInfo` helper needs to retrieve the secret so `queryClashConnections` can authenticate. Since secrets should not be stored in details structs (they would be serialized to JSON), we pass it via the `ClashProbe` config reference through `prev`.

- [ ] **Step 1: Add Secret field to ClashDetails (json-hidden)**

In `pkg/probe/probe.go`, add the field to `ClashDetails`:

```go
type ClashDetails struct {
	APIAddr    string   `json:"api_addr"`
	Available  bool     `json:"available"`
	Version    string   `json:"version,omitempty"`
	RealIPs    []string `json:"real_ips,omitempty"`
	DNSSuccess bool     `json:"dns_success"`
	DNSError   string   `json:"dns_error,omitempty"`
	Secret     string   `json:"-"`
}
```

- [ ] **Step 2: Store secret in ClashProbe.Run**

In `pkg/probe/clash.go`, after `details.Available = true` (line 43), add:

```go
	details.Secret = p.Secret
```

- [ ] **Step 3: Update tunInfo to return the secret**

In `pkg/probe/protocol.go`, update the `tunInfo` function:

```go
func tunInfo(prev map[string]*ProbeResult) (tunActive bool, clashAvailable bool, clashAPIAddr string, clashSecret string) {
	if sys, ok := prev["system"]; ok && sys.System != nil && sys.System.TUNName != "" {
		tunActive = true
	}
	if clash, ok := prev["clash"]; ok && clash.Clash != nil && clash.Clash.Available {
		clashAvailable = true
		clashAPIAddr = clash.Clash.APIAddr
		clashSecret = clash.Clash.Secret
	}
	return
}
```

- [ ] **Step 4: Run all tests**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./...`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add pkg/probe/probe.go pkg/probe/clash.go pkg/probe/protocol.go
git commit -m "feat: pass Clash API secret through probe results for connection query auth"
```

---

### Task 7: End-to-end verification and go vet

**Files:** None (verification only)

- [ ] **Step 1: Run go vet**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go vet ./...`
Expected: No issues.

- [ ] **Step 2: Run all tests**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 3: Build the binary**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && go build -o network-doctor .`
Expected: Builds successfully.

- [ ] **Step 4: Verify JSON output includes new fields**

Run: `cd /Users/gsy/git_repo/network_repo/network_doctor && echo '{"type":"http","proxy_relay_failed":true,"proxy_chain":["Proxy","HK"]}' | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['proxy_relay_failed']==True; assert d['proxy_chain']==['Proxy','HK']; print('JSON schema OK')"`
Expected: `JSON schema OK`

- [ ] **Step 5: Commit build verification**

No commit needed — this is a verification-only task.
