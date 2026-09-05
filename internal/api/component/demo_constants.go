// 游戏常量示例数据（demo seed）：一种常量一个独立单下拉 staticForm 模板。
// 与 web 端「生成示例常量」（constantTemplateAudit.ts）保持同一套假数据；
// key 固定 consts--demo-*，种子幂等（已存在的 key 跳过）。
package component

import (
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

// demoConstantSpec 单个示例常量定义。
type demoConstantSpec struct {
	key     string
	title   string
	titleEn string
	field   string // schema 属性名（lowerCamelCase）
	options []string
}

// demoConstants 游戏运营常用常量假数据。
var demoConstants = []demoConstantSpec{
	{
		key:     "consts--demo-ban-reason",
		title:   "封禁原因",
		titleEn: "Ban Reason",
		field:   "banReason",
		options: []string{"恶意刷单", "使用外挂", "辱骂他人", "账号风险"},
	},
	{
		key:     "consts--demo-vip-level",
		title:   "会员等级",
		titleEn: "VIP Level",
		field:   "vipLevel",
		options: []string{"VIP1", "VIP2", "VIP3", "VIP4", "VIP5", "VIP6"},
	},
	{
		key:     "consts--demo-server-status",
		title:   "服务器状态",
		titleEn: "Server Status",
		field:   "serverStatus",
		options: []string{"正常", "繁忙", "维护中"},
	},
	{
		key:     "consts--demo-pay-channel",
		title:   "支付渠道",
		titleEn: "Pay Channel",
		field:   "payChannel",
		options: []string{"微信支付", "支付宝", "苹果支付", "谷歌支付"},
	},
}

// buildDemoConstantTemplate 由常量定义构造组件模板记录。
func buildDemoConstantTemplate(spec demoConstantSpec) (*model.ComponentTemplate, error) {
	staticSchema, err := json.Marshal(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			spec.field: map[string]interface{}{
				"type":      "string",
				"title":     spec.title,
				"enum":      spec.options,
				"enumNames": spec.options,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal staticSchema: %w", err)
	}
	tree, err := json.Marshal([]map[string]interface{}{{
		"id":   "staticForm-demo-" + spec.key[len("consts--demo-"):],
		"type": "staticForm",
		"props": map[string]interface{}{
			"title":        spec.title,
			"span":         12,
			"staticSchema": string(staticSchema),
		},
	}})
	if err != nil {
		return nil, fmt.Errorf("marshal tree: %w", err)
	}
	name, _ := json.Marshal(map[string]string{"zh-CN": spec.title, "en-US": spec.titleEn})
	description, _ := json.Marshal(map[string]string{
		"zh-CN": fmt.Sprintf("示例常量下拉（%d 个选项）", len(spec.options)),
		"en-US": fmt.Sprintf("Demo constant dropdown (%d options)", len(spec.options)),
	})
	return &model.ComponentTemplate{
		Key:               spec.key,
		Name:              model.JSON(name),
		Description:       model.JSON(description),
		Category:          "常量",
		Icon:              "ControlOutlined",
		RequiredFunctions: model.JSON([]byte("[]")),
		Tree:              model.JSON(tree),
		Builtin:           false,
		CreatedBy:         "demo",
	}, nil
}

// buildDemoConstantTemplates 构造全部示例常量模板。
func buildDemoConstantTemplates() ([]*model.ComponentTemplate, error) {
	out := make([]*model.ComponentTemplate, 0, len(demoConstants))
	for _, spec := range demoConstants {
		tpl, err := buildDemoConstantTemplate(spec)
		if err != nil {
			return nil, err
		}
		out = append(out, tpl)
	}
	return out, nil
}

// SeedDemoConstants serves POST /component-templates/seed-demo-constants：
// 幂等填充游戏常量示例模板（已存在的 key 跳过）。
func (h *Handler) SeedDemoConstants(c *gin.Context) {
	templates, err := buildDemoConstantTemplates()
	if err != nil {
		response.Error(c, err)
		return
	}
	created, skipped := 0, 0
	for _, tpl := range templates {
		if _, err := h.model.FindByKey(c.Request.Context(), tpl.Key); err == nil {
			skipped++
			continue
		}
		if err := h.model.Create(c.Request.Context(), tpl); err != nil {
			response.Error(c, err)
			return
		}
		created++
	}
	response.Success(c, gin.H{"created": created, "skipped": skipped})
}
