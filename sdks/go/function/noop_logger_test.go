package function

import "testing"

// NoOpLogger 四个方法都应是安全空操作（panic 即失败）。
func TestNoOpLoggerMethodsAreSafe(t *testing.T) {
	logger := &NoOpLogger{}
	logger.Debug("debug", "key", "value")
	logger.Info("info", "key", "value")
	logger.Warn("warn", "key", "value")
	logger.Error("error", "key", "value")
}
