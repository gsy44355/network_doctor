package target

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/network-doctor/network-doctor/pkg/probe"
)

var portToScheme = map[int]string{
	80: "http", 443: "https", 3306: "mysql",
	6379: "redis", 5432: "postgresql", 22: "ssh",
}

var schemeToPort = map[string]int{
	"http": 80, "https": 443, "mysql": 3306,
	"redis": 6379, "postgresql": 5432, "ssh": 22,
}

func Parse(raw string) (*probe.Target, error) {
	t := &probe.Target{Raw: raw}
	if strings.Contains(raw, "://") {
		if err := parseURI(t, raw); err != nil {
			return nil, err
		}
	} else {
		if err := parseHostPort(t, raw); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func parseURI(t *probe.Target, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("无法解析目标地址: %s", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if idx := strings.Index(scheme, "+"); idx != -1 {
		scheme = scheme[:idx]
	}
	t.Scheme = scheme
	host := u.Hostname()
	portStr := u.Port()
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("无效的端口号: %s", portStr)
		}
		if err := validatePort(p); err != nil {
			return err
		}
		t.Port = p
	} else if dp, ok := schemeToPort[t.Scheme]; ok {
		t.Port = dp
	} else {
		return fmt.Errorf("未知协议且未指定端口: %s", t.Scheme)
	}
	if ip := net.ParseIP(host); ip != nil {
		t.IP = host
		t.IsIP = true
	} else {
		t.Host = host
	}
	return nil
}

func parseHostPort(t *probe.Target, raw string) error {
	host, portStr, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
		t.Port = 443
		t.Scheme = "https"
	} else {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("无效的端口号: %s", portStr)
		}
		if err := validatePort(p); err != nil {
			return err
		}
		t.Port = p
		if scheme, ok := portToScheme[p]; ok {
			t.Scheme = scheme
		} else {
			t.Scheme = "tcp"
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		t.IP = host
		t.IsIP = true
	} else {
		t.Host = host
	}
	return nil
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("端口号超出范围: %d", port)
	}
	return nil
}

func ParseLines(content string) ([]*probe.Target, error) {
	var targets []*probe.Target
	for lineNo, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		t, err := Parse(line)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行解析失败: %w", lineNo+1, err)
		}
		targets = append(targets, t)
	}
	return targets, nil
}
