package crash

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Snapshot 一个 dump 产物的只读描述。
type Snapshot struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
}

// DumpStore agent 管理的 dump 产物根目录：每受管进程一个子目录
// （root/<process>/），面板列表/下载/清理都以此为界（#67 S3 接线）。
type DumpStore struct {
	root string
}

// NewDumpStore 声明产物根目录（不立即建目录，落盘时惰性创建）。
func NewDumpStore(root string) *DumpStore { return &DumpStore{root: root} }

// Root 产物根目录。
func (s *DumpStore) Root() string { return s.root }

// DirFor 受管进程的产物子目录。
func (s *DumpStore) DirFor(processName string) string {
	return filepath.Join(s.root, processName)
}

// List 列出受管进程的 dump 产物，按修改时间新→旧。目录不存在返回空表。
func (s *DumpStore) List(processName string) ([]Snapshot, error) {
	dir := s.DirFor(processName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Snapshot
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // 轮询间隙被清理，跳过
		}
		out = append(out, Snapshot{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

// SweepDefaults 保留期与配额默认档（调研 §8 报批值：72h / 2GiB）。
const (
	DefaultRetention = 72 * time.Hour
	DefaultQuota     = int64(2 << 30) // 2GiB
)

// Sweep 对单个进程产物执行保留期 + 配额清理：先删超龄，再从最旧开始删到
// 总量不超配额。返回删除的文件数；目录不存在是 no-op。
func (s *DumpStore) Sweep(processName string, retention time.Duration, quotaBytes int64) (int, error) {
	snaps, err := s.List(processName)
	if err != nil || len(snaps) == 0 {
		return 0, err
	}
	if retention <= 0 {
		retention = DefaultRetention
	}
	if quotaBytes <= 0 {
		quotaBytes = DefaultQuota
	}
	removed := 0
	cutoff := time.Now().Add(-retention)
	var total int64
	kept := snaps[:0]
	for _, sn := range snaps {
		if sn.ModTime.Before(cutoff) {
			if os.Remove(sn.Path) == nil {
				removed++
			}
			continue
		}
		total += sn.Size
		kept = append(kept, sn)
	}
	// kept 是新→旧；从尾部（最旧）开始删超配额部分。
	for i := len(kept) - 1; i >= 0 && total > quotaBytes; i-- {
		if os.Remove(kept[i].Path) == nil {
			removed++
			total -= kept[i].Size
		}
	}
	return removed, nil
}
