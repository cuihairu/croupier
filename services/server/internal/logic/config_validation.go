package logic

import (
	"encoding/json"
	"encoding/xml"
	"strconv"
	"strings"
)

func validateConfigContent(format, content string) []string {
	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "json":
		if err := json.Unmarshal([]byte(content), new(any)); err != nil {
			return []string{err.Error()}
		}
	case "csv":
		return validateCSV(content)
	case "xml":
		if err := xml.Unmarshal([]byte(content), new(any)); err != nil {
			return []string{err.Error()}
		}
	case "ini":
		return validateINI(content)
	case "yaml", "yml":
		return validateYAML(content)
	default:
	}
	return nil
}

func validateCSV(content string) []string {
	lines := splitLines(content)
	cols := -1
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		cur := len(strings.Split(ln, ","))
		if cols < 0 {
			cols = cur
			continue
		}
		if cur != cols {
			return []string{"inconsistent column count"}
		}
	}
	return nil
}

func validateINI(content string) []string {
	lines := splitLines(content)
	errors := []string{}
	for idx, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, ";") || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			continue
		}
		if i := strings.Index(s, "="); i > 0 {
			if strings.TrimSpace(s[:i]) == "" {
				errors = append(errors, "line "+itoa(idx+1)+": empty key")
			}
			continue
		}
		errors = append(errors, "line "+itoa(idx+1)+": invalid ini syntax")
	}
	return errors
}

func validateYAML(content string) []string {
	if strings.Contains(content, "\x00") {
		return []string{"contains NUL byte"}
	}
	lines := splitLines(content)
	errors := []string{}
	for idx, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasPrefix(s, "-") || strings.Contains(s, ":") {
			continue
		}
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
			break
		}
		errors = append(errors, "line "+itoa(idx+1)+": suspicious yaml (no ':' or '-')")
		if len(errors) >= 5 {
			break
		}
	}
	return errors
}

func splitLines(content string) []string {
	return strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
