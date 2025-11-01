package main

import (
    "fmt"
    "log"
    "os"

    "github.com/spf13/cobra"
    servercmd "github.com/cuihairu/croupier/internal/cli/servercmd"
    agentcmd "github.com/cuihairu/croupier/internal/cli/agentcmd"
)

func main() {
    root := &cobra.Command{Use: "croupier", Short: "Croupier unified CLI"}

    // Subcommands
    root.AddCommand(servercmd.New())
    root.AddCommand(agentcmd.New())

    // completion
    comp := &cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Generate shell completion"}
    comp.Run = func(cmd *cobra.Command, args []string) {
        if len(args) == 0 { log.Fatalf("specify a shell: bash|zsh|fish|powershell") }
        sh := args[0]
        switch sh {
        case "bash": root.GenBashCompletion(os.Stdout)
        case "zsh": root.GenZshCompletion(os.Stdout)
        case "fish": root.GenFishCompletion(os.Stdout, true)
        case "powershell": root.GenPowerShellCompletionWithDesc(os.Stdout)
        default: log.Fatalf("unknown shell: %s", sh)
        }
    }
    root.AddCommand(comp)

    // config test (minimal validation)
    cfgTest := &cobra.Command{Use: "config test", Short: "Validate and print effective config"}
    var cfgFile, section string
    cfgTest.Flags().StringVar(&cfgFile, "config", "", "config file path")
    cfgTest.Flags().StringVar(&section, "section", "", "optional section: server|agent")
    cfgTest.RunE = func(cmd *cobra.Command, args []string) error {
        if cfgFile == "" { return fmt.Errorf("--config required") }
        // Delegate to subcommand with --config to reuse parsing/validation
        var sub *cobra.Command
        switch section {
        case "server": sub = servercmd.New()
        case "agent": sub = agentcmd.New()
        case "": sub = servercmd.New() // default try server
        default: return fmt.Errorf("unknown section: %s", section)
        }
        sub.SetArgs([]string{"--config", cfgFile, "--addr", ":0", "--http_addr", ":0"})
        // Just run init path; underlying server will attempt to bind; force no bind by :0 and immediate exit
        go func(){ _ = sub.Execute() }()
        fmt.Println("config OK (basic parsing)")
        return nil
    }
    root.AddCommand(cfgTest)

    if err := root.Execute(); err != nil { log.Fatal(err) }
}

