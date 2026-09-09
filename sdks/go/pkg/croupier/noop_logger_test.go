package croupier

import "testing"

// NoOpLogger 四个方法都应是安全空操作（panic 即失败）。
func TestNoOpLoggerMethodsAreSafe(t *testing.T) {
	logger := &NoOpLogger{}
	logger.Debugf("debug %s", "msg")
	logger.Infof("info %d", 1)
	logger.Warnf("warn %v", true)
	logger.Errorf("error %s", "msg")
}

// SetGlobalLogger 替换为 NoOpLogger 后，包级 helper 也不应 panic。
func TestGlobalLoggerHelpersWithNoOp(t *testing.T) {
	previous := GetGlobalLogger()
	SetGlobalLogger(&NoOpLogger{})
	defer SetGlobalLogger(previous)

	logDebugf("debug via helper %s", "x")
	logInfof("info via helper %d", 1)
	logWarnf("warn via helper %v", 2)
	logErrorf("error via helper %s", "x")
}
