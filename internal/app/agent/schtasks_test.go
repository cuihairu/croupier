package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 英文 locale 的 schtasks /query /fo csv /v 形态（截取关键列，列序保持真实输出）
const schtasksCSVEnglish = `"HostName","TaskName","Next Run Time","Status","Logon Mode","Last Run Time","Last Result","Author","Task To Run","Start In","Comment","Scheduled Task State","Idle Time","Power Management","Run As User","Delete Task If Not Rescheduled","Stop Task If Runs X Hours and X Mins","Schedule Type","Start Time","Date","Days","Months","Repeat: Every","Repeat: Until: Time","Repeat: Until: Duration","Repeat: Stop If Still Running"
"DESKTOP-A","\MyApp\NightlyBackup","2026/9/28 3:00","Ready","Interactive","2026/9/27 3:00","0","author","C:\backup\nightly.exe -q","C:\backup\","","Enabled","","","CORP\svc-backup","Disabled","72:00","Daily","","2026/1/1","Every 1 day(s)","N/A","N/A","N/A","N/A","Disabled"
"DESKTOP-A","\MyApp\Report (weekly, full)","2026/9/28 6:30","Ready","Interactive","2026/9/21 6:30","0","author","C:\tools\report.cmd","C:\tools\","","Enabled","","","CORP\svc-report","Disabled","72:00","Weekly","","2026/1/1","MON","N/A","N/A","N/A","N/A","Disabled"
"DESKTOP-A","\MyApp\LegacyJob","None","Disabled","Interactive","N/A","267011","author","C:\old\job.exe","","","Disabled","","","CORP\svc-old","Disabled","72:00","One time Only","","","","N/A","N/A","N/A","N/A","Disabled"
`

// 非英文 locale（列名为译文）：列名匹配失败，回退固定下标
const schtasksCSVChinese = `"主机名","任务名","下次运行时间","模式","登录模式","上次运行时间","上次结果","作者","要运行的任务","起始于","注释","计划任务状态","空闲时间","电源管理","运行方式用户","如果不重新计划则删除任务","如果运行时间超过则停止任务","计划类型","开始时间","日期","天","月份","重复: 间隔","重复: 直到: 时间","重复: 持续时间","重复: 如果仍在运行则停止"
"DESKTOP-B","\维护\清理","2026/9/28 1:00","就绪","交互","2026/9/27 1:00","0","author","C:\clean\tidy.exe","","","已启用","","","CORP\svc","已禁用","72:00","每天","","2026/1/1","每 1 天","N/A","N/A","N/A","N/A","已禁用"
`

func TestParseSchtasksCSV_EnglishLocale(t *testing.T) {
	jobs := ParseSchtasksCSV([]byte(schtasksCSVEnglish))
	require.Len(t, jobs, 3)

	assert.Equal(t, `\MyApp\NightlyBackup`, jobs[0].SourceFile)
	assert.Equal(t, `C:\backup\nightly.exe -q`, jobs[0].Command)
	assert.Equal(t, `CORP\svc-backup`, jobs[0].User)
	assert.True(t, jobs[0].Enabled)
	// Schedule 文本 = 计划类型 + 天 + 开始时间
	assert.Contains(t, jobs[0].Schedule, "Daily")

	// 值内含引号包裹的逗号（CSV 引号字段）
	assert.Equal(t, `\MyApp\Report (weekly, full)`, jobs[1].SourceFile)
	assert.Contains(t, jobs[1].Schedule, "MON")

	// Scheduled Task State=Disabled → Enabled=false
	assert.False(t, jobs[2].Enabled)
}

func TestParseSchtasksCSV_NonEnglishFallback(t *testing.T) {
	jobs := ParseSchtasksCSV([]byte(schtasksCSVChinese))
	require.Len(t, jobs, 1)

	assert.Equal(t, `\维护\清理`, jobs[0].SourceFile)
	assert.Equal(t, `C:\clean\tidy.exe`, jobs[0].Command)
	assert.True(t, jobs[0].Enabled)
	assert.Contains(t, jobs[0].Schedule, "每天")
}

func TestParseSchtasksCSV_SkipsMalformedRows(t *testing.T) {
	// 空任务名行与缺列行均跳过，不中断整表
	out := schtasksCSVEnglish + `"DESKTOP-A","","","","","","","","","","","","","","","","","","","","","","","","",""
"DESKTOP-A","\Short"
`
	jobs := ParseSchtasksCSV([]byte(out))
	// "\Short" 行 TaskName 列存在但 Task To Run 越界取空串——仍产出一条（宽容解析）
	assert.Len(t, jobs, 4)
	for _, j := range jobs {
		assert.NotEmpty(t, j.SourceFile)
	}
}

func TestParseSchtasksCSV_TruncatedOrEmptyOutput(t *testing.T) {
	assert.Nil(t, ParseSchtasksCSV(nil))
	assert.Nil(t, ParseSchtasksCSV([]byte("")))
	// 只有 header、无数据行
	assert.Nil(t, ParseSchtasksCSV([]byte(`"HostName","TaskName"`)))
	// 截断的 CSV（引号未闭合）——csv reader 报错时返回 nil 而非 panic
	assert.Nil(t, ParseSchtasksCSV([]byte(`"HostName","TaskName"`+"\n\"x")))
}

func TestFilterSchtasksSystemJobs(t *testing.T) {
	jobs := []CronJob{
		{SourceFile: `\Microsoft\Windows\Defrag\ScheduledDefrag`},
		{SourceFile: `\microsoft\windows\cleanup`}, // 大小写不敏感
		{SourceFile: `\MyApp\NightlyBackup`},
		{SourceFile: ``},
	}
	kept := FilterSchtasksSystemJobs(jobs)
	require.Len(t, kept, 2)
	assert.Equal(t, `\MyApp\NightlyBackup`, kept[0].SourceFile)
	assert.Equal(t, ``, kept[1].SourceFile)
}

func TestSchtasksScheduleText(t *testing.T) {
	assert.Equal(t, "Daily Every 1 day(s) 03:00", schtasksScheduleText("Daily", "Every 1 day(s)", "03:00"))
	assert.Equal(t, "Daily", schtasksScheduleText("Daily", "", ""))
	assert.Equal(t, "", schtasksScheduleText("", "", ""))
}
