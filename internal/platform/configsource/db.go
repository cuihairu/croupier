package configsource

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// dbSource browses a game database as config: 表即"文件"（根目录平铺），
// 读取时渲染为 CSV（首行列名）。只读——数据库直管项目的写入仍走其
// 既有后台；应急接管写入（managed 模式）属后续阶段。
//
// Config: {"dsn", "tables": ["activity", ...]}（tables 为空 = 全部表）。
type dbSource struct {
	dsn    string
	tables map[string]struct{} // 空 = 不限制

	db     *gorm.DB
	openAt time.Time
}

func newDBSource(cfg map[string]interface{}) (Source, error) {
	dsn := configString(cfg, "dsn", "")
	if dsn == "" {
		return nil, fmt.Errorf("db source requires dsn")
	}
	s := &dbSource{dsn: dsn}
	if list := configStrings(cfg, "tables"); len(list) > 0 {
		s.tables = map[string]struct{}{}
		for _, t := range list {
			s.tables[t] = struct{}{}
		}
	}
	return s, nil
}

func (s *dbSource) Type() string { return "db" }

// conn lazily opens the gorm connection.
func (s *dbSource) conn() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	db, err := gorm.Open(mysql.Open(s.dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	s.db, s.openAt = db, time.Now()
	return db, nil
}

// validTable checks the whitelist and rejects identifier injection.
func (s *dbSource) validTable(name string) bool {
	if name == "" || strings.ContainsAny(name, "`; \t\n\"'") {
		return false
	}
	if s.tables == nil {
		return true
	}
	_, ok := s.tables[name]
	return ok
}

func (s *dbSource) List(ctx context.Context, dir string) ([]Entry, error) {
	if _, err := cleanPath(dir); err != nil {
		return nil, err
	}
	if dir != "" {
		return nil, nil // 表平铺在根，无子目录
	}
	db, err := s.conn()
	if err != nil {
		return nil, err
	}
	var tables []string
	if err := db.WithContext(ctx).Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	out := make([]Entry, 0, len(tables))
	for _, t := range tables {
		if !s.validTable(t) {
			continue
		}
		out = append(out, Entry{Name: t + ".csv", Path: t + ".csv"})
	}
	return out, nil
}

// Read renders a table as CSV (首行列名，最多 500 行)。
func (s *dbSource) Read(ctx context.Context, path string) ([]byte, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	table := strings.TrimSuffix(path, ".csv")
	if !s.validTable(table) {
		return nil, fmt.Errorf("table not allowed: %s", path)
	}
	db, err := s.conn()
	if err != nil {
		return nil, err
	}
	rows, err := db.WithContext(ctx).Raw("SELECT * FROM `" + table + "` LIMIT 500").Rows()
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	if err := w.Write(cols); err != nil {
		return nil, err
	}
	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		record := make([]string, len(cols))
		for i, v := range vals {
			switch tv := v.(type) {
			case []byte:
				record[i] = string(tv)
			case nil:
				record[i] = ""
			default:
				record[i] = fmt.Sprintf("%v", tv)
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}
