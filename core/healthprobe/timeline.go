package healthprobe

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// TimelineWriter 故障窗口时间线的本地留存（真值通道；离线不丢）。
type TimelineWriter interface {
	AppendWindow(ev WindowEvent) error
}

// FileTimeline JSON-lines 追加文件（每行一个窗口事件）。
type FileTimeline struct {
	mu sync.Mutex
	f  *os.File
}

// OpenFileTimeline 打开（必要时创建父目录）本地时间线文件。
func OpenFileTimeline(path string) (*FileTimeline, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create timeline dir: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open timeline file: %w", err)
	}
	return &FileTimeline{f: f}, nil
}

// AppendWindow 追加一条窗口事件（JSON 行）。
func (t *FileTimeline) AppendWindow(ev WindowEvent) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f == nil {
		return fmt.Errorf("timeline closed")
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := t.f.Write(append(line, '\n')); err != nil {
		return err
	}
	return t.f.Sync()
}

// Close 落盘并关闭文件。
func (t *FileTimeline) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f == nil {
		return nil
	}
	err := t.f.Close()
	t.f = nil
	return err
}

// LoadTimeline 读回全部窗口事件（按文件顺序）。
func LoadTimeline(path string) ([]WindowEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []WindowEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev WindowEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return out, fmt.Errorf("decode timeline line: %w", err)
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}
