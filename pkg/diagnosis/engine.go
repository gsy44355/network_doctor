package diagnosis

import (
	"fmt"
	"strings"

	"github.com/network-doctor/network-doctor/pkg/probe"
)

func joinIPs(ips []string) string {
	return strings.Join(ips, ", ")
}

type Diagnosis struct {
	Reachable  bool     `json:"reachable"`
	Summary    string   `json:"diagnosis"`
	Suggestion string   `json:"suggestion,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

func Diagnose(results map[string]*probe.ProbeResult) *Diagnosis {
	d := &Diagnosis{Reachable: true}

	if sys, ok := results["system"]; ok && sys.System != nil {
		if sys.System.TUNName != "" {
			d.Warnings = append(d.Warnings, fmt.Sprintf("请求经过 TUN 设备 (%s)", sys.System.TUN))
		}
		if sys.System.Proxy != "" {
			d.Warnings = append(d.Warnings, fmt.Sprintf("当前请求通过系统代理 (%s) 转发", sys.System.Proxy))
		}
	}

	// Clash proxy diagnosis
	if clash, ok := results["clash"]; ok && clash.Clash != nil {
		cd := clash.Clash
		if cd.Available {
			if !cd.DNSSuccess && cd.DNSError != "" {
				d.Warnings = append(d.Warnings, fmt.Sprintf("代理侧 DNS 解析失败: %s，目标可能通过代理不可达", cd.DNSError))
			} else if cd.DNSSuccess && len(cd.RealIPs) > 0 {
				d.Warnings = append(d.Warnings, fmt.Sprintf("代理侧 DNS 解析成功，真实 IP: %s", joinIPs(cd.RealIPs)))
			}
		} else if sys, ok := results["system"]; ok && sys.System != nil && sys.System.TUNName != "" {
			d.Warnings = append(d.Warnings, "检测到 TUN 设备但无法连接代理 API，诊断结果可能不准确")
		}
	}

	if dns, ok := results["dns"]; ok {
		if dns.Status == probe.StatusError {
			d.Reachable = false
			d.Summary = "域名解析失败，检查 DNS 配置或域名是否正确"
			d.Suggestion = "确认域名拼写正确，或尝试更换 DNS 服务器"
			return d
		}
		if dns.DNS != nil {
			if dns.DNS.FakeIP {
				d.Warnings = append(d.Warnings, "DNS 返回 Fake IP (198.18.x.x)，DNS 被代理接管")
			}
			if dns.DNS.InternalDomain {
				d.Warnings = append(d.Warnings, "内部域名，仅在当前 DNS 可解析")
			} else if dns.DNS.Consistent != nil && !*dns.DNS.Consistent {
				d.Warnings = append(d.Warnings, "DNS 解析结果与公共 DNS 不一致，可能存在 DNS 劫持")
			}
		}
	}

	if conn, ok := results["conn"]; ok && conn.Status == probe.StatusError {
		d.Reachable = false
		if conn.Conn != nil {
			port := conn.Conn.Port
			switch conn.Conn.ErrorType {
			case "timeout":
				d.Summary = fmt.Sprintf("TCP:%d 连接超时，可能被防火墙拦截", port)
				d.Suggestion = fmt.Sprintf("检查安全组/防火墙是否开放 %d 端口", port)
			case "refused":
				d.Summary = fmt.Sprintf("TCP:%d 连接被拒绝，目标服务未启动", port)
				d.Suggestion = "确认目标服务是否正在运行"
			case "unreachable":
				d.Summary = fmt.Sprintf("TCP:%d 主机不可达", port)
				d.Suggestion = "检查网络路由或目标主机是否在线"
			default:
				d.Summary = fmt.Sprintf("TCP:%d 连接失败", port)
			}
		} else {
			d.Summary = "TCP 连接失败"
		}
		return d
	}

	if tlsResult, ok := results["tls"]; ok {
		if tlsResult.Status == probe.StatusError {
			d.Reachable = false
			d.Summary = "TLS 握手失败"
			if tlsResult.TLS != nil && tlsResult.TLS.MITM {
				d.Summary = "检测到代理证书，可能被中间人劫持"
				d.Suggestion = "检查是否有企业代理或安全软件拦截 HTTPS 流量"
			} else if tlsResult.TLS != nil && !tlsResult.TLS.SNIMatch {
				d.Summary = "SNI 与证书不匹配"
				d.Suggestion = "目标可能配置了错误的证书"
			}
			return d
		}
		if tlsResult.Status == probe.StatusWarning && tlsResult.TLS != nil {
			if tlsResult.TLS.MITM {
				d.Warnings = append(d.Warnings, "检测到代理证书: "+tlsResult.TLS.MITMDetail)
			}
			if !tlsResult.TLS.SNIMatch {
				d.Warnings = append(d.Warnings, "SNI 与证书不匹配")
			}
		}
	}

	if proto, ok := results["protocol"]; ok {
		if proto.Status == probe.StatusError {
			d.Reachable = false
			d.Summary = "TCP 连通但协议握手失败，可能被代理拦截或服务异常"
			d.Suggestion = "检查目标服务是否正常运行，或是否有代理/防火墙拦截应用层协议"
			return d
		}
		if proto.Protocol != nil && proto.Protocol.AuthRequired {
			d.Warnings = append(d.Warnings, "协议可达，服务端要求认证（这不是网络问题）")
		}
	}

	if d.Reachable {
		d.Summary = "目标可达"
	}

	return d
}
