package cmd

import (
	"fmt"
	"wx_channel/internal/config"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "打印版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load()
		fmt.Printf("wx_channel v%s\n", cfg.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
