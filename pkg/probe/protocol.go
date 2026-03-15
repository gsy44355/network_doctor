package probe

import (
	"context"
	"crypto/tls"
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

	switch target.Scheme {
	case "http", "https":
		return p.probeHTTP(ctx, target)
	case "mysql":
		return p.probeMySQL(ctx, target)
	case "redis":
		return p.probeRedis(ctx, target)
	case "postgresql":
		return p.probePostgreSQL(ctx, target)
	case "ssh":
		return p.probeSSH(ctx, target)
	default:
		return p.probeGenericTCP(ctx, target)
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

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("MySQL 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "mysql"}
		return result
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

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

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("Redis 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "redis"}
		return result
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

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

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("PostgreSQL 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "postgresql"}
		return result
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

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

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("SSH 连接失败: %v", err))
		result.Protocol = &ProtocolDetails{Type: "ssh"}
		return result
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

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

	addr := p.addr(target)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		result := NewResult("protocol", StatusError, fmt.Sprintf("TCP 重连失败: %v", err))
		result.Protocol = details
		return result
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	if n > 0 {
		details.Banner = strings.TrimSpace(string(buf[:n]))
	}

	msg := "TCP 连接成功"
	if details.Banner != "" {
		msg += " | Banner: " + details.Banner
	}
	result := NewResult("protocol", StatusOK, msg)
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
