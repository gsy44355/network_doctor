package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/network-doctor/network-doctor/pkg/diagnosis"
	"github.com/network-doctor/network-doctor/pkg/output"
	"github.com/network-doctor/network-doctor/pkg/probe"
	"github.com/network-doctor/network-doctor/pkg/target"
)

var (
	flagJSON    bool
	flagVerbose bool
	flagNoColor bool
	flagTimeout string
	flagFile    string
)

var rootCmd = &cobra.Command{
	Use:   "network-doctor <target>",
	Short: "网络可达性诊断工具",
	Long:  "检测本机到目标地址的连通性，定位不可达的具体原因",
	Args:  cobra.ArbitraryArgs,
	RunE:  run,
}

func init() {
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "JSON 格式输出")
	rootCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "显示详细信息")
	rootCmd.Flags().BoolVar(&flagNoColor, "no-color", false, "禁用彩色输出")
	rootCmd.Flags().StringVar(&flagTimeout, "timeout", "10s", "每个探针的超时时间")
	rootCmd.Flags().StringVarP(&flagFile, "file", "f", "", "从文件读取目标列表")
}

func run(cmd *cobra.Command, args []string) error {
	if flagFile == "" && len(args) == 0 {
		return fmt.Errorf("请提供目标地址或使用 -f 指定目标文件")
	}

	timeout, err := time.ParseDuration(flagTimeout)
	if err != nil {
		return fmt.Errorf("无效的超时时间: %s", flagTimeout)
	}

	if flagNoColor {
		color.NoColor = true
	}

	var targets []*probe.Target
	if flagFile != "" {
		content, err := os.ReadFile(flagFile)
		if err != nil {
			return fmt.Errorf("无法读取目标文件: %v", err)
		}
		targets, err = target.ParseLines(string(content))
		if err != nil {
			return err
		}
	} else {
		for _, arg := range args {
			t, err := target.Parse(arg)
			if err != nil {
				return err
			}
			targets = append(targets, t)
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("未找到有效的目标地址")
	}

	exitCode := 0

	if flagJSON && len(targets) > 1 {
		var batch []output.JSONOutput
		for _, t := range targets {
			results, diag := runProbes(t, timeout)
			probeMap := make(map[string]*probe.ProbeResult)
			for _, r := range results {
				r.FinalizeStatus()
				probeMap[r.Name] = r
			}
			batch = append(batch, output.JSONOutput{
				Target:     t.Raw,
				Reachable:  diag.Reachable,
				Probes:     probeMap,
				Diagnosis:  diag.Summary,
				Suggestion: diag.Suggestion,
				Warnings:   diag.Warnings,
			})
			if !diag.Reachable {
				exitCode = 1
			}
		}
		renderer := &output.JSONRenderer{}
		if err := renderer.RenderBatch(os.Stdout, batch); err != nil {
			fmt.Fprintf(os.Stderr, "JSON 输出错误: %v\n", err)
			os.Exit(2)
		}
	} else {
		for i, t := range targets {
			if i > 0 {
				fmt.Println()
			}
			results, diag := runProbes(t, timeout)
			if flagJSON {
				renderer := &output.JSONRenderer{}
				if err := renderer.Render(os.Stdout, t.Raw, results, diag, flagVerbose); err != nil {
					fmt.Fprintf(os.Stderr, "JSON 输出错误: %v\n", err)
					os.Exit(2)
				}
			} else {
				renderer := &output.TextRenderer{NoColor: flagNoColor}
				renderer.Render(os.Stdout, t.Raw, results, diag, flagVerbose)
			}
			if !diag.Reachable {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
	return nil
}

func runProbes(t *probe.Target, timeout time.Duration) ([]*probe.ProbeResult, *diagnosis.Diagnosis) {
	probes := []probe.Probe{
		&probe.SystemProbe{},
		&probe.DNSProbe{},
		&probe.ConnProbe{},
		&probe.TLSProbe{Verbose: flagVerbose},
		&probe.ProtocolProbe{},
	}

	prev := make(map[string]*probe.ProbeResult)
	var results []*probe.ProbeResult

	for _, p := range probes {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		result := p.Run(ctx, t, prev)
		cancel()

		prev[result.Name] = result
		results = append(results, result)
	}

	diag := diagnosis.Diagnose(prev)
	return results, diag
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}
