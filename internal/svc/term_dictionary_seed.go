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
	Domain    string   `json:"domain"`
	Key       string   `json:"key"`
	Aliases   []string `json:"aliases"`
	DisplayZh string   `json:"display_zh"`
	DisplayEn string   `json:"display_en"`
	Order     int      `json:"order"`
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
				DisplayZh: strings.TrimSpace(item.DisplayZh),
				DisplayEn: strings.TrimSpace(item.DisplayEn),
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
			{Domain: "resource", Key: "player", Aliases: []string{"players", "user", "users", "role"}, DisplayZh: "玩家", DisplayEn: "Player", Order: 10},
			{Domain: "resource", Key: "guild", Aliases: []string{"clan", "alliance"}, DisplayZh: "公会", DisplayEn: "Guild", Order: 20},
			{Domain: "resource", Key: "item", Aliases: []string{"bag", "inventory"}, DisplayZh: "道具", DisplayEn: "Item", Order: 30},
			{Domain: "resource", Key: "mail", Aliases: []string{"message", "messages"}, DisplayZh: "邮件", DisplayEn: "Mail", Order: 40},
			{Domain: "resource", Key: "order", Aliases: []string{"payment", "payments", "transaction"}, DisplayZh: "订单", DisplayEn: "Order", Order: 50},
			{Domain: "resource", Key: "match", Aliases: []string{"battle", "arena"}, DisplayZh: "对局", DisplayEn: "Match", Order: 60},
			{Domain: "operation", Key: "create", Aliases: []string{"add", "new"}, DisplayZh: "创建", DisplayEn: "Create", Order: 10},
			{Domain: "operation", Key: "read", Aliases: []string{"get", "list", "query", "search", "detail"}, DisplayZh: "查询", DisplayEn: "Query", Order: 20},
			{Domain: "operation", Key: "update", Aliases: []string{"edit", "patch", "modify"}, DisplayZh: "更新", DisplayEn: "Update", Order: 30},
			{Domain: "operation", Key: "delete", Aliases: []string{"remove"}, DisplayZh: "删除", DisplayEn: "Delete", Order: 40},
			{Domain: "operation", Key: "ban", Aliases: []string{"mute", "unban", "unmute"}, DisplayZh: "风控", DisplayEn: "Risk Control", Order: 50},
			{Domain: "operation", Key: "execute", Aliases: []string{"invoke", "run"}, DisplayZh: "执行", DisplayEn: "Execute", Order: 60},
		},
	}
}
