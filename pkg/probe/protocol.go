package probe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type ProtocolProbe struct{}

func (p *ProtocolProbe) Name() string { return "protocol" }

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

func (p *ProtocolProbe) probeHTTP(ctx context.Context, target *Target) *ProbeResult {
	scheme := target.Scheme
	host := target.Host
	if host == "" {
		host = target.IP
	}

	url := fmt.Sprintf("%s://%s:%d", scheme, host, target.Port)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return NewResult("protocol", StatusError, fmt.Sprintf("HTTP 请求构造失败: %v", err))
	}

	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("HTTP 请求失败: %v", err))
		result.SetDuration(elapsed)
		result.Protocol = &ProtocolDetails{Type: "http"}
		return result
	}
	defer resp.Body.Close()

	details := &ProtocolDetails{
		Type:       "http",
		StatusCode: resp.StatusCode,
	}
	if server := resp.Header.Get("Server"); server != "" {
		details.Version = server
	}

	msg := fmt.Sprintf("%d %s (%dms)", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Milliseconds())
	if details.Version != "" {
		msg += " | Server: " + details.Version
	}

	result := NewResult("protocol", StatusOK, msg)
	result.SetDuration(elapsed)
	result.Protocol = details
	return result
}

func (p *ProtocolProbe) probeMySQL(ctx context.Context, target *Target) *ProbeResult {
	addr := p.addr(target)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, dialTimeout(ctx))
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("MySQL 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "mysql"}
		return result
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(readTimeout(ctx)))

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		result := NewResult("protocol", StatusError, "MySQL 握手失败: 无法读取服务器响应")
		result.Protocol = &ProtocolDetails{Type: "mysql"}
		return result
	}

	payloadLen := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	if payloadLen > 1024 {
		payloadLen = 1024
	}
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		result := NewResult("protocol", StatusError, "MySQL 握手失败: 无法读取完整数据包")
		result.Protocol = &ProtocolDetails{Type: "mysql"}
		return result
	}

	elapsed := time.Since(start)

	if len(payload) < 2 {
		result := NewResult("protocol", StatusError, "MySQL 握手失败: 数据包太短")
		result.Protocol = &ProtocolDetails{Type: "mysql"}
		return result
	}

	if payload[0] == 0xFF {
		result := NewResult("protocol", StatusWarning, "MySQL 服务端返回错误")
		result.SetDuration(elapsed)
		result.Protocol = &ProtocolDetails{Type: "mysql", AuthRequired: true}
		return result
	}

	version := ""
	for i := 1; i < len(payload); i++ {
		if payload[i] == 0 {
			version = string(payload[1:i])
			break
		}
	}

	details := &ProtocolDetails{Type: "mysql", Version: version}
	msg := fmt.Sprintf("MySQL %s (%dms)", version, elapsed.Milliseconds())

	result := NewResult("protocol", StatusOK, msg)
	result.SetDuration(elapsed)
	result.Protocol = details
	return result
}

func (p *ProtocolProbe) probeRedis(ctx context.Context, target *Target) *ProbeResult {
	addr := p.addr(target)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, dialTimeout(ctx))
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("Redis 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "redis"}
		return result
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(readTimeout(ctx)))

	_, err = conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	if err != nil {
		result := NewResult("protocol", StatusError, "Redis PING 发送失败")
		result.Protocol = &ProtocolDetails{Type: "redis"}
		return result
	}

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	elapsed := time.Since(start)

	if err != nil {
		result := NewResult("protocol", StatusError, "Redis 响应读取失败")
		result.Protocol = &ProtocolDetails{Type: "redis"}
		return result
	}

	resp := string(buf[:n])
	details := &ProtocolDetails{Type: "redis"}

	if strings.Contains(resp, "PONG") {
		details.Banner = "PONG"
		msg := fmt.Sprintf("Redis PONG (%dms)", elapsed.Milliseconds())
		result := NewResult("protocol", StatusOK, msg)
		result.SetDuration(elapsed)
		result.Protocol = details
		return result
	}
	if strings.Contains(resp, "NOAUTH") || strings.Contains(resp, "AUTH") {
		details.AuthRequired = true
		details.Banner = strings.TrimSpace(resp)
		msg := fmt.Sprintf("Redis 需要认证 (%dms)", elapsed.Milliseconds())
		result := NewResult("protocol", StatusOK, msg)
		result.SetDuration(elapsed)
		result.Protocol = details
		return result
	}

	details.Banner = strings.TrimSpace(resp)
	msg := fmt.Sprintf("Redis 未知响应: %s", details.Banner)
	result := NewResult("protocol", StatusWarning, msg)
	result.SetDuration(elapsed)
	result.Protocol = details
	return result
}

func (p *ProtocolProbe) probePostgreSQL(ctx context.Context, target *Target) *ProbeResult {
	addr := p.addr(target)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, dialTimeout(ctx))
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("PostgreSQL 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "postgresql"}
		return result
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(readTimeout(ctx)))

	user := "probe"
	database := "postgres"
	startupMsg := buildPGStartup(user, database)
	if _, err := conn.Write(startupMsg); err != nil {
		result := NewResult("protocol", StatusError, "PostgreSQL startup 消息发送失败")
		result.Protocol = &ProtocolDetails{Type: "postgresql"}
		return result
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	elapsed := time.Since(start)

	if err != nil && n == 0 {
		result := NewResult("protocol", StatusError, "PostgreSQL 无响应")
		result.SetDuration(elapsed)
		result.Protocol = &ProtocolDetails{Type: "postgresql"}
		return result
	}

	details := &ProtocolDetails{Type: "postgresql"}

	if n > 0 {
		msgType := buf[0]
		switch msgType {
		case 'R':
			details.AuthRequired = true
			msg := fmt.Sprintf("PostgreSQL 需要认证 (%dms)", elapsed.Milliseconds())
			result := NewResult("protocol", StatusOK, msg)
			result.SetDuration(elapsed)
			result.Protocol = details
			return result
		case 'E':
			msg := fmt.Sprintf("PostgreSQL 返回错误 (%dms)", elapsed.Milliseconds())
			result := NewResult("protocol", StatusOK, msg)
			result.SetDuration(elapsed)
			result.Protocol = details
			return result
		}
	}

	msg := fmt.Sprintf("PostgreSQL 响应 (%dms)", elapsed.Milliseconds())
	result := NewResult("protocol", StatusOK, msg)
	result.SetDuration(elapsed)
	result.Protocol = details
	return result
}

func buildPGStartup(user, database string) []byte {
	params := fmt.Sprintf("user\x00%s\x00database\x00%s\x00\x00", user, database)
	length := 4 + 4 + len(params)
	msg := make([]byte, length)
	msg[0] = byte(length >> 24)
	msg[1] = byte(length >> 16)
	msg[2] = byte(length >> 8)
	msg[3] = byte(length)
	msg[4] = 0
	msg[5] = 3
	msg[6] = 0
	msg[7] = 0
	copy(msg[8:], params)
	return msg
}

func (p *ProtocolProbe) probeSSH(ctx context.Context, target *Target) *ProbeResult {
	addr := p.addr(target)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, dialTimeout(ctx))
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("SSH 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "ssh"}
		return result
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(readTimeout(ctx)))

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	elapsed := time.Since(start)

	if err != nil && n == 0 {
		result := NewResult("protocol", StatusError, "SSH 无 banner 响应")
		result.SetDuration(elapsed)
		result.Protocol = &ProtocolDetails{Type: "ssh"}
		return result
	}

	banner := strings.TrimSpace(string(buf[:n]))
	details := &ProtocolDetails{Type: "ssh", Banner: banner}

	if strings.HasPrefix(banner, "SSH-") {
		parts := strings.SplitN(banner, "-", 3)
		if len(parts) >= 3 {
			details.Version = parts[2]
		}
	}

	msg := fmt.Sprintf("SSH %s (%dms)", banner, elapsed.Milliseconds())
	result := NewResult("protocol", StatusOK, msg)
	result.SetDuration(elapsed)
	result.Protocol = details
	return result
}

func (p *ProtocolProbe) probeGenericTCP(ctx context.Context, target *Target) *ProbeResult {
	details := &ProtocolDetails{Type: "tcp"}
	start := time.Now()

	addr := p.addr(target)
	conn, err := net.DialTimeout("tcp", addr, dialTimeout(ctx))
	if err != nil {
		elapsed := time.Since(start)
		result := NewResult("protocol", StatusError, fmt.Sprintf("TCP 重连失败: %v", err))
		result.SetDuration(elapsed)
		result.Protocol = details
		return result
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(readTimeout(ctx)))
	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	if n > 0 {
		details.Banner = strings.TrimSpace(string(buf[:n]))
	}

	elapsed := time.Since(start)
	msg := "TCP 连接成功"
	if details.Banner != "" {
		msg += " | Banner: " + details.Banner
	}
	result := NewResult("protocol", StatusOK, msg)
	result.SetDuration(elapsed)
	result.Protocol = details
	return result
}

func (p *ProtocolProbe) addr(target *Target) string {
	host := target.Host
	if target.IsIP || host == "" {
		host = target.IP
	}
	return fmt.Sprintf("%s:%d", host, target.Port)
}

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

func dialTimeout(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		return time.Until(deadline)
	}
	return 10 * time.Second
}

func readTimeout(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 5*time.Second {
			return remaining
		}
	}
	return 5 * time.Second
}
