package generator

import (
	"strings"
	"unicode"
)

// HumanizeKey 把机器 key 转成可读标题：
//   - 分隔符（. _ - 空格）拆词；
//   - camelCase 边界拆词（含小写→大写、字母↔数字、大写尾缩写后接小写词，
//     如 "HTTPServer" -> "HTTP Server"）；
//   - 每个词首字母大写后以空格连接。
//
// 例："player.ban" -> "Player Ban"；"inventory_list" -> "Inventory List"；
// "playerId" -> "Player Id"。非 ASCII 词（中文等）原样保留、不受影响。
// 用于函数未声明 summary、词条字典未配置时的菜单/标题兜底，对齐
// docs/architecture/ui-generation.md 的 humanize 承诺。
func HumanizeKey(key string) string {
	trimmed := strings.Trim(strings.TrimSpace(key), "._-")
	if trimmed == "" {
		return ""
	}
	fields := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})
	var words []string
	for _, field := range fields {
		words = append(words, splitCamelWord(field)...)
	}
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func splitCamelWord(word string) []string {
	runes := []rune(word)
	if len(runes) <= 1 {
		return []string{word}
	}
	var out []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		boundary := unicode.IsLower(prev) && unicode.IsUpper(cur) || // aB
			unicode.IsDigit(prev) != unicode.IsDigit(cur) || // a1 / 1a
			unicode.IsUpper(prev) && unicode.IsUpper(cur) && nextLower // HTTP S -> HTTP|Server
		if boundary {
			out = append(out, string(runes[start:i]))
			start = i
		}
	}
	return append(out, string(runes[start:]))
}
