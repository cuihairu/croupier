package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖 List/Create/Transition 主路径与错误分支（此前 0%——仅通过 handler 间接覆盖部分）。

func TestServiceList_FiltersAndPaging(t *testing.T) {
	f := newFixture(t)
	f.seedRelease(t, "full", "1.0.0", 100)
	f.seedRelease(t, "gray", "1.1.0", 30)

	// 全量
	all, err := f.svc.List(context.Background(), &ReleaseListRequest{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, all.Items, 2)
	assert.Equal(t, int64(2), all.Total)

	// 状态过滤
	grayOnly, err := f.svc.List(context.Background(), &ReleaseListRequest{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Status: "gray", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, grayOnly.Items, 1)

	// 分页
	p1, err := f.svc.List(context.Background(), &ReleaseListRequest{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.Len(t, p1.Items, 1)
	assert.Equal(t, int64(2), p1.Total)
}

func TestServiceCreate_ValidationAndSuccess(t *testing.T) {
	f := newFixture(t)

	// 无效类型
	_, err := f.svc.Create(context.Background(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: "1.0.0", Type: "bogus",
	})
	assert.Error(t, err)

	// 成功创建 full
	res, err := f.svc.Create(context.Background(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: "2.0.0", Type: "full",
	})
	require.NoError(t, err)
	assert.NotZero(t, res.Release.Id)
	assert.Equal(t, "draft", res.Release.Status)

	// 渠道校验
	_, err = f.svc.Create(context.Background(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "", Platform: "android",
		Version: "3.0.0", Type: "full",
	})
	assert.Error(t, err)
}

func TestServiceTransition_ErrorsAndHappy(t *testing.T) {
	f := newFixture(t)

	// 无效 action / 无效 ID / 不存在 ID
	_, err := f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: "1", Action: "bogus"})
	assert.Error(t, err)
	_, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: "abc", Action: "full"})
	assert.Error(t, err)
	_, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: "999", Action: "testing"})
	assert.Error(t, err)

	// 非法迁移：draft → testing（状态机要求 draft→uploading→testing）
	draft := f.seedRelease(t, "draft", "1.0.0", 0)
	_, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(draft.ID), Action: "testing"})
	assert.Error(t, err)

	// 合法链：testing → gray（灰度）→ full → rollback
	//（testing 起点需 artifact 已传——由既有 UploadArtifact 用例覆盖，此处直接 seed）
	rel := f.seedRelease(t, "testing", "1.0.0", 0)

	res, err := f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "gray", GrayPercent: intPtr(30)})
	require.NoError(t, err)
	assert.Equal(t, "gray", res.Release.Status)

	res, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "full"})
	require.NoError(t, err)
	assert.Equal(t, "full", res.Release.Status)

	res, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "rollback"})
	require.NoError(t, err)
	assert.Equal(t, "rolled_back", res.Release.Status)
}

func intPtr(v int) *int { return &v }

func uitoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
