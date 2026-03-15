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
	var httpProxy, httpsProxy string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HTTPEnable : 1") {
			httpEnabled = true
		}
		if strings.HasPrefix(line, "HTTPSEnable : 1") {
			httpsEnabled = true
		}
		if strings.HasPrefix(line, "HTTPProxy :") {
			httpProxy = strings.TrimPrefix(line, "HTTPProxy : ")
		}
		if strings.HasPrefix(line, "HTTPSProxy :") {
			httpsProxy = strings.TrimPrefix(line, "HTTPSProxy : ")
		}
	}

	if scheme == "https" && httpsEnabled && httpsProxy != "" {
		return httpsProxy
	}
	if httpEnabled && httpProxy != "" {
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
