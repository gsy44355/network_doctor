package probe

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type ConnProbe struct{}

func (p *ConnProbe) Name() string { return "conn" }

func (p *ConnProbe) Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult {
	if dns, ok := prev["dns"]; ok && dns.Status == StatusError {
		return NewResult("conn", StatusSkipped, "跳过 (DNS 解析失败)")
	}

	addr := fmt.Sprintf("%s:%d", target.IP, target.Port)
	if target.IP == "" {
		addr = fmt.Sprintf("%s:%d", target.Host, target.Port)
	}

	start := time.Now()
	deadline, hasDeadline := ctx.Deadline()
	timeout := 10 * time.Second
	if hasDeadline {
		timeout = time.Until(deadline)
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	details := &ConnDetails{Port: target.Port}

	if err != nil {
		errType := classifyConnError(err)
		details.ErrorType = errType

		var msg string
		switch errType {
		case "refused":
			msg = fmt.Sprintf("TCP:%d: ❌ 连接被拒绝", target.Port)
		case "timeout":
			msg = fmt.Sprintf("TCP:%d: ❌ 连接超时", target.Port)
		case "unreachable":
			msg = fmt.Sprintf("TCP:%d: ❌ 主机不可达", target.Port)
		default:
			msg = fmt.Sprintf("TCP:%d: ❌ %v", target.Port, err)
		}

		result := NewResult("conn", StatusError, msg)
		result.SetDuration(elapsed)
		result.Conn = details
		return result
	}
	conn.Close()

	msg := fmt.Sprintf("TCP:%d: ✅ %dms", target.Port, elapsed.Milliseconds())
	result := NewResult("conn", StatusOK, msg)
	result.SetDuration(elapsed)
	result.Conn = details
	return result
}

func classifyConnError(err error) string {
	s := err.Error()
	if strings.Contains(s, "connection refused") {
		return "refused"
	}
	if strings.Contains(s, "timeout") || strings.Contains(s, "deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(s, "no route") || strings.Contains(s, "unreachable") {
		return "unreachable"
	}
	return "unknown"
}
