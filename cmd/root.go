package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/network-doctor/network-doctor/pkg/config"
	"github.com/network-doctor/network-doctor/pkg/diagnosis"
	"github.com/network-doctor/network-doctor/pkg/output"
	"github.com/network-doctor/network-doctor/pkg/probe"
	"github.com/network-doctor/network-doctor/pkg/target"
)

var (
	flagJSON        bool
	flagVerbose     bool
	flagNoColor     bool
	flagTimeout     string
	flagFile        string
	flagClashAPI    string
	flagClashSecret string
	flagConcurrency int
)

type targetResult struct {
	target  *probe.Target
	results []*probe.ProbeResult
	diag    *diagnosis.Diagnosis
}

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
	rootCmd.Flags().StringVar(&flagClashAPI, "clash-api", "", "Clash External Controller 地址 (如 127.0.0.1:9090)")
	rootCmd.Flags().StringVar(&flagClashSecret, "clash-secret", "", "Clash API 认证密钥")
	rootCmd.Flags().IntVarP(&flagConcurrency, "concurrency", "c", 5, "并发检测数量")
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

	cfg := config.Load()
	exitCode := 0

	// 并行探测所有目标
	allResults := make([]targetResult, len(targets))
	if len(targets) > 1 {
		sem := make(chan struct{}, flagConcurrency)
		var wg sync.WaitGroup
		for i, t := range targets {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, tgt *probe.Target) {
				defer wg.Done()
				defer func() { <-sem }()
				results, diag := runProbes(tgt, timeout, cfg)
				allResults[idx] = targetResult{target: tgt, results: results, diag: diag}
			}(i, t)
		}
		wg.Wait()
	} else {
		results, diag := runProbes(targets[0], timeout, cfg)
		allResults[0] = targetResult{target: targets[0], results: results, diag: diag}
	}

	// 输出结果
	if flagJSON && len(targets) > 1 {
		var batch []output.JSONOutput
		for _, tr := range allResults {
			probeMap := make(map[string]*probe.ProbeResult)
			for _, r := range tr.results {
				r.FinalizeStatus()
				probeMap[r.Name] = r
			}
			batch = append(batch, output.JSONOutput{
				Target:     tr.target.Raw,
				Reachable:  tr.diag.Reachable,
				Probes:     probeMap,
				Diagnosis:  tr.diag.Summary,
				Suggestion: tr.diag.Suggestion,
				Warnings:   tr.diag.Warnings,
			})
			if !tr.diag.Reachable {
				exitCode = 1
			}
		}
		renderer := &output.JSONRenderer{}
		if err := renderer.RenderBatch(os.Stdout, batch); err != nil {
			fmt.Fprintf(os.Stderr, "JSON 输出错误: %v\n", err)
			os.Exit(2)
		}
	} else {
		for i, tr := range allResults {
			if len(targets) > 1 {
				if i > 0 {
					fmt.Println()
				}
				fmt.Fprintf(os.Stdout, "──── %s ────\n", tr.target.Raw)
			}
			if flagJSON {
				renderer := &output.JSONRenderer{}
				if err := renderer.Render(os.Stdout, tr.target.Raw, tr.results, tr.diag, flagVerbose); err != nil {
					fmt.Fprintf(os.Stderr, "JSON 输出错误: %v\n", err)
					os.Exit(2)
				}
			} else {
				renderer := &output.TextRenderer{NoColor: flagNoColor}
				renderer.Render(os.Stdout, tr.target.Raw, tr.results, tr.diag, flagVerbose)
			}
			if !tr.diag.Reachable {
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
	return nil
}

func runProbes(t *probe.Target, timeout time.Duration, cfg *config.Config) ([]*probe.ProbeResult, *diagnosis.Diagnosis) {
	clashAPI := flagClashAPI
	clashSecret := flagClashSecret
	if clashAPI == "" {
		clashAPI = cfg.ClashAPI
	}
	if clashSecret == "" {
		clashSecret = cfg.ClashSecret
	}

	probes := []probe.Probe{
		&probe.SystemProbe{},
		&probe.ClashProbe{APIAddr: clashAPI, Secret: clashSecret},
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
