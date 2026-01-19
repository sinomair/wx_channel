package cmd

import (
	"fmt"
	"os"
	"wx_channel/internal/app"
	"wx_channel/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	port    int
	dev     string
	Version = "1.0.0" // Should be injected or loaded from config
)

var rootCmd = &cobra.Command{
	Use:   "wx_channel",
	Short: "WeChat Channel Video Downloader",
	Long:  `A tool to download videos from WeChat Channels with auto-decryption and de-duplication.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load()
		// Apply flags to config
		if port != 0 {
			cfg.SetPort(port)
		}
		app.Run(cfg)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Persist flags
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 0, "Proxy server network port")
	rootCmd.PersistentFlags().StringVarP(&dev, "dev", "d", "", "Proxy server network device")

	// Bind flags to viper
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("dev", rootCmd.PersistentFlags().Lookup("dev"))
}

func initConfig() {
	// 加载配置
	config.Load()
}
