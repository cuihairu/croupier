package main

import "github.com/spf13/cobra"

var configFile string

var rootCmd = &cobra.Command{
	Use:     "server",
	Long:    "croupier server",
	Version: "0.1.0",
	Run: func(cmd *cobra.Command, args []string) {
		app := newServerApplication()
		app.Run()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "conf", "", "", "config file path")
}
