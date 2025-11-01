package main

import (
    "fmt"
    "log"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    servercmd "github.com/cuihairu/croupier/internal/cli/servercmd"
    agentcmd "github.com/cuihairu/croupier/internal/cli/agentcmd"
    edgecmd "github.com/cuihairu/croupier/internal/cli/edgecmd"
    common "github.com/cuihairu/croupier/internal/cli/common"
)

func main() {
    root := &cobra.Command{Use: "croupier", Short: "Croupier unified CLI"}

    // Subcommands
    root.AddCommand(servercmd.New())
    root.AddCommand(agentcmd.New())
    root.AddCommand(edgecmd.New())

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
        v := viper.New()
        v.SetConfigFile(cfgFile)
        if err := v.ReadInConfig(); err != nil { return err }
        // Prepare snapshot helper
        snapshot := func(sub *viper.Viper, sect string) error {
            if sub == nil { return fmt.Errorf("section %s not found", sect) }
            // merge log subsection for snapshot
            common.MergeLogSection(sub)
            m := map[string]any{}
            for _, k := range sub.AllKeys() { m[k] = sub.Get(k) }
            // validate strictly
            var err error
            switch sect {
            case "server": err = common.ValidateServerConfig(sub, true)
            case "agent": err = common.ValidateAgentConfig(sub, true)
            }
            if err != nil { return err }
            // print pretty JSON
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(map[string]any{"section": sect, "effective": m})
        }
        switch section {
        case "server": return snapshot(v.Sub("server"), "server")
        case "agent": return snapshot(v.Sub("agent"), "agent")
        case "":
            if s := v.Sub("server"); s != nil { if err := snapshot(s, "server"); err == nil { return nil } }
            if a := v.Sub("agent"); a != nil { if err := snapshot(a, "agent"); err == nil { return nil } }
            return fmt.Errorf("no valid section found; specify --section")
        default:
            return fmt.Errorf("unknown section: %s", section)
        }
    }
    root.AddCommand(cfgTest)

    if err := root.Execute(); err != nil { log.Fatal(err) }
}
