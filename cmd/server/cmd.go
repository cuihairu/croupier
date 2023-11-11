package main

import (
	"github.com/chuihairu/croupier/internal/version"
	"github.com/spf13/cobra"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:     "server",
	Long:    "croupier server",
	Version: version.GetVersion(),
	Run: func(cmd *cobra.Command, args []string) {
		app := newServerApplication()
		app.Run()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "conf", "", "", "config file path")
}
