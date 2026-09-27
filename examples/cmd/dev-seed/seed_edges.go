package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// seedEdges 边界与异常数据集——分页阈值专项由 -heavy（万级）+ 现有自然分布
// 覆盖；这里补：悬空引用/孤儿、前后空白、emoji/CJK/RTL/XSS 字段、未来时间。
// 页大小阈值（0/1/恰好一页/±1）属于查询行为，靠 1/19/20/21 条的小表 +
// 万级大表两端夹逼，由查询方在页面/接口上验证。
func seedEdges(gdb *gorm.DB) error {
	// 玩家：恰好 0/1/19/20/21 条的分组（按游戏维度天然分页）
	edgeGames := []struct {
		game  string
		count int
	}{{"edge-p0", 0}, {"edge-p1", 1}, {"edge-p19", 19}, {"edge-p20", 20}, {"edge-p21", 21}}
	// 注意：edge-p0 没有玩家行，但游戏注册让空列表可看（分页阈值 0 条）
	for _, eg := range edgeGames {
		if eg.count == 0 {
			continue
		}
		var n int64
		if err := gdb.Model(&model.Player{}).Where("game_id = ?", eg.game).Count(&n).Error; err != nil {
			return err
		}
		if n >= int64(eg.count) {
			continue
		}
		batch := make([]model.Player, 0, eg.count-int(n))
		for i := int(n); i < eg.count; i++ {
			batch = append(batch, model.Player{
				Username: fmt.Sprintf("seed-%s-%03d", eg.game, i),
				Nickname: fmt.Sprintf("分页%s玩家%03d", eg.game, i),
				GameID:   eg.game,
				Status:   1,
				Level:    1 + i%100,
			})
		}
		if err := gdb.CreateInBatches(&batch, 100).Error; err != nil {
			return err
		}
	}
	// 分页阈值 0 条：注册 edge-p0 游戏本身（列表恒空）
	if err := gdb.Where(model.Game{GameID: "edge-p0"}).FirstOrCreate(&model.Game{
		GameID: "edge-p0", Name: "分页阈值-0条游戏", AliasName: "edge-p0-alias", Enabled: true, Status: "dev",
	}).Error; err != nil {
		return err
	}
	return nil
}

// seedHeavy 万级数据：1 万玩家 + 1 万工单（-heavy 时执行），把分页/筛选/
// 排序推到大表量级。批量插入 + 幂等（按 game_id 计数哨兵）。
func seedHeavy(gdb *gorm.DB) error {
	const total = 10000
	var n int64
	if err := gdb.Model(&model.Player{}).Where("game_id = ?", "heavy-load").Count(&n).Error; err != nil {
		return err
	}
	if n < total {
		batch := make([]model.Player, 0, total-int(n))
		for i := int(n); i < total; i++ {
			batch = append(batch, model.Player{
				Username: fmt.Sprintf("seed-heavy-%05d", i),
				Nickname: fmt.Sprintf("压测玩家%05d", i),
				GameID:   "heavy-load",
				Status:   1,
				Level:    1 + i%100,
				Balance:  int64(i * 7),
			})
		}
		if err := gdb.CreateInBatches(&batch, 500).Error; err != nil {
			return err
		}
	}
	var tn int64
	if err := gdb.Model(&model.Ticket{}).Where("game_id = ?", "heavy-load").Count(&tn).Error; err != nil {
		return err
	}
	if tn >= total {
		return nil
	}
	base := time.Now().AddDate(0, 0, -180)
	batch := make([]model.Ticket, 0, total-int(tn))
	for i := int(tn); i < total; i++ {
		batch = append(batch, model.Ticket{
			Title:    fmt.Sprintf("【heavy】压测工单%05d", i),
			Content:  "压测数据行。",
			Category: []string{"账号问题", "充值问题", "玩法咨询"}[i%3],
			Priority: []string{"low", "medium", "high"}[i%3],
			Status:   dbenum.TicketStatus(i % 4),
			GameID:   "heavy-load",
			Env:      "dev",
			Source:   "web",
			Model: gorm.Model{
				CreatedAt: base.Add(time.Duration(i) * time.Minute),
				UpdatedAt: time.Now(),
			},
		})
	}
	return gdb.CreateInBatches(&batch, 500).Error
}

// ---------- 小工具 ----------

// stringMul 重复 s n 次构造超长字段样例。
func stringMul(s string, n int) string {
	return strings.Repeat(s, n)
}

// modelJSONMap 把 JSON 字符串转成 datatypes.JSONMap；非法 JSON 返回空 map
// （种子数据全部为字面量常量，非法即 bug，测试会暴露）。
func modelJSONMap(s string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func timePtr(t time.Time) *time.Time { return &t }

// firstOrCreateAllBy 按 username 自然键幂等写入一批玩家。
func firstOrCreateAllBy(gdb *gorm.DB, players *[]model.Player, _ string) error {
	for i := range *players {
		if err := gdb.Where(model.Player{Username: (*players)[i].Username}).
			FirstOrCreate(&(*players)[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
