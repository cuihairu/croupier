package common

import (
    "encoding/json"
    "io"
    "log"
    "os"
    "strings"
    "time"
)

// SetupLogger configures standard log package according to level/format.
// level: debug|info|warn|error (currently informational only for std log)
// format: console|json
func SetupLogger(level, format string) {
    lvl := strings.ToLower(strings.TrimSpace(level))
    if lvl == "" { lvl = "info" }
    f := strings.ToLower(strings.TrimSpace(format))
    if f == "json" {
        log.SetFlags(0)
        log.SetOutput(jsonWriter{level: lvl, w: os.Stderr})
    } else {
        // console
        log.SetFlags(log.LstdFlags | log.Lmicroseconds)
        log.SetOutput(os.Stderr)
    }
}

type jsonWriter struct {
    level string
    w     io.Writer
}

func (j jsonWriter) Write(p []byte) (int, error) {
    // Strip trailing newline; wrap into JSON with ts, level, msg
    msg := strings.TrimRight(string(p), "\n")
    rec := map[string]any{
        "ts":    time.Now().Format(time.RFC3339Nano),
        "level": j.level,
        "msg":   msg,
    }
    b, _ := json.Marshal(rec)
    b = append(b, '\n')
    return j.w.Write(b)
}

