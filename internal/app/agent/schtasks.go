//go:build !js

package agent

import (
	"bytes"
	"encoding/csv"
	"strings"
)

// #24：Windows 计划任务采集的 schtasks CSV 解析。
//
// 该文件不带 windows build tag：解析是纯函数，linux CI 也要跑回归；
// exec.Command("schtasks", ...) 的调用方在 sysinfo_windows.go（仅 windows 编译）。
//
// schtasks /query /fo csv /v 的输出首行是本地化的列名（英文系统如下，顺序固定）：
//
//	HostName,TaskName,Next Run Time,Status,Logon Mode,Last Run Time,Last Result,
//	Author,Task To Run,Start In,Comment,Scheduled Task State,Idle Time,Power
//	Management,Run As User,Delete Task If Not Rescheduled,Stop Task If Runs X Hours
//	and X Mins,Schedule Type,Start Time,Date,Days,Months,Repeat: Every,...
//
// 非英文系统列名是译文，无法按名匹配——回退到固定下标（顺序跨 locale 不变）；
// 状态值同样本地化（"Disabled"/"已禁用"），Enabled 判定用禁用词启发式。

// schtasks 英文列名 → 固定下标的回退表（/v 输出顺序，跨 locale 稳定）
const (
	schtasksIdxTaskName           = 1
	schtasksIdxStatus             = 3
	schtasksIdxTaskToRun          = 8
	schtasksIdxScheduledTaskState = 11
	schtasksIdxRunAsUser          = 14
	schtasksIdxScheduleType       = 17
	schtasksIdxStartTime          = 18
	schtasksIdxDays               = 20
)

// schtasksDisabledMarkers 命中任意片段即视为禁用（覆盖英文与常见中文输出）
var schtasksDisabledMarkers = []string{"disabl", "禁用"}

// schtasksColumnIndex 按 header 行求列下标；找不到（非英文 locale）回退 -1
func schtasksColumnIndex(header []string, name string) int {
	for i := range header {
		if strings.EqualFold(strings.TrimSpace(header[i]), name) {
			return i
		}
	}
	return -1
}

// schtasksCell 按列下标取值；下标越界（畸形行）返回空串
func schtasksCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// schtasksScheduleText 把 Schedule Type/Days/Start Time 拼成人类可读的计划文本
// （Windows 无原生 cron 表达式，展示语义对齐 CronJob.Schedule 的「计划描述」位）
func schtasksScheduleText(typ, days, startTime string) string {
	parts := make([]string, 0, 3)
	if typ != "" {
		parts = append(parts, typ)
	}
	if days != "" {
		parts = append(parts, days)
	}
	if startTime != "" {
		parts = append(parts, startTime)
	}
	return strings.Join(parts, " ")
}

// schtasksEnabled 判定任务是否启用：优先 Scheduled Task State 列，
// 非英文 locale 下该值本地化，用禁用词启发式
func schtasksEnabled(stateValue string) bool {
	lower := strings.ToLower(stateValue)
	for _, marker := range schtasksDisabledMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

// ParseSchtasksCSV 解析 `schtasks /query /fo csv /v` 输出（含 header 行）。
// 解析失败的脏行跳过（不中断整表）；返回的任务保持输出顺序。
func ParseSchtasksCSV(output []byte) []CronJob {
	reader := csv.NewReader(bytes.NewReader(output))
	// 列数不固定：脏行（缺列）按实际列数宽容解析，缺的列取空串，
	// 不因个别行异常丢弃整表（运维只读展示面，宁缺勿断）
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		return nil
	}
	header := records[0]
	idxTaskName := schtasksColumnIndex(header, "TaskName")
	idxState := schtasksColumnIndex(header, "Scheduled Task State")
	idxRun := schtasksColumnIndex(header, "Task To Run")
	idxUser := schtasksColumnIndex(header, "Run As User")
	idxType := schtasksColumnIndex(header, "Schedule Type")
	idxStart := schtasksColumnIndex(header, "Start Time")
	idxDays := schtasksColumnIndex(header, "Days")
	if idxTaskName < 0 {
		// 非英文 locale：列名匹配不到，回退固定下标
		idxTaskName = schtasksIdxTaskName
		idxState = schtasksIdxScheduledTaskState
		idxRun = schtasksIdxTaskToRun
		idxUser = schtasksIdxRunAsUser
		idxType = schtasksIdxScheduleType
		idxStart = schtasksIdxStartTime
		idxDays = schtasksIdxDays
	}

	jobs := make([]CronJob, 0, len(records)-1)
	for _, row := range records[1:] {
		if len(row) <= idxTaskName || schtasksCell(row, idxTaskName) == "" {
			continue
		}
		jobs = append(jobs, CronJob{
			Schedule: schtasksScheduleText(
				schtasksCell(row, idxType),
				schtasksCell(row, idxDays),
				schtasksCell(row, idxStart),
			),
			Command:    schtasksCell(row, idxRun),
			User:       schtasksCell(row, idxUser),
			SourceFile: schtasksCell(row, idxTaskName),
			Enabled:    schtasksEnabled(schtasksCell(row, idxState)),
		})
	}
	return jobs
}

// FilterSchtasksSystemJobs 剔除 Windows 内置的计划任务（\Microsoft\ 前缀）。
// 内置任务动辄数百条且非 GM 关注点，采集端裁剪、注释与文档「已知边界」均有说明。
func FilterSchtasksSystemJobs(jobs []CronJob) []CronJob {
	kept := make([]CronJob, 0, len(jobs))
	for _, j := range jobs {
		if strings.HasPrefix(strings.ToLower(j.SourceFile), `\microsoft\`) {
			continue
		}
		kept = append(kept, j)
	}
	return kept
}
