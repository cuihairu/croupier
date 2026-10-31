package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/spf13/cobra"
)

// tokenCmd 签发平台 token 的父命令。
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "签发平台 token（管理员 JWT / 受限机器身份）",
	Long: `签发平台认证 token。

子命令 agent-sync 签发 ARCH-021 受限机器身份 token：绑定单个 Agent 与其
scope（gameId|env），只能用于扩展同步端点，不能当作管理员身份使用。`,
}

var (
	machineTokenAgentID string
	machineTokenGameID  string
	machineTokenEnv     string
	machineTokenTTL     time.Duration
)

// machineTokenCmd 签发 Agent 受限机器身份 token（ARCH-021）。
var machineTokenCmd = &cobra.Command{
	Use:   "agent-sync",
	Short: "签发 Agent 受限机器身份 token（扩展同步）",
	Long: `签发受限机器身份 token，供 Agent 扩展同步拉取器使用。

token 绑定单个 Agent（--agent-id）与其 scope（--game-id / --env），仅能访问
扩展同步端点（GET /api/v1/agents/<id>/extensions）。服务端在其它路径一律
403 拒绝，且不能当作管理员身份使用。

把输出写入 Agent 环境变量 CROUPIER_EXTENSION_SYNC_TOKEN 即可启用。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfigFile(cfgFile)
		if err != nil {
			return err
		}
		secret, err := jwtutil.ResolveSecret(c)
		if err != nil {
			return err
		}
		token, err := jwtutil.SignMachine(secret, machineTokenAgentID, machineTokenGameID, machineTokenEnv, machineTokenTTL, time.Now())
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", token)
		return nil
	},
}

func init() {
	machineTokenCmd.Flags().StringVar(&machineTokenAgentID, "agent-id", "", "Agent ID（必填，token 绑定的身份）")
	machineTokenCmd.Flags().StringVar(&machineTokenGameID, "game-id", "", "scope：game ID")
	machineTokenCmd.Flags().StringVar(&machineTokenEnv, "env", "", "scope：环境（如 prod）")
	machineTokenCmd.Flags().DurationVar(&machineTokenTTL, "ttl", 30*24*time.Hour, "token 有效期（0/负=默认 30 天）")
	_ = machineTokenCmd.MarkFlagRequired("agent-id")
	tokenCmd.AddCommand(machineTokenCmd)
	rootCmd.AddCommand(tokenCmd)
}
