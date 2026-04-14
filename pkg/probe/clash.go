package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type ClashProbe struct {
	APIAddr string
	Secret  string
}

func (p *ClashProbe) Name() string { return "clash" }

func (p *ClashProbe) Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult {
	// Skip if no TUN detected and no API address configured
	hasTUN := false
	if sys, ok := prev["system"]; ok && sys.System != nil && sys.System.TUNName != "" {
		hasTUN = true
	}
	if !hasTUN && p.APIAddr == "" {
		return NewResult("clash", StatusSkipped, "未检测到 TUN 设备，跳过代理检测")
	}

	start := time.Now()
	details := &ClashDetails{}

	// Discover API address
	apiAddr := p.discoverAPI(ctx)
	if apiAddr == "" {
		result := NewResult("clash", StatusSkipped, "未发现 Clash API")
		result.SetDuration(time.Since(start))
		result.Clash = details
		return result
	}
	details.APIAddr = apiAddr
	details.Available = true
	details.Secret = p.Secret

	// Get version
	details.Version = p.getVersion(ctx, apiAddr)

	// Query DNS through Clash
	if !target.IsIP {
		p.queryDNS(ctx, apiAddr, target.Host, details)
	}

	elapsed := time.Since(start)

	var msg string
	if details.DNSSuccess {
		msg = fmt.Sprintf("Clash (%s) 代理侧 DNS 解析成功: %s", details.Version, strings.Join(details.RealIPs, ", "))
	} else if details.DNSError != "" {
		msg = fmt.Sprintf("Clash (%s) 代理侧 DNS 解析失败: %s", details.Version, details.DNSError)
	} else {
		msg = fmt.Sprintf("Clash API 可用 (%s)", details.APIAddr)
	}

	status := StatusOK
	if !details.DNSSuccess && details.DNSError != "" {
		status = StatusWarning
	}

	result := NewResult("clash", status, msg)
	result.SetDuration(elapsed)
	result.Clash = details
	return result
}

func (p *ClashProbe) discoverAPI(ctx context.Context) string {
	if p.APIAddr != "" {
		if p.checkAPI(ctx, p.APIAddr) {
			return p.APIAddr
		}
		return ""
	}

	// Auto-discover common ports
	for _, addr := range []string{"127.0.0.1:9090", "127.0.0.1:9097"} {
		if p.checkAPI(ctx, addr) {
			return addr
		}
	}
	return ""
}

func (p *ClashProbe) checkAPI(ctx context.Context, addr string) bool {
	client := p.httpClient()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+addr+"/version", nil)
	if err != nil {
		return false
	}
	p.setAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (p *ClashProbe) getVersion(ctx context.Context, addr string) string {
	client := p.httpClient()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+addr+"/version", nil)
	if err != nil {
		return ""
	}
	p.setAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var v struct {
		Version string `json:"version"`
	}
	body, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(body, &v) == nil {
		return v.Version
	}
	return ""
}

func (p *ClashProbe) queryDNS(ctx context.Context, addr, host string, details *ClashDetails) {
	client := p.httpClient()
	url := fmt.Sprintf("http://%s/dns/query?name=%s&type=A", addr, host)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		details.DNSError = err.Error()
		return
	}
	p.setAuth(req)

	resp, err := client.Do(req)
	if err != nil {
		details.DNSError = err.Error()
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		details.DNSError = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		return
	}

	// Clash DNS response format: {"Status":0,"Answer":[{"data":"1.2.3.4",...}]}
	var dnsResp struct {
		Status int `json:"Status"`
		Answer []struct {
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		details.DNSError = "无法解析 Clash DNS 响应"
		return
	}

	if dnsResp.Status != 0 || len(dnsResp.Answer) == 0 {
		details.DNSError = fmt.Sprintf("DNS 查询失败 (RCODE=%d)", dnsResp.Status)
		return
	}

	details.DNSSuccess = true
	for _, ans := range dnsResp.Answer {
		if ip := net.ParseIP(ans.Data); ip != nil {
			details.RealIPs = append(details.RealIPs, ans.Data)
		}
	}
}

func (p *ClashProbe) httpClient() *http.Client {
	return &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		},
	}
}

func (p *ClashProbe) setAuth(req *http.Request) {
	if p.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+p.Secret)
	}
}
