package probe

import (
	"context"
	"net"
	"strings"
)

type SystemProbe struct{}

func (p *SystemProbe) Name() string { return "system" }

func (p *SystemProbe) Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult {
	result := NewResult("system", StatusOK, "")
	details := &SystemDetails{}

	proxy := detectProxy(target.Scheme)
	if proxy != "" {
		details.Proxy = proxy
	}

	tunName, tunDesc := detectTUN()
	if tunName != "" {
		details.TUN = tunDesc
		details.TUNName = tunName
	}

	iface := detectOutInterface(target)
	details.Interface = iface

	details.Route = detectRoute(target)

	result.System = details

	var parts []string
	if proxy != "" {
		parts = append(parts, "代理: "+proxy)
		result.Status = StatusWarning
	} else {
		parts = append(parts, "代理: 无")
	}
	if tunName != "" {
		parts = append(parts, "TUN: "+tunDesc)
		result.Status = StatusWarning
	} else {
		parts = append(parts, "TUN: 无")
	}
	parts = append(parts, "出口: "+iface)
	result.Message = strings.Join(parts, " | ")

	return result
}

func detectTUN() (name, desc string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", ""
	}

	// Ordered list: longer prefixes first to ensure correct matching
	type tunEntry struct {
		prefix string
		label  string
	}
	tunEntries := []tunEntry{
		{"wintun", "WinTUN"},
		{"flannel", "Flannel"},
		{"utun", "TUN"},
		{"tun", "TUN"},
		{"tap", "TAP"},
		{"cali", "Calico"},
		{"wg", "WireGuard"},
	}

	for _, iface := range ifaces {
		nameLower := strings.ToLower(iface.Name)
		for _, entry := range tunEntries {
			if strings.HasPrefix(nameLower, entry.prefix) {
				if iface.Flags&net.FlagUp == 0 {
					continue
				}
				return iface.Name, iface.Name + " (" + entry.label + ")"
			}
		}
	}
	return "", ""
}

func detectOutInterface(target *Target) string {
	addr := target.Host
	if target.IsIP {
		addr = target.IP
	}
	conn, err := net.Dial("udp", addr+":80")
	if err != nil {
		return "未知"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return localAddr.IP.String()
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				if ipnet.IP.Equal(localAddr.IP) {
					return iface.Name
				}
			}
		}
	}
	return localAddr.IP.String()
}
