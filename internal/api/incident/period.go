package incident

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// Report period tiers (docs/design/incident-reports.md section 5.1).
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

// resolvedPeriod is a resolved report period: the current period [Start,End),
// the period-over-period window of the same tier, and the year-over-year window
// of the same tier last year (the year tier has no YoY -> YoyMissing).
// Boundaries are computed in the server's local time zone (section 5.1); the
// timestamps stored in the database are always UTC and are converted by the
// caller as needed.
type resolvedPeriod struct {
	Type      string
	Key       string
	Start     time.Time
	End       time.Time
	PrevStart time.Time
	PrevEnd   time.Time
	YoyStart  time.Time
	YoyEnd    time.Time
	// YoyMissing: always true for the year tier; also true for the week tier
	// when last year has no such ISO week number (e.g. W53).
	// For the remaining tiers it is decided from the data epoch (report.go) and
	// is not precomputed here.
	YoyMissing bool
	// PrevMissing: decided by the caller when the PoP window as a whole
	// predates the start of the data; not decided here.
}

// resolvePeriod resolves a tier + period key into three interval groups.
// An empty periodKey means the current period (including now).
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
	// YoY = the same ISO week number last year; if last year does not have that
	// week number (W53 against a 52-week year) -> missing.
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

// isoWeekMonday returns the Monday 00:00 of week of the ISO year (local time
// zone). January 4 always falls in W1, so the Monday of the week containing it
// is used as the anchor. The constructed result is verified by reading back
// through ISOWeek (invalid combinations such as asking for W53 in a 52-week
// year error out).
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
		YoyMissing: true, // the year tier has no YoY column (section 5.1)
	}, nil
}
