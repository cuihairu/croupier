package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "为指定的 shell 生成自动完成脚本",
	Long: `为 bash、zsh、fish 或 powershell 生成自动完成脚本。
要在 shell 中启用完成功能：

Bash:
  $ source <(croupier-server completion bash)

  # 添加到 .bashrc 或 .bash_profile 中以便永久生效:
  $ echo "source <(croupier-server completion bash)" >> ~/.bashrc

Zsh:
  # 如果 shell 补全尚未启用，请先运行：
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # 加载完成脚本：
  $ source <(croupier-server completion zsh)

  # 添加到 .zshrc 中以便永久生效:
  $ echo "source <(croupier-server completion zsh)" >> ~/.zshrc

Fish:
  $ croupier-server completion fish | source

  # 添加到 config.fish 中以便永久生效:
  $ croupier-server completion fish > ~/.config/fish/completions/croupier-server.fish

PowerShell:
  PS> croupier-server completion powershell | Out-String | Invoke-Expression

  # 添加到 PowerShell profile 中以便永久生效:
  PS> croupier-server completion powershell >> $PROFILE`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		switch args[0] {
		case "bash":
			err = rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			err = rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			err = rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			err = rootCmd.GenPowerShellCompletion(os.Stdout)
		default:
			return fmt.Errorf("不支持的 shell: %s", args[0])
		}
		return err
	},
}
