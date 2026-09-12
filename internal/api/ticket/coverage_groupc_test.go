// 补齐 ticket 包剩余可覆盖分支（group C）：
// service.Transition 更新成功后回读评论列表失败（ticket_comments 表缺失）。
package ticket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceTransition_ListCommentsError(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	// 状态变更与工单回读均正常，仅评论列表查询失败。
	require.NoError(t, db.Migrator().DropTable("ticket_comments"))
	_, err := s.Transition(context.Background(), &TransitionRequest{ID: id, Status: "closed"})
	require.Error(t, err)
}
