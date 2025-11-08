package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Writer struct {
	mu   sync.Mutex
	f    *os.File
	prev []byte // previous hash
}

func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f, prev: make([]byte, 32)}, nil
}

func (w *Writer) Close() error { return w.f.Close() }

type Event struct {
	Time   time.Time         `json:"time"`
	Kind   string            `json:"kind"`
	Actor  string            `json:"actor"`
	Target string            `json:"target"`
	Meta   map[string]string `json:"meta"`
	Prev   string            `json:"prev"`
	Hash   string            `json:"hash"`
}

func (w *Writer) Log(kind, actor, target string, meta map[string]string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	ev := Event{Time: time.Now().UTC(), Kind: kind, Actor: actor, Target: target, Meta: meta, Prev: hex.EncodeToString(w.prev)}
	b, _ := json.Marshal(ev)
	h := sha256.Sum256(append(w.prev, b...))
	ev.Hash = hex.EncodeToString(h[:])
	b, _ = json.Marshal(ev)
	if _, err := w.f.Write(append(b, '\n')); err != nil {
		return err
	}
	copy(w.prev, h[:])
	return nil
}

func filepathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
