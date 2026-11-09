package incident

import (
	"time"
)

// buildCompare 组装指标三列（§5.4）。
func buildCompare(cur *periodStats, prev, yoy *periodStats, pick metricPick,
	epochOK bool, epoch time.Time, prevEnd, yoyEnd time.Time, yoyForceMissing bool) CompareValue {
	curVal, curHas := pick(cur)
	cv := CompareValue{}
	if curHas {
		v := curVal
		cv.Value = &v
	}
	cv.Prev = buildDelta(curVal, curHas, prev, pick, prevEnd, epochOK, epoch)
	if yoyForceMissing || yoy == nil {
		cv.Yoy = &CompareDelta{Missing: true}
	} else {
		cv.Yoy = buildDelta(curVal, curHas, yoy, pick, yoyEnd, epochOK, epoch)
	}
	return cv
}

// buildDelta 计算单个对照期差值；对照期整体晚于数据起点（该期完全无数据）
// 或基期无样本 → missing（不置 0、不算假数，§5.4）。
func buildDelta(curVal float64, curHas bool, base *periodStats, pick metricPick,
	baseEnd time.Time, epochOK bool, epoch time.Time) *CompareDelta {
	if base == nil || !epochOK || !baseEnd.After(epoch) {
		return &CompareDelta{Missing: true}
	}
	baseVal, baseHas := pick(base)
	if !baseHas {
		return &CompareDelta{Missing: true}
	}
	d := &CompareDelta{}
	bv := baseVal
	d.Value = &bv
	if curHas {
		delta := curVal - baseVal
		d.Delta = &delta
		if baseVal != 0 {
			pct := delta / baseVal * 100
			d.Pct = &pct
		}
	}
	return d
}

// ---- Service：汇总 ----
