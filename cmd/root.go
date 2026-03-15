package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	fmt.Println("network-doctor: not yet implemented")
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}
