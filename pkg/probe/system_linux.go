//go:build linux
// +build linux

package probe

import (
	"os"
	"os/exec"
	"strings"
)

func detectProxy(scheme string) string {
	for _, env := range []string{"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "https_proxy", "http_proxy", "all_proxy"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
}

func detectRoute(target *Target) string {
	addr := target.Host
	if target.IsIP {
		addr = target.IP
	}
	out, err := exec.Command("ip", "route", "get", addr).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	parts := strings.Fields(s)
	var dev, via string
	for i, p := range parts {
		if p == "dev" && i+1 < len(parts) {
			dev = parts[i+1]
		}
		if p == "via" && i+1 < len(parts) {
			via = parts[i+1]
		}
	}
	if dev != "" && via != "" {
		return dev + " → " + via + " → 目标"
	}
	if dev != "" {
		return dev + " → 目标"
	}
	return ""
}
