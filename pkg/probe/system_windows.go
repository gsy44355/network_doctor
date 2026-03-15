//go:build windows
// +build windows

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
	out, err := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyServer").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "ProxyServer") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func detectRoute(target *Target) string {
	addr := target.Host
	if target.IsIP {
		addr = target.IP
	}
	out, err := exec.Command("route", "print", addr).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, addr) {
			return line
		}
	}
	return ""
}
