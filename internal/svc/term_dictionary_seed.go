package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
)

type termDictionarySeedConfig struct {
	Items []termDictionarySeedItem `json:"items"`
}

type termDictionarySeedItem struct {
	Domain  string            `json:"domain"`
	Key     string            `json:"key"`
	Aliases []string          `json:"aliases"`
	Display map[string]string `json:"display"`
	Order   int               `json:"order"`

	// 旧文件字段：displayZh/displayEn。仅为防止旧种子文件升级时把
	// 已入库的翻译清空而保留读取；canonical 写法是 display。
	DisplayZh string `json:"displayZh"`
	DisplayEn string `json:"displayEn"`
}

// termDisplay 解析种子的本地化文本：display 优先，旧双字段兜底，key 统一 BCP47。
func (it termDictionarySeedItem) termDisplay() map[string]string {
	if len(it.Display) > 0 {
		return model.NormalizeTermDisplay(it.Display)
	}
	legacy := map[string]string{}
	if strings.TrimSpace(it.DisplayZh) != "" {
		legacy["zh-CN"] = it.DisplayZh
	}
	if strings.TrimSpace(it.DisplayEn) != "" {
		legacy["en-US"] = it.DisplayEn
	}
	return model.NormalizeTermDisplay(legacy)
}

func seedBootstrapTermDictionary(ctx *ServiceContext) error {
	if ctx == nil || ctx.TermDictModel == nil {
		return nil
	}
	bg := context.Background()
	cfg := loadTermDictionaryConfig(ctx)
	for _, item := range cfg.Items {
		domain := strings.TrimSpace(strings.ToLower(item.Domain))
		key := strings.TrimSpace(strings.ToLower(item.Key))
		if domain == "" {
			return fmt.Errorf("term dictionary seed domain is required for key %q", item.Key)
		}
		if key == "" {
			return fmt.Errorf("term dictionary seed key is required for domain %q", domain)
		}
		aliases := append([]string{key}, item.Aliases...)
		for _, alias := range aliases {
			alias = strings.TrimSpace(strings.ToLower(alias))
			if alias == "" {
				return fmt.Errorf("term dictionary seed alias is required for domain %q key %q", domain, key)
			}
			err := ctx.TermDictModel.Upsert(bg, &model.TermDictionary{
				Domain:    domain,
				TermKey:   key,
				Alias:     alias,
				Display:   item.termDisplay(),
				SortOrder: item.Order,
			})
			if err != nil {
				return fmt.Errorf("seed term dictionary failed for domain %q alias %q: %w", domain, alias, err)
			}
		}
	}
	return nil
}

func loadTermDictionaryConfig(ctx *ServiceContext) termDictionarySeedConfig {
	if ctx == nil {
		return defaultTermDictionaryConfig()
	}
	base := resolveBootstrapBaseDir(ctx.Config)
	path := filepath.Join(base, "term_dictionary.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultTermDictionaryConfig()
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	var cfg termDictionarySeedConfig
	if err := json.Unmarshal(data, &cfg); err != nil || len(cfg.Items) == 0 {
		return defaultTermDictionaryConfig()
	}
	return cfg
}

func defaultTermDictionaryConfig() termDictionarySeedConfig {
	return termDictionarySeedConfig{
		Items: []termDictionarySeedItem{
			{Domain: "resource", Key: "player", Aliases: []string{"players", "user", "users", "role"}, Display: map[string]string{"zh-CN": "玩家", "en-US": "Player"}, Order: 10},
			{Domain: "resource", Key: "guild", Aliases: []string{"clan", "alliance"}, Display: map[string]string{"zh-CN": "公会", "en-US": "Guild"}, Order: 20},
			{Domain: "resource", Key: "item", Aliases: nil, Display: map[string]string{"zh-CN": "道具", "en-US": "Item"}, Order: 30},
			{Domain: "resource", Key: "inventory", Aliases: []string{"bag"}, Display: map[string]string{"zh-CN": "背包", "en-US": "Inventory"}, Order: 31},
			{Domain: "resource", Key: "mail", Aliases: []string{"message", "messages"}, Display: map[string]string{"zh-CN": "邮件", "en-US": "Mail"}, Order: 40},
			{Domain: "resource", Key: "order", Aliases: []string{"payment", "payments", "transaction"}, Display: map[string]string{"zh-CN": "订单", "en-US": "Order"}, Order: 50},
			{Domain: "resource", Key: "match", Aliases: []string{"battle", "arena"}, Display: map[string]string{"zh-CN": "对局", "en-US": "Match"}, Order: 60},
			{Domain: "operation", Key: "create", Aliases: []string{"add", "new"}, Display: map[string]string{"zh-CN": "创建", "en-US": "Create"}, Order: 10},
			{Domain: "operation", Key: "read", Aliases: []string{"get", "list", "query", "search", "detail"}, Display: map[string]string{"zh-CN": "查询", "en-US": "Query"}, Order: 20},
			{Domain: "operation", Key: "update", Aliases: []string{"edit", "patch", "modify"}, Display: map[string]string{"zh-CN": "更新", "en-US": "Update"}, Order: 30},
			{Domain: "operation", Key: "delete", Aliases: []string{"remove"}, Display: map[string]string{"zh-CN": "删除", "en-US": "Delete"}, Order: 40},
			{Domain: "operation", Key: "ban", Aliases: []string{"mute", "unban", "unmute"}, Display: map[string]string{"zh-CN": "风控", "en-US": "Risk Control"}, Order: 50},
			{Domain: "operation", Key: "execute", Aliases: []string{"invoke", "run"}, Display: map[string]string{"zh-CN": "执行", "en-US": "Execute"}, Order: 60},
		},
	}
}
