//go:build !race

package hotreload

// testBinaryRaceEnabled 报告测试二进制是否带 -race 编译。
// runtime/race 在 Go 1.27 已无公开 API，故用构建标签在两种模式下分别定义。
const testBinaryRaceEnabled = false
