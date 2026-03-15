package probe

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type DNSProbe struct{}

func (p *DNSProbe) Name() string { return "dns" }

func (p *DNSProbe) Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult {
	if target.IsIP {
		return NewResult("dns", StatusSkipped, "目标为 IP 地址，跳过 DNS 解析")
	}

	start := time.Now()
	details := &DNSDetails{}

	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, target.Host)
	elapsed := time.Since(start)

	if err != nil {
		result := NewResult("dns", StatusError, fmt.Sprintf("DNS 解析失败: %v", err))
		result.SetDuration(elapsed)
		result.DNS = details
		return result
	}

	for _, ip := range ips {
		if ip.IP.To4() != nil {
			details.IPv4 = append(details.IPv4, ip.IP.String())
		} else {
			details.IPv6 = append(details.IPv6, ip.IP.String())
		}
	}

	if len(details.IPv4) > 0 {
		target.IP = details.IPv4[0]
	} else if len(details.IPv6) > 0 {
		target.IP = details.IPv6[0]
	}

	details.Server = getSystemDNS()

	checkDNSConsistency(ctx, target.Host, details)

	result := NewResult("dns", StatusOK, "")
	result.SetDuration(elapsed)
	result.DNS = details

	var parts []string
	if len(details.IPv4) > 0 {
		parts = append(parts, details.IPv4[0]+fmt.Sprintf(" (%dms)", elapsed.Milliseconds()))
	}
	if len(details.IPv6) > 0 {
		parts = append(parts, "AAAA: "+details.IPv6[0])
	} else {
		parts = append(parts, "AAAA: 无")
	}

	if details.InternalDomain {
		parts = append(parts, "内部域名")
		result.Status = StatusWarning
	} else if details.Consistent != nil && !*details.Consistent {
		parts = append(parts, "一致性: ❌")
		result.Status = StatusWarning
	} else {
		parts = append(parts, "一致性: ✅")
	}

	result.Message = strings.Join(parts, " | ")
	return result
}

func checkDNSConsistency(ctx context.Context, host string, details *DNSDetails) {
	publicResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}

	pubIPs, err := publicResolver.LookupIPAddr(ctx, host)
	if err != nil {
		details.InternalDomain = true
		return
	}

	pubIPSet := make(map[string]bool)
	for _, ip := range pubIPs {
		pubIPSet[ip.IP.String()] = true
		if details.PublicDNSResult == "" {
			details.PublicDNSResult = ip.IP.String()
		}
	}

	consistent := false
	for _, ip := range details.IPv4 {
		if pubIPSet[ip] {
			consistent = true
			break
		}
	}
	for _, ip := range details.IPv6 {
		if pubIPSet[ip] {
			consistent = true
			break
		}
	}
	details.Consistent = &consistent
}

func getSystemDNS() string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "nameserver") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}
