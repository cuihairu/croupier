// Command dev-seed 铺一套有代表性的本地开发/测试数据，让每个页面「有东西
// 可看可筛」：多游戏/多环境、各类角色用户、工单+FAQ+分类、函数目录+页面+
// 配置版本、bugs↔工单关联、调度任务、公告、站内消息，以及一组边界/异常
// 数据（分页阈值、字段边界、悬空引用/孤儿、时间边界），见 seed_edges.go。
//
// 直接写数据库（无单条创建 API 的实体、REST 会拒绝的孤儿/悬空引用恰是
// 要造的边界态），不要求 server 在跑。模式参照 e2e-function-seed：
// flag + 幂等（自然键 FirstOrCreate / 哨兵跳过），可重复执行。
//
// 目标库：-dsn 透传 internal/db.Open（sqlite 文件 / postgres URL）。
// 默认 data/croupier.db，建表走 model.AutoMigrate（单库模式：全部表同库）。
// 已有数据一律保留——本命令只增不删不改。
//
// 用法：
//
//	go run ./examples/cmd/dev-seed                       # data/croupier.db
//	go run ./examples/cmd/dev-seed -dsn test-data/croupier.db
//	go run ./examples/cmd/dev-seed -heavy                # 追加万级分页数据
//
// 预置账号：seed-admin / seed-gm / seed-ops，密码均为 seed-Admin123。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cuihairu/croupier/internal/db"
	"github.com/cuihairu/croupier/internal/model"
)

// exit 是 os.Exit 的测试注入点（同 e2e-function-seed）。
var exit = os.Exit

func main() {
	exit(runMain(os.Args[0], os.Args[1:], os.Stderr))
}

// runMain 解析 flag 并执行种子写入，返回进程退出码：
// 0=成功，1=运行期失败（诊断已写 output），2=flag 解析错误（-h 为 0）。
func runMain(prog string, args []string, output io.Writer) int {
	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(output)
	dsn := fs.String("dsn", "", "数据库 DSN（sqlite 文件路径或 postgres:// URL，空=data/croupier.db）")
	heavy := fs.Bool("heavy", false, "追加万级分页数据（1 万玩家 + 1 万工单，较慢）")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	gdb, err := db.Open(*dsn)
	if err != nil {
		fmt.Fprintf(output, "dev-seed: 打开数据库失败: %v\n", err)
		return 1
	}
	// 单库模式建表（dev/CI 同路径）；postgres 走 AutoMigrate 的自愈循环。
	if err := model.AutoMigrate(gdb); err != nil {
		fmt.Fprintf(output, "dev-seed: 建表失败: %v\n", err)
		return 1
	}

	if err := seedAll(gdb, *heavy); err != nil {
		fmt.Fprintf(output, "dev-seed: %v\n", err)
		return 1
	}
	fmt.Fprintln(output, "dev-seed: 完成（幂等，可重复执行）")
	return 0
}
