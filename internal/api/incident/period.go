package incident

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// 报表周期档位（docs/design/incident-reports.md §5.1）。
const (
	PeriodWeek    = "week"
	PeriodMonth   = "month"
	PeriodQuarter = "quarter"
	PeriodYear    = "year"
)

var (
	weekKeyRe    = regexp.MustCompile(`^(\d{4})-W(\d{1,2})$`)
	monthKeyRe   = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
	quarterKeyRe = regexp.MustCompile(`^(\d{4})-Q([1-4])$`)
	yearKeyRe    = regexp.MustCompile(`^(\d{4})$`)
)

// resolvedPeriod 是一个已解析的报表周期：本期 [Start,End)、环比上一同档
// 周期、同比去年同档周期（年档无同比 → YoyMissing）。边界在 server 本地
// 时区计算（§5.1），库内时间戳一律 UTC 由调用方按需转换。
type resolvedPeriod struct {
	Type      string
	Key       string
	Start     time.Time
	End       time.Time
	PrevStart time.Time
	PrevEnd   time.Time
	YoyStart  time.Time
	YoyEnd    time.Time
	// YoyMissing：年档恒真；周档去年无同 ISO 周号（如 W53）亦真。
	// 其余档案由数据 epoch 判定（report.go），不在此预处理。
	YoyMissing bool
	// PrevMissing：环比周期整体早于数据起点时由调用方判定，此处不判。
}

// resolvePeriod 把档位 + 周期标识解析为三组区间。periodKey 为空 = 当前
// 周期（含 now）。
func resolvePeriod(periodType, periodKey string, now time.Time) (*resolvedPeriod, error) {
	switch periodType {
	case PeriodWeek:
		return resolveWeekPeriod(periodKey, now)
	case PeriodMonth:
		return resolveMonthPeriod(periodKey, now)
	case PeriodQuarter:
		return resolveQuarterPeriod(periodKey, now)
	case PeriodYear:
		return resolveYearPeriod(periodKey, now)
	default:
		return nil, errorx.NewBadRequest("无效的报表档位: " + periodType + "（支持 week/month/quarter/year）")
	}
}

func resolveWeekPeriod(key string, now time.Time) (*resolvedPeriod, error) {
	start, wkYear, wkNum, err := weekStartOf(key, now)
	if err != nil {
		return nil, err
	}
	end := start.AddDate(0, 0, 7)
	p := &resolvedPeriod{
		Type: PeriodWeek, Key: fmt.Sprintf("%d-W%02d", wkYear, wkNum),
		Start: start, End: end,
		PrevStart: start.AddDate(0, 0, -7), PrevEnd: start,
	}
	// 同比 = 去年同 ISO 周号；去年不足该周号（W53 对 52 周年）→ missing。
	yoy, err := isoWeekMonday(wkYear-1, wkNum)
	if err != nil {
		p.YoyMissing = true
	} else {
		p.YoyStart, p.YoyEnd = yoy, yoy.AddDate(0, 0, 7)
	}
	return p, nil
}

func weekStartOf(key string, now time.Time) (time.Time, int, int, error) {
	if key == "" {
		y, w := now.ISOWeek()
		m, err := isoWeekMonday(y, w)
		return m, y, w, err
	}
	m := weekKeyRe.FindStringSubmatch(key)
	if m == nil {
		return time.Time{}, 0, 0, errorx.NewBadRequest("周档标识须为 YYYY-Www 形态（如 2026-W41）")
	}
	year, _ := strconv.Atoi(m[1])
	week, _ := strconv.Atoi(m[2])
	if week < 1 || week > 53 {
		return time.Time{}, 0, 0, errorx.NewBadRequest("ISO 周号须在 1-53 内")
	}
	start, err := isoWeekMonday(year, week)
	return start, year, week, err
}

// isoWeekMonday 求 ISO 年第 week 周的周一 00:00（本地时区）。1 月 4 日恒在
// W1，以其所在周的周一为锚。构造结果经 ISOWeek 回读校验（52 周年请求 W53
// 等非法组合报错）。
func isoWeekMonday(year, week int) (time.Time, error) {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.Local)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := jan4.AddDate(0, 0, 1-wd).AddDate(0, 0, (week-1)*7)
	gy, gw := monday.ISOWeek()
	if gy != year || gw != week {
		return time.Time{}, errorx.NewBadRequest(
			fmt.Sprintf("%d 年无 ISO 第 %d 周", year, week))
	}
	return monday, nil
}

func resolveMonthPeriod(key string, now time.Time) (*resolvedPeriod, error) {
	var start time.Time
	if key == "" {
		y, m, _ := now.Date()
		start = time.Date(y, m, 1, 0, 0, 0, 0, time.Local)
	} else {
		mm := monthKeyRe.FindStringSubmatch(key)
		if mm == nil || len(mm[2]) != 2 {
			return nil, errorx.NewBadRequest("月档标识须为 YYYY-MM 形态（如 2026-10）")
		}
		y, _ := strconv.Atoi(mm[1])
		mo, _ := strconv.Atoi(mm[2])
		if mo < 1 || mo > 12 {
			return nil, errorx.NewBadRequest("月份须在 01-12 内")
		}
		start = time.Date(y, time.Month(mo), 1, 0, 0, 0, 0, time.Local)
	}
	end := start.AddDate(0, 1, 0)
	return &resolvedPeriod{
		Type: PeriodMonth, Key: start.Format("2006-01"),
		Start: start, End: end,
		PrevStart: start.AddDate(0, -1, 0), PrevEnd: start,
		YoyStart: start.AddDate(-1, 0, 0), YoyEnd: end.AddDate(-1, 0, 0),
	}, nil
}

func resolveQuarterPeriod(key string, now time.Time) (*resolvedPeriod, error) {
	var start time.Time
	if key == "" {
		y, m, _ := now.Date()
		start = time.Date(y, time.Month((int(m)-1)/3*3+1), 1, 0, 0, 0, 0, time.Local)
	} else {
		qq := quarterKeyRe.FindStringSubmatch(key)
		if qq == nil {
			return nil, errorx.NewBadRequest("季档标识须为 YYYY-Qn 形态（如 2026-Q4）")
		}
		y, _ := strconv.Atoi(qq[1])
		q, _ := strconv.Atoi(qq[2])
		start = time.Date(y, time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.Local)
	}
	end := start.AddDate(0, 3, 0)
	return &resolvedPeriod{
		Type: PeriodQuarter, Key: start.Format("2006") + "-Q" + strconv.Itoa((int(start.Month())-1)/3+1),
		Start: start, End: end,
		PrevStart: start.AddDate(0, -3, 0), PrevEnd: end.AddDate(0, -3, 0),
		YoyStart: start.AddDate(-1, 0, 0), YoyEnd: end.AddDate(-1, 0, 0),
	}, nil
}

func resolveYearPeriod(key string, now time.Time) (*resolvedPeriod, error) {
	var start time.Time
	if key == "" {
		start = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.Local)
	} else {
		ym := yearKeyRe.FindStringSubmatch(key)
		if ym == nil {
			return nil, errorx.NewBadRequest("年档标识须为 YYYY 形态（如 2026）")
		}
		y, _ := strconv.Atoi(ym[1])
		start = time.Date(y, time.January, 1, 0, 0, 0, 0, time.Local)
	}
	end := start.AddDate(1, 0, 0)
	return &resolvedPeriod{
		Type: PeriodYear, Key: start.Format("2006"),
		Start: start, End: end,
		PrevStart: start.AddDate(-1, 0, 0), PrevEnd: end.AddDate(-1, 0, 0),
		YoyMissing: true, // 年档无同比列（§5.1）
	}, nil
}
