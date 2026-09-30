package cicd_test

// Create 显式 enabled 指针翼（覆盖率巡检 R48：service.go:219
// `row.Enabled = *req.Enabled` 是全量 profile 中唯一无主未覆盖块——其余
// 21 文件 31 语句均与第 3-28 轮登记/回避台账逐行吻合）。
//
// 锁定契约：
// - `"enabled": true` 显式指针 → :219 赋值翼执行；true 非 bool 零值，
//   不触发 gorm:"default:true" 的丢列行为，INSERT 真实落库；
// - 对照组：键缺省（nil 指针）不进赋值翼，落库形态与显式 true 同为
//   true（默认值），两翼行为同形但代码路径分叉；
// - 已知缺陷边界（不在本用例断言面）：显式 false 会被
//   model.CicdIntegration.Enabled 的 gorm:"default:true" 在驱动层丢弃
//   （「创建即停用」不可达）——见 service_paths_test.go
//   TestCicdService_CreateEnabledFalseIsDroppedKnownDefect（t.Skip 登记，
//   修法需 *bool 模型字段 + 编号迁移，另立批次）。
import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCicdService_CreateExplicitEnabledWings(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	// 显式 true 指针 → :219 赋值翼执行
	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"enabled-true","endpoint":"http://ci.internal","enabled":true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"enabled":true`)

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "enabled-true").First(&row).Error)
	assert.True(t, row.Enabled, "显式 true 应真实落库（非零值，不涉 default 丢弃缺陷）")

	// 对照：键缺省（nil 指针）不进赋值翼，默认 true 同形落库
	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"enabled-nil","endpoint":"http://ci.internal"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"enabled":true`)

	// 坑规避：GORM First(&row) 会把 struct 既有主键并入条件——第二查须用新变量
	var rowNil model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "enabled-nil").First(&rowNil).Error)
	assert.True(t, rowNil.Enabled, "键缺省走构造器默认 true")
}
