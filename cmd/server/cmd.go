package main

import (
	"github.com/chuihairu/croupier/internal"
	"github.com/chuihairu/croupier/internal/version"
	"github.com/spf13/cobra"
)

var configFile string
var debug bool
var genConfig bool
var rootCmd = &cobra.Command{
	Use:     "server",
	Long:    "croupier server",
	Version: version.GetVersion(),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := internal.ServerApplicationInstance()
		if genConfig {
			return app.SaveConfig(configFile)
		}
		err := app.LoadConfig(configFile, debug)
		if err != nil {
			return err
		}
		app.Run()
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config/config.yaml", "config file")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "debug mode")
	rootCmd.PersistentFlags().BoolVarP(&genConfig, "generateConfig", "g", false, "generate config")
}
