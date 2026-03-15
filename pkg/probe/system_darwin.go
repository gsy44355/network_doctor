//go:build darwin
// +build darwin

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

	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(out), "\n")
	var httpEnabled, httpsEnabled bool
	var httpProxy, httpsProxy, httpPort, httpsPort string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HTTPEnable : 1") {
			httpEnabled = true
		}
		if strings.HasPrefix(line, "HTTPSEnable : 1") {
			httpsEnabled = true
		}
		if strings.HasPrefix(line, "HTTPProxy :") {
			httpProxy = strings.TrimSpace(strings.TrimPrefix(line, "HTTPProxy :"))
		}
		if strings.HasPrefix(line, "HTTPSProxy :") {
			httpsProxy = strings.TrimSpace(strings.TrimPrefix(line, "HTTPSProxy :"))
		}
		if strings.HasPrefix(line, "HTTPPort :") {
			httpPort = strings.TrimSpace(strings.TrimPrefix(line, "HTTPPort :"))
		}
		if strings.HasPrefix(line, "HTTPSPort :") {
			httpsPort = strings.TrimSpace(strings.TrimPrefix(line, "HTTPSPort :"))
		}
	}

	if scheme == "https" && httpsEnabled && httpsProxy != "" {
		if httpsPort != "" && httpsPort != "0" {
			return httpsProxy + ":" + httpsPort
		}
		return httpsProxy
	}
	if httpEnabled && httpProxy != "" {
		if httpPort != "" && httpPort != "0" {
			return httpProxy + ":" + httpPort
		}
		return httpProxy
	}
	return ""
}

func detectRoute(target *Target) string {
	addr := target.Host
	if target.IsIP {
		addr = target.IP
	}
	out, err := exec.Command("route", "-n", "get", addr).Output()
	if err != nil {
		return ""
	}
	var iface, gateway string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			iface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
		if strings.HasPrefix(line, "gateway:") {
			gateway = strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
	}
	if iface != "" && gateway != "" {
		return iface + " → " + gateway + " → 目标"
	}
	return ""
}
