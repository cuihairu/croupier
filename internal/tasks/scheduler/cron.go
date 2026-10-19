package scheduler

import (
	"fmt"
	"strings"
	"time"

	process "github.com/cuihairu/croupier/core/process"
)

// CronSpec 是解析后的五字段 cron 表达式。
type CronSpec = process.CronSpec

// ParseCron 解析五字段 cron 表达式（分 时 日 月 周）。
// 支持 *、数字、区间 a-b、步进 */n 与 a-b/n、逗号列表。
func ParseCron(expr string) (*CronSpec, error) {
	spec, err := process.ParseCron(expr)
	if err != nil {
		return nil, translateCronError(expr, err)
	}
	return spec, nil
}

func translateCronError(expr string, err error) error {
	if len(expr) == 0 || len(expr) >= 3 && expr[0] == '*' && expr[1] == ' ' && expr[2] == '*' {
		return fmt.Errorf("cron 表达式需要 5 个字段（分 时 日 月 周）: %w", err)
	}
	replacer := strings.NewReplacer(
		"invalid range", "非法区间",
		"invalid value", "非法数值",
		"invalid step", "非法步进",
		"value outside range", "数值超出范围",
	)
	return fmt.Errorf("cron 表达式非法: %s", replacer.Replace(err.Error()))
}

// Slot 把触发时间对齐到分钟（幂等窗口键）。
func Slot(t time.Time) time.Time { return process.Slot(t) }
