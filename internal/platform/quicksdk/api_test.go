package quicksdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// newTestServer creates a test HTTP server that returns the given response.
func newTestServer(resp Response) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
}

// newTestClient creates a client pointing at the given server.
func newTestClient(srv *httptest.Server) *Client {
	c, _ := NewClient(Config{
		OpenID:     "test",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)
	return c
}

func TestGetChannelList(t *testing.T) {
	data := `[{"channelName":"TestChannel","channelCode":"100","marketOs":"1"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	channels, err := c.GetChannelList(context.Background(), "product1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].ChannelName != "TestChannel" {
		t.Errorf("expected channel name 'TestChannel', got %q", channels[0].ChannelName)
	}
}

func TestGetServerList(t *testing.T) {
	data := `[{"serverName":"Server1"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	servers, err := c.GetServerList(context.Background(), "product1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].ServerName != "Server1" {
		t.Errorf("expected server name 'Server1', got %q", servers[0].ServerName)
	}
}

func TestGetProductList(t *testing.T) {
	data := `[{"productName":"Game","productCode":"gc","productKey":"pk","callbackKey":"ck","md5Key":"mk","gameType":1,"callbackUrl":"http://example.com"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	products, err := c.GetProductList(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	if products[0].ProductName != "Game" {
		t.Errorf("expected product name 'Game', got %q", products[0].ProductName)
	}
}

func TestGetRoleInfo(t *testing.T) {
	data := `{"total":1,"list":[{"uid":"u1","username":"player1","roleName":"Hero","roleId":"r1"}]}`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	roles, err := c.GetRoleInfo(context.Background(), "p1", "s1", "r1", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(roles))
	}
	if roles[0].RoleName != "Hero" {
		t.Errorf("expected role name 'Hero', got %q", roles[0].RoleName)
	}
}

func TestGetRoleInfo_WithAllParams(t *testing.T) {
	var capturedForm url.Values
	data := `{"total":0,"list":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		capturedForm = r.Form
		json.NewEncoder(w).Encode(Response{Status: true, Data: json.RawMessage(data)})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _ = c.GetRoleInfo(context.Background(), "p1", "s1", "r1", "role1", "user1")

	if capturedForm.Get("roleId") != "r1" {
		t.Errorf("expected roleId 'r1', got %q", capturedForm.Get("roleId"))
	}
	if capturedForm.Get("roleName") != "role1" {
		t.Errorf("expected roleName 'role1', got %q", capturedForm.Get("roleName"))
	}
	if capturedForm.Get("username") != "user1" {
		t.Errorf("expected username 'user1', got %q", capturedForm.Get("username"))
	}
}

func TestGetOrderList(t *testing.T) {
	data := `[{"orderNo":"ORD001","userName":"player1","amount":9.99,"status":4}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	orders, err := c.GetOrderList(context.Background(), "p1", OrderListOptions{
		BeginTime: "2024-01-01 00:00:00",
		EndTime:   "2024-01-31 23:59:59",
		Page:      1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].OrderNo != "ORD001" {
		t.Errorf("expected orderNo 'ORD001', got %q", orders[0].OrderNo)
	}
}

func TestGetDayReport(t *testing.T) {
	data := `[{"date":"2024-01-01","newUser":100,"allPay":500.50}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	reports, err := c.GetDayReport(context.Background(), "p1", DayReportOptions{
		BeginTime: 1704067200,
		EndTime:   1704153600,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].NewUser != 100 {
		t.Errorf("expected newUser 100, got %d", reports[0].NewUser)
	}
}

func TestGetDayHourReport(t *testing.T) {
	data := `[{"date_hour":"2024-01-01 10","newUser":10}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	reports, err := c.GetDayHourReport(context.Background(), "p1", DayHourReportOptions{
		DateTime: 1704067200,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
}

func TestGetUserLive(t *testing.T) {
	data := `[{"date":"2024-01-01","newUser":100,"live_2":80,"live_7":50}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	reports, err := c.GetUserLive(context.Background(), "p1", UserLiveOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
}

func TestGetChannelDaysReport(t *testing.T) {
	data := `[{"channelName":"ch1","newUser":50,"allPay":1000.0}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	reports, err := c.GetChannelDaysReport(context.Background(), "p1", ChannelDaysReportOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
}

func TestGetChannelReport(t *testing.T) {
	data := `[{"channelName":"ch1","newUser":50}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	reports, err := c.GetChannelReport(context.Background(), "p1", ChannelReportOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
}

func TestGetAdReport(t *testing.T) {
	data := `[{"date":"2024-01-01","planCode":"p1","installNum":100}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	reports, err := c.GetAdReport(context.Background(), "p1", "2024-01-01", "2024-01-31", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
}

func TestGetAdReport_WithPlans(t *testing.T) {
	var capturedForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		capturedForm = r.Form
		json.NewEncoder(w).Encode(Response{Status: true, Data: json.RawMessage(`[]`)})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _ = c.GetAdReport(context.Background(), "p1", "2024-01-01", "2024-01-31", "plan1,plan2")

	if capturedForm.Get("plans") != "plan1,plan2" {
		t.Errorf("expected plans 'plan1,plan2', got %q", capturedForm.Get("plans"))
	}
}

func TestGetMediaAppList(t *testing.T) {
	data := `[{"appId":"a1","mediaId":"m1","mediaName":"TestMedia"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	apps, err := c.GetMediaAppList(context.Background(), "Toutiao")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
}

func TestGetAdPlanGroupList(t *testing.T) {
	data := `[{"groupId":"g1","groupName":"Group1"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	groups, err := c.GetAdPlanGroupList(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

func TestGetPackageVersionList(t *testing.T) {
	data := `[{"packId":"p1","packName":"Pack","versionNo":"1.0","versionName":"v1"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	versions, err := c.GetPackageVersionList(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
}

func TestGetAdPagesList(t *testing.T) {
	data := `[{"pageId":"p1","pageName":"Page","tempId":"t1","tempName":"Template"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	pages, err := c.GetAdPagesList(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestCreateAdPlan(t *testing.T) {
	data := `{"success":1,"total":1,"list":[{"planName":"Plan1","planCode":"pc1"}]}`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	result, err := c.CreateAdPlan(context.Background(), CreateAdPlanOptions{
		ProductCode: "p1",
		Plans:       "Plan1",
		Platform:    "APP",
		ActiveNode:  "INSTALL",
		URLType:     "URL",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success != 1 {
		t.Errorf("expected success 1, got %d", result.Success)
	}
	if len(result.List) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(result.List))
	}
}

func TestUpdateAdPlan(t *testing.T) {
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(`{}`)})
	defer srv.Close()

	c := newTestClient(srv)
	err := c.UpdateAdPlan(context.Background(), UpdateAdPlanOptions{
		ProductCode: "p1",
		Action:      "FROM_CODE",
		URLType:     "URL",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAdPlanList(t *testing.T) {
	data := `{"total":1,"list":[{"planName":"Plan1","planCode":"pc1"}]}`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	result, err := c.GetAdPlanList(context.Background(), "p1", AdPlanListOptions{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
}

func TestGetUserLostList(t *testing.T) {
	data := `[{"channelName":"ch1","userId":"u1","payAmount":100.0,"warnLevel":"high"}]`
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(data)})
	defer srv.Close()

	c := newTestClient(srv)
	users, err := c.GetUserLostList(context.Background(), "p1", UserLostListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestPushMessage(t *testing.T) {
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(`{}`)})
	defer srv.Close()

	c := newTestClient(srv)
	err := c.PushMessage(context.Background(), "p1", "c1", "huawei", "Title", "Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPI_ErrorPropagation(t *testing.T) {
	srv := newTestServer(Response{Status: false, Message: "api error"})
	defer srv.Close()

	c := newTestClient(srv)

	tests := []struct {
		name string
		fn   func() error
	}{
		{"GetChannelList", func() error { _, err := c.GetChannelList(context.Background(), "p1"); return err }},
		{"GetServerList", func() error { _, err := c.GetServerList(context.Background(), "p1"); return err }},
		{"GetProductList", func() error { _, err := c.GetProductList(context.Background()); return err }},
		{"GetDayReport", func() error { _, err := c.GetDayReport(context.Background(), "p1", DayReportOptions{}); return err }},
		{"GetDayHourReport", func() error {
			_, err := c.GetDayHourReport(context.Background(), "p1", DayHourReportOptions{})
			return err
		}},
		{"GetUserLive", func() error { _, err := c.GetUserLive(context.Background(), "p1", UserLiveOptions{}); return err }},
		{"GetChannelDaysReport", func() error {
			_, err := c.GetChannelDaysReport(context.Background(), "p1", ChannelDaysReportOptions{})
			return err
		}},
		{"GetChannelReport", func() error {
			_, err := c.GetChannelReport(context.Background(), "p1", ChannelReportOptions{})
			return err
		}},
		{"GetAdReport", func() error { _, err := c.GetAdReport(context.Background(), "p1", "d1", "d2", ""); return err }},
		{"GetMediaAppList", func() error { _, err := c.GetMediaAppList(context.Background(), "Toutiao"); return err }},
		{"GetAdPlanGroupList", func() error { _, err := c.GetAdPlanGroupList(context.Background(), "p1"); return err }},
		{"GetPackageVersionList", func() error { _, err := c.GetPackageVersionList(context.Background(), "p1"); return err }},
		{"GetAdPagesList", func() error { _, err := c.GetAdPagesList(context.Background(), "p1"); return err }},
		{"GetAdPlanList", func() error { _, err := c.GetAdPlanList(context.Background(), "p1", AdPlanListOptions{}); return err }},
		{"GetUserLostList", func() error {
			_, err := c.GetUserLostList(context.Background(), "p1", UserLostListOptions{})
			return err
		}},
		{"PushMessage", func() error { return c.PushMessage(context.Background(), "p1", "c1", "g", "t", "b") }},
		{"UpdateAdPlan", func() error { return c.UpdateAdPlan(context.Background(), UpdateAdPlanOptions{}) }},
		{"CreateAdPlan", func() error { _, err := c.CreateAdPlan(context.Background(), CreateAdPlanOptions{}); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}
