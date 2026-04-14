package probe

import (
	"context"
	"fmt"
	"time"
)

type Status int

const (
	StatusOK Status = iota
	StatusWarning
	StatusError
	StatusSkipped
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarning:
		return "warning"
	case StatusError:
		return "error"
	case StatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

type ProbeResult struct {
	Name      string        `json:"name"`
	Status    Status        `json:"-"`
	StatusStr string        `json:"status"`
	Duration  time.Duration `json:"-"`
	DurationMs int64        `json:"duration_ms,omitempty"`
	Message   string        `json:"message,omitempty"`
	System    *SystemDetails   `json:"system,omitempty"`
	DNS       *DNSDetails      `json:"dns,omitempty"`
	Clash     *ClashDetails    `json:"clash,omitempty"`
	Conn      *ConnDetails     `json:"conn,omitempty"`
	TLS       *TLSDetails      `json:"tls,omitempty"`
	Protocol  *ProtocolDetails `json:"protocol,omitempty"`
}

func NewResult(name string, status Status, msg string) *ProbeResult {
	return &ProbeResult{Name: name, Status: status, StatusStr: status.String(), Message: msg}
}

func (r *ProbeResult) SetDuration(d time.Duration) {
	r.Duration = d
	r.DurationMs = d.Milliseconds()
}

// FinalizeStatus syncs StatusStr from Status. Call before JSON serialization.
func (r *ProbeResult) FinalizeStatus() {
	r.StatusStr = r.Status.String()
}

type SystemDetails struct {
	Proxy     string `json:"proxy"`
	TUN       string `json:"tun"`
	TUNName   string `json:"tun_name,omitempty"`
	Interface string `json:"interface"`
	Route     string `json:"route,omitempty"`
}

type DNSDetails struct {
	IPv4           []string `json:"ipv4"`
	IPv6           []string `json:"ipv6"`
	Server         string   `json:"server,omitempty"`
	Consistent     *bool    `json:"consistent,omitempty"`
	InternalDomain bool     `json:"internal_domain,omitempty"`
	PublicDNSResult string  `json:"public_dns_result,omitempty"`
	FakeIP         bool     `json:"fake_ip,omitempty"`
}

type ClashDetails struct {
	APIAddr    string   `json:"api_addr"`
	Available  bool     `json:"available"`
	Version    string   `json:"version,omitempty"`
	RealIPs    []string `json:"real_ips,omitempty"`
	DNSSuccess bool     `json:"dns_success"`
	DNSError   string   `json:"dns_error,omitempty"`
}

type ConnDetails struct {
	Port      int    `json:"port"`
	ErrorType string `json:"error_type,omitempty"`
}

type TLSDetails struct {
	Version    string   `json:"version"`
	SNIMatch   bool     `json:"sni_match"`
	Issuer     string   `json:"issuer"`
	MITM       bool     `json:"mitm"`
	MITMDetail string   `json:"mitm_detail,omitempty"`
	NotBefore  string   `json:"not_before,omitempty"`
	NotAfter   string   `json:"not_after,omitempty"`
	DaysLeft   int      `json:"days_left,omitempty"`
	Chain      []string `json:"chain,omitempty"`
	SHA256     string   `json:"sha256,omitempty"`
}

type ProtocolDetails struct {
	Type             string   `json:"type"`
	StatusCode       int      `json:"status_code,omitempty"`
	Version          string   `json:"version,omitempty"`
	Banner           string   `json:"banner,omitempty"`
	AuthRequired     bool     `json:"auth_required,omitempty"`
	ProxyRelayFailed bool     `json:"proxy_relay_failed,omitempty"`
	ProxyChain       []string `json:"proxy_chain,omitempty"`
}

type Probe interface {
	Name() string
	Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult
}

type Target struct {
	Raw    string `json:"raw"`
	Scheme string `json:"scheme"`
	Host   string `json:"host,omitempty"`
	IP     string `json:"ip,omitempty"`
	Port   int    `json:"port"`
	IsIP   bool   `json:"is_ip"`
}

func (t *Target) NeedsTLS() bool {
	return t.Scheme == "https"
}

func (t *Target) Address() string {
	host := t.Host
	if t.IsIP {
		host = t.IP
	}
	return fmt.Sprintf("%s:%d", host, t.Port)
}
