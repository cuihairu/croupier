// 覆盖目标：各 Get*/Create* API 方法在响应 Data 为非法 JSON 时的解析失败分支。
package quicksdk

import (
	"context"
	"encoding/json"
	"testing"
)

// newBadJSONTestServer 返回 status=true 但 data 为非法 JSON 的服务。
func newBadJSONTestServer(t *testing.T) (*Client, func()) {
	t.Helper()
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(`"definitely-not-json"`)})
	return newTestClient(srv), srv.Close
}

func TestV8APIParseErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("GetChannelList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetChannelList(ctx, "p"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetServerList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetServerList(ctx, "p"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetProductList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetProductList(ctx); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetRoleInfo", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetRoleInfo(ctx, "p", "s", "r", "n", "u"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetOrderList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetOrderList(ctx, "p", OrderListOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetDayReport", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetDayReport(ctx, "p", DayReportOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetDayHourReport", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetDayHourReport(ctx, "p", DayHourReportOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetUserLive", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetUserLive(ctx, "p", UserLiveOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetChannelDaysReport", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetChannelDaysReport(ctx, "p", ChannelDaysReportOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetChannelReport", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetChannelReport(ctx, "p", ChannelReportOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetAdReport", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetAdReport(ctx, "p", "2024-01-01", "2024-01-02", ""); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetMediaAppList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetMediaAppList(ctx, "media"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetAdPlanGroupList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetAdPlanGroupList(ctx, "p"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetPackageVersionList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetPackageVersionList(ctx, "p"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetAdPagesList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetAdPagesList(ctx, "p"); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("CreateAdPlan", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.CreateAdPlan(ctx, CreateAdPlanOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetAdPlanList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetAdPlanList(ctx, "p", AdPlanListOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
	t.Run("GetUserLostList", func(t *testing.T) {
		c, closeSrv := newBadJSONTestServer(t)
		defer closeSrv()
		if _, err := c.GetUserLostList(ctx, "p", UserLostListOptions{}); err == nil {
			t.Error("expected parse error")
		}
	})
}
