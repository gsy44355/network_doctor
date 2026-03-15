package probe

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"
)

var mitmCAOrgs = []string{
	"Zscaler", "Blue Coat", "Symantec", "Fortinet",
	"Palo Alto Networks", "Check Point", "Barracuda",
	"Cisco Umbrella", "Sophos", "McAfee",
	"Sangfor", "Huawei", "Netskope", "Forcepoint",
}

type TLSProbe struct {
	Verbose bool
}

func (p *TLSProbe) Name() string { return "tls" }

func (p *TLSProbe) Run(ctx context.Context, target *Target, prev map[string]*ProbeResult) *ProbeResult {
	if !target.NeedsTLS() {
		return NewResult("tls", StatusSkipped, "非 TLS 协议，跳过")
	}

	if conn, ok := prev["conn"]; ok && conn.Status == StatusError {
		return NewResult("tls", StatusSkipped, "跳过 (TCP 不通)")
	}

	addr := fmt.Sprintf("%s:%d", target.IP, target.Port)
	if target.IP == "" {
		addr = fmt.Sprintf("%s:%d", target.Host, target.Port)
	}

	serverName := target.Host
	if serverName == "" {
		serverName = target.IP
	}

	start := time.Now()
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Timeout = time.Until(deadline)
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
	})

	elapsed := time.Since(start)

	if err != nil {
		result := NewResult("tls", StatusError, fmt.Sprintf("TLS 握手失败: %v", err))
		result.SetDuration(elapsed)
		return result
	}
	defer conn.Close()

	state := conn.ConnectionState()
	details := &TLSDetails{}
	details.Version = tlsVersionString(state.Version)

	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		if len(cert.Issuer.Organization) > 0 {
			details.Issuer = cert.Issuer.Organization[0]
		} else {
			details.Issuer = cert.Issuer.CommonName
		}
		details.SNIMatch = verifySNI(cert, serverName)

		mitm, mitmDetail := detectMITM(cert)
		details.MITM = mitm
		details.MITMDetail = mitmDetail

		if p.Verbose {
			details.NotBefore = cert.NotBefore.Format("2006-01-02")
			details.NotAfter = cert.NotAfter.Format("2006-01-02")
			details.DaysLeft = int(time.Until(cert.NotAfter).Hours() / 24)
			details.SHA256 = fmt.Sprintf("%x", sha256.Sum256(cert.Raw))
			for _, c := range state.PeerCertificates {
				name := c.Subject.CommonName
				if len(c.Subject.Organization) > 0 {
					name = c.Subject.Organization[0]
				}
				details.Chain = append(details.Chain, name)
			}
		}
	}

	result := NewResult("tls", StatusOK, "")
	result.SetDuration(elapsed)
	result.TLS = details

	if details.MITM {
		result.Status = StatusWarning
	}
	if !details.SNIMatch {
		result.Status = StatusWarning
	}

	var parts []string
	parts = append(parts, details.Version)
	if details.SNIMatch {
		parts = append(parts, "SNI: ✅")
	} else {
		parts = append(parts, "SNI: ❌")
	}
	parts = append(parts, "颁发者: "+details.Issuer)
	if details.MITM {
		parts = append(parts, "中间人: ⚠️ "+details.MITMDetail)
	} else {
		parts = append(parts, "中间人: ✅")
	}
	result.Message = strings.Join(parts, " | ")

	return result
}

func verifySNI(cert *x509.Certificate, serverName string) bool {
	return cert.VerifyHostname(serverName) == nil
}

func detectMITM(cert *x509.Certificate) (bool, string) {
	// Self-signed: Issuer and Subject are identical
	if cert.Issuer.String() == cert.Subject.String() {
		return true, "自签名证书"
	}

	// Check known enterprise proxy CAs
	issuerOrg := ""
	if len(cert.Issuer.Organization) > 0 {
		issuerOrg = cert.Issuer.Organization[0]
	}
	for _, ca := range mitmCAOrgs {
		if strings.Contains(strings.ToLower(issuerOrg), strings.ToLower(ca)) {
			return true, "企业代理证书 (" + ca + ")"
		}
	}

	// Check if cert is issued by a non-public CA (not in system trust store)
	opts := x509.VerifyOptions{DNSName: ""}
	if _, err := cert.Verify(opts); err != nil {
		if _, ok := err.(x509.UnknownAuthorityError); ok {
			return true, "非公共 CA 签发"
		}
	}

	return false, ""
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "v1.0"
	case tls.VersionTLS11:
		return "v1.1"
	case tls.VersionTLS12:
		return "v1.2"
	case tls.VersionTLS13:
		return "v1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}
