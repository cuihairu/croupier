// Excel → JSON compiler (excel-online-design.md §4/§5). One compiler serves
// both entry points: uploaded .xlsx files (parsed server-side with excelize)
// and Univer snapshot JSON from the web editor. The output shape is
// identical so consumers never care about the source.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelSheet is one compiled table.
type ExcelSheet struct {
	Fields []string                 `json:"fields"`
	Types  map[string]string        `json:"types,omitempty"`
	Rows   []map[string]interface{} `json:"rows"`
}

// ExcelWorkbook is the compiled artifact stored as ConfigVersion value
// (namespace=gameplay).
type ExcelWorkbook struct {
	Sheets map[string]*ExcelSheet `json:"sheets"`
}

// CompileOptions tunes the compiler.
type CompileOptions struct {
	// MaxRows caps rows per sheet (protect the version store).
	MaxRows int
	// MaxSheets caps the sheet count.
	MaxSheets int
}

func defaultCompileOptions() CompileOptions { return CompileOptions{MaxRows: 20000, MaxSheets: 50} }

// CompileXLSX parses an .xlsx byte slice into the compiled workbook.
func CompileXLSX(data []byte) (*ExcelWorkbook, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解析 xlsx 失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("工作簿没有任何 sheet")
	}
	opts := defaultCompileOptions()
	if len(sheets) > opts.MaxSheets {
		return nil, fmt.Errorf("sheet 数量 %d 超过上限 %d", len(sheets), opts.MaxSheets)
	}

	wb := &ExcelWorkbook{Sheets: map[string]*ExcelSheet{}}
	for _, name := range sheets {
		rows, err := f.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("读取 sheet %s 失败: %w", name, err)
		}
		sheet, err := compileRows(name, rows, opts)
		if err != nil {
			return nil, err
		}
		if sheet != nil {
			wb.Sheets[name] = sheet
		}
	}
	if len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("所有 sheet 均为空")
	}
	return wb, nil
}

// CompileSnapshot compiles an Univer-style snapshot: {"sheets": {"name":
// {"cellData": {"0": {"0": {"v": ...}}}}}}. Only plain values are consumed;
// formulas arrive as their cached results (v).
func CompileSnapshot(raw []byte) (*ExcelWorkbook, error) {
	var snap struct {
		Sheets map[string]struct {
			CellData map[string]map[string]struct {
				V interface{} `json:"v"`
			} `json:"cellData"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("快照格式无效: %w", err)
	}
	if len(snap.Sheets) == 0 {
		return nil, fmt.Errorf("快照没有任何 sheet")
	}
	opts := defaultCompileOptions()
	if len(snap.Sheets) > opts.MaxSheets {
		return nil, fmt.Errorf("sheet 数量超过上限 %d", opts.MaxSheets)
	}

	wb := &ExcelWorkbook{Sheets: map[string]*ExcelSheet{}}
	for name, sheet := range snap.Sheets {
		rows := snapshotToRows(sheet.CellData)
		compiled, err := compileRows(name, rows, opts)
		if err != nil {
			return nil, err
		}
		if compiled != nil {
			wb.Sheets[name] = compiled
		}
	}
	if len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("所有 sheet 均为空")
	}
	return wb, nil
}

// snapshotToRows converts sparse {row:{col:{v}}} into dense [][]string.
func snapshotToRows(cellData map[string]map[string]struct {
	V interface{} `json:"v"`
}) [][]string {
	maxRow, maxCol := -1, -1
	for r, cols := range cellData {
		ri := atoiDefault(r, -1)
		if ri < 0 {
			continue
		}
		if ri > maxRow {
			maxRow = ri
		}
		for c := range cols {
			ci := atoiDefault(c, -1)
			if ci < 0 {
				continue
			}
			if ci > maxCol {
				maxCol = ci
			}
		}
	}
	if maxRow < 0 || maxCol < 0 {
		return nil
	}
	grid := make([][]string, maxRow+1)
	for i := range grid {
		grid[i] = make([]string, maxCol+1)
	}
	for r, cols := range cellData {
		ri := atoiDefault(r, -1)
		if ri < 0 {
			continue
		}
		for c, cell := range cols {
			ci := atoiDefault(c, -1)
			if ci < 0 || ci >= len(grid[ri]) {
				continue
			}
			grid[ri][ci] = fmt.Sprint(cell.V)
		}
	}
	return grid
}

// compileRows implements the §4 protocol: row0=fields, optional row1=#type,
// rest=data. Returns nil for empty sheets (skipped, not an error).
func compileRows(name string, rows [][]string, opts CompileOptions) (*ExcelSheet, error) {
	var data [][]string
	for _, r := range rows {
		if !rowAllEmpty(r) {
			data = append(data, r)
		}
	}
	if len(data) < 1 {
		return nil, nil
	}

	fields := splitIdentifiers(data[0])
	if len(fields) == 0 {
		return nil, fmt.Errorf("sheet %s 首行没有合法字段名", name)
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if seen[f] {
			return nil, fmt.Errorf("sheet %s 字段 %s 重复", name, f)
		}
		seen[f] = true
	}

	sheet := &ExcelSheet{Fields: fields, Types: map[string]string{}}
	start := 1
	if len(data) > 1 && len(data[1]) > 0 && strings.HasPrefix(strings.TrimSpace(data[1][0]), "#") {
		// Type row aligns column-by-column with the header (#… marker in the
		// first cell, empty cells = untyped).
		for i := 0; i < len(fields); i++ {
			if i >= len(data[1]) {
				break
			}
			t := strings.TrimSpace(data[1][i])
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			if !validCellType(t) {
				return nil, fmt.Errorf("sheet %s 字段 %s 类型 %q 无效（int/string/float/bool）", name, fields[i], t)
			}
			sheet.Types[fields[i]] = t
		}
		start = 2
	}

	for ri := start; ri < len(data); ri++ {
		if len(sheet.Rows) >= opts.MaxRows {
			return nil, fmt.Errorf("sheet %s 行数超过上限 %d", name, opts.MaxRows)
		}
		row := map[string]interface{}{}
		empty := true
		for ci, field := range sheet.Fields {
			raw := ""
			if ci < len(data[ri]) {
				raw = strings.TrimSpace(data[ri][ci])
			}
			if raw == "" {
				continue
			}
			empty = false
			typed, err := coerceCell(raw, sheet.Types[field])
			if err != nil {
				return nil, fmt.Errorf("sheet %s 第 %d 行字段 %s: %w", name, ri+1, field, err)
			}
			row[field] = typed
		}
		if !empty {
			sheet.Rows = append(sheet.Rows, row)
		}
	}
	if len(sheet.Rows) == 0 {
		return nil, nil
	}
	return sheet, nil
}

// splitIdentifiers extracts the valid identifier cells of the header row.
func splitIdentifiers(header []string) []string {
	out := make([]string, 0, len(header))
	for _, cell := range header {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		out = append(out, cell)
	}
	return out
}

func validCellType(t string) bool {
	switch t {
	case "int", "string", "float", "bool":
		return true
	}
	return false
}

func coerceCell(raw, typ string) (interface{}, error) {
	switch typ {
	case "int":
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return n, nil
		}
		return nil, fmt.Errorf("%q 不是 int", raw)
	case "float":
		if n, err := strconv.ParseFloat(raw, 64); err == nil {
			return n, nil
		}
		return nil, fmt.Errorf("%q 不是 float", raw)
	case "bool":
		switch strings.ToLower(raw) {
		case "true", "1":
			return true, nil
		case "false", "0":
			return false, nil
		}
		return nil, fmt.Errorf("%q 不是 bool", raw)
	default:
		// Untyped: numbers stay numbers, everything else string.
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return n, nil
		}
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f, nil
		}
		return raw, nil
	}
}

func rowAllEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
