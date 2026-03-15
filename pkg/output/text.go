package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/network-doctor/network-doctor/pkg/diagnosis"
	"github.com/network-doctor/network-doctor/pkg/probe"
)

type TextRenderer struct {
	NoColor bool
}

func (r *TextRenderer) Render(w io.Writer, target string, results []*probe.ProbeResult, diag *diagnosis.Diagnosis, verbose bool) error {
	if r.NoColor {
		color.NoColor = true
	}

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	labelMap := map[string]string{
		"system":   "[系统]",
		"dns":      "[DNS] ",
		"conn":     "[连通]",
		"tls":      "[TLS] ",
		"protocol": "",
	}

	for _, result := range results {
		label := labelMap[result.Name]

		if result.Name == "protocol" && result.Protocol != nil {
			schemeLabel := strings.ToUpper(result.Protocol.Type)
			label = fmt.Sprintf("[%s]", schemeLabel)
		}
		if label == "" {
			label = fmt.Sprintf("[%s]", result.Name)
		}

		msgColor := green
		if result.Status == probe.StatusError {
			msgColor = red
		} else if result.Status == probe.StatusWarning || result.Status == probe.StatusSkipped {
			msgColor = yellow
		}

		fmt.Fprintf(w, "%s %s\n", cyan(label), msgColor(result.Message))

		if verbose {
			r.renderVerbose(w, result)
		}
	}

	fmt.Fprintln(w)
	if diag.Reachable {
		summary := green("✅") + " " + bold(diag.Summary)
		if len(diag.Warnings) > 0 {
			summary += " (" + diag.Warnings[0] + ")"
		}
		fmt.Fprintln(w, summary)
	} else {
		fmt.Fprintln(w, red("❌")+" "+bold("不可达: "+diag.Summary))
		if diag.Suggestion != "" {
			fmt.Fprintln(w, "   建议: "+diag.Suggestion)
		}
	}

	if len(diag.Warnings) > 1 {
		for _, warn := range diag.Warnings[1:] {
			fmt.Fprintln(w, yellow("⚠️")+"  "+warn)
		}
	}

	return nil
}

func (r *TextRenderer) renderVerbose(w io.Writer, result *probe.ProbeResult) {
	switch {
	case result.System != nil && result.System.Route != "":
		fmt.Fprintf(w, "       路由: %s\n", result.System.Route)
	case result.DNS != nil:
		if result.DNS.Server != "" {
			line := "       服务器: " + result.DNS.Server
			if result.DNS.PublicDNSResult != "" {
				line += " | 公共DNS: " + result.DNS.PublicDNSResult
			}
			fmt.Fprintln(w, line)
		}
	case result.TLS != nil:
		tls := result.TLS
		if tls.NotBefore != "" {
			fmt.Fprintf(w, "       有效期: %s ~ %s (剩余 %d 天)\n", tls.NotBefore, tls.NotAfter, tls.DaysLeft)
		}
		if len(tls.Chain) > 0 {
			fmt.Fprintf(w, "       证书链: %s\n", strings.Join(tls.Chain, " → "))
		}
		if tls.SHA256 != "" {
			fmt.Fprintf(w, "       指纹: SHA256:%s\n", tls.SHA256[:16]+"...")
		}
	}
}
