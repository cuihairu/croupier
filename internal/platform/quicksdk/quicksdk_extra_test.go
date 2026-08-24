package quicksdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCapturingServer returns a server that records the latest form values and
// replies with a successful Response carrying the given raw data payload.
func newCapturingServer(data string) (*httptest.Server, func() url.Values) {
	var mu sync.Mutex
	form := url.Values{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		mu.Lock()
		form = r.Form
		mu.Unlock()
		json.NewEncoder(w).Encode(Response{Status: true, Data: json.RawMessage(data)})
	}))
	snapshot := func() url.Values {
		mu.Lock()
		defer mu.Unlock()
		return form
	}
	return srv, snapshot
}

// --- API parse-error branches ---------------------------------------------

func TestExtra_API_ParseErrors(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Response{Status: true, Data: json.RawMessage(`{not-json`)})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"GetChannelList", func() error { _, err := c.GetChannelList(ctx, "p"); return err }},
		{"GetServerList", func() error { _, err := c.GetServerList(ctx, "p"); return err }},
		{"GetProductList", func() error { _, err := c.GetProductList(ctx); return err }},
		{"GetRoleInfo", func() error { _, err := c.GetRoleInfo(ctx, "p", "s", "", "", ""); return err }},
		{"GetOrderList", func() error {
			_, err := c.GetOrderList(ctx, "p", OrderListOptions{})
			return err
		}},
		{"GetDayReport", func() error {
			_, err := c.GetDayReport(ctx, "p", DayReportOptions{})
			return err
		}},
		{"GetDayHourReport", func() error {
			_, err := c.GetDayHourReport(ctx, "p", DayHourReportOptions{})
			return err
		}},
		{"GetUserLive", func() error {
			_, err := c.GetUserLive(ctx, "p", UserLiveOptions{})
			return err
		}},
		{"GetChannelDaysReport", func() error {
			_, err := c.GetChannelDaysReport(ctx, "p", ChannelDaysReportOptions{})
			return err
		}},
		{"GetChannelReport", func() error {
			_, err := c.GetChannelReport(ctx, "p", ChannelReportOptions{})
			return err
		}},
		{"GetAdReport", func() error { _, err := c.GetAdReport(ctx, "p", "d1", "d2", ""); return err }},
		{"GetMediaAppList", func() error { _, err := c.GetMediaAppList(ctx, "Toutiao"); return err }},
		{"GetAdPlanGroupList", func() error { _, err := c.GetAdPlanGroupList(ctx, "p"); return err }},
		{"GetPackageVersionList", func() error { _, err := c.GetPackageVersionList(ctx, "p"); return err }},
		{"GetAdPagesList", func() error { _, err := c.GetAdPagesList(ctx, "p"); return err }},
		{"CreateAdPlan", func() error {
			_, err := c.CreateAdPlan(ctx, CreateAdPlanOptions{ProductCode: "p"})
			return err
		}},
		{"GetAdPlanList", func() error {
			_, err := c.GetAdPlanList(ctx, "p", AdPlanListOptions{Page: 1})
			return err
		}},
		{"GetUserLostList", func() error {
			_, err := c.GetUserLostList(ctx, "p", UserLostListOptions{})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to parse")
		})
	}
}

// --- API optional parameter branches ---------------------------------------

func TestExtra_GetOrderList_AllOptionalParams(t *testing.T) {
	srv, form := newCapturingServer(`[]`)
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetOrderList(context.Background(), "p1", OrderListOptions{
		BeginTime:   "2024-01-01 00:00:00",
		EndTime:     "2024-01-02 00:00:00",
		ChannelList: "100,101",
		Page:        2,
		OrderStatus: 4,
	})
	require.NoError(t, err)
	f := form()
	assert.Equal(t, "2024-01-01 00:00:00", f.Get("btime"))
	assert.Equal(t, "2024-01-02 00:00:00", f.Get("etime"))
	assert.Equal(t, "100,101", f.Get("channelList"))
	assert.Equal(t, "2", f.Get("page"))
	assert.Equal(t, "4", f.Get("orderStatus"))
}

func TestExtra_ReportMethods_OptionalParams(t *testing.T) {
	ctx := context.Background()

	t.Run("day report", func(t *testing.T) {
		srv, form := newCapturingServer(`[]`)
		defer srv.Close()
		c := newTestClient(srv)
		_, err := c.GetDayReport(ctx, "p", DayReportOptions{
			ChannelCode: "ch1", BeginTime: 1700000000, EndTime: 1700008640, UseRMB: true,
		})
		require.NoError(t, err)
		f := form()
		assert.Equal(t, "ch1", f.Get("channelCode"))
		assert.Equal(t, "1700000000", f.Get("bTime"))
		assert.Equal(t, "1700008640", f.Get("eTime"))
		assert.Equal(t, "1", f.Get("usermb"))
	})

	t.Run("day hour report", func(t *testing.T) {
		srv, form := newCapturingServer(`[]`)
		defer srv.Close()
		c := newTestClient(srv)
		_, err := c.GetDayHourReport(ctx, "p", DayHourReportOptions{
			ChannelCode: "ch2", DateTime: 1700000000, UseRMB: true,
		})
		require.NoError(t, err)
		f := form()
		assert.Equal(t, "ch2", f.Get("channelCode"))
		assert.Equal(t, "1700000000", f.Get("dateTime"))
		assert.Equal(t, "1", f.Get("usermb"))
	})

	t.Run("user live", func(t *testing.T) {
		srv, form := newCapturingServer(`[]`)
		defer srv.Close()
		c := newTestClient(srv)
		_, err := c.GetUserLive(ctx, "p", UserLiveOptions{
			ChannelCode: "ch3", BeginTime: 100, EndTime: 200,
		})
		require.NoError(t, err)
		f := form()
		assert.Equal(t, "ch3", f.Get("channelCode"))
		assert.Equal(t, "100", f.Get("bTime"))
		assert.Equal(t, "200", f.Get("eTime"))
	})

	t.Run("channel days report", func(t *testing.T) {
		srv, form := newCapturingServer(`[]`)
		defer srv.Close()
		c := newTestClient(srv)
		_, err := c.GetChannelDaysReport(ctx, "p", ChannelDaysReportOptions{
			ChannelCode: "ch4", BeginTime: 300, EndTime: 400, UseRMB: true,
		})
		require.NoError(t, err)
		f := form()
		assert.Equal(t, "ch4", f.Get("channelCode"))
		assert.Equal(t, "300", f.Get("bTime"))
		assert.Equal(t, "400", f.Get("eTime"))
		assert.Equal(t, "1", f.Get("usermb"))
	})

	t.Run("channel report", func(t *testing.T) {
		srv, form := newCapturingServer(`[]`)
		defer srv.Close()
		c := newTestClient(srv)
		_, err := c.GetChannelReport(ctx, "p", ChannelReportOptions{
			ChannelCode: "ch5", DateTime: 500, UseRMB: true,
		})
		require.NoError(t, err)
		f := form()
		assert.Equal(t, "ch5", f.Get("channelCode"))
		assert.Equal(t, "500", f.Get("dateTime"))
		assert.Equal(t, "1", f.Get("usermb"))
	})

	t.Run("ad report with plans", func(t *testing.T) {
		srv, form := newCapturingServer(`[]`)
		defer srv.Close()
		c := newTestClient(srv)
		_, err := c.GetAdReport(ctx, "p", "sdate", "edate", "plan1")
		require.NoError(t, err)
		assert.Equal(t, "plan1", form().Get("plans"))
	})
}

func TestExtra_CreateAdPlan_AllOptionalParams(t *testing.T) {
	srv, form := newCapturingServer(`{"success":1,"total":1}`)
	defer srv.Close()

	c := newTestClient(srv)
	result, err := c.CreateAdPlan(context.Background(), CreateAdPlanOptions{
		ProductCode:    "p",
		Plans:          "planA,planB",
		Platform:       "APP",
		ChannelID:      "1001",
		PrivateChannel: "pc1",
		MediaAppID:     "ma1",
		ConvertSDKType: "1",
		CovertType:     "2",
		GroupID:        "g1",
		AdPageID:       "ap1",
		Note:           "note1",
		ActiveNode:     "INSTALL",
		URLType:        "URL",
		GameURL:        "https://game.example.com",
		Package:        "com.example.game",
		GameVersionID:  "v1",
		CPSList:        "cps1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Success)

	f := form()
	expected := map[string]string{
		"channelId": "1001", "privateChannel": "pc1", "mediaAppId": "ma1",
		"convertSdkType": "1", "covertType": "2", "groupId": "g1",
		"adPageId": "ap1", "note": "note1", "gameUrl": "https://game.example.com",
		"package": "com.example.game", "gameVersionId": "v1", "cpsList": "cps1",
	}
	for k, want := range expected {
		assert.Equal(t, want, f.Get(k), "param %s", k)
	}
}

func TestExtra_UpdateAdPlan_AllOptionalParams(t *testing.T) {
	srv, form := newCapturingServer(`null`)
	defer srv.Close()

	c := newTestClient(srv)
	err := c.UpdateAdPlan(context.Background(), UpdateAdPlanOptions{
		ProductCode:   "p",
		Action:        "FROM_VERSION",
		Plans:         "planX",
		URLType:       "CLOUD",
		GameURL:       "https://dl.example.com",
		GameVersionID: "v9",
		OldVersionID:  "v8",
		NewVersionID:  "v9",
	})
	require.NoError(t, err)

	f := form()
	for k, want := range map[string]string{
		"plans": "planX", "gameUrl": "https://dl.example.com",
		"gameVersionId": "v9", "oldVersionId": "v8", "newVersionId": "v9",
	} {
		assert.Equal(t, want, f.Get(k), "param %s", k)
	}
}

func TestExtra_GetAdPlanList_AllOptionalParams(t *testing.T) {
	srv, form := newCapturingServer(`{"total":0,"list":[]}`)
	defer srv.Close()

	c := newTestClient(srv)
	result, err := c.GetAdPlanList(context.Background(), "p", AdPlanListOptions{
		Page:      1,
		PageRows:  50,
		Status:    "DEPLOY_COMPLETE",
		ChannelID: "2001",
		AdGroupID: "grp",
		PlanCode:  "PC001",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	f := form()
	for k, want := range map[string]string{
		"pageRows": "50", "status": "DEPLOY_COMPLETE",
		"channelId": "2001", "adGroupId": "grp", "planCode": "PC001",
	} {
		assert.Equal(t, want, f.Get(k), "param %s", k)
	}
}

func TestExtra_GetUserLostList_AllOptionalParams(t *testing.T) {
	srv, form := newCapturingServer(`[]`)
	defer srv.Close()

	c := newTestClient(srv)
	users, err := c.GetUserLostList(context.Background(), "p", UserLostListOptions{
		ChannelCode: "ch6", DateTime: 900, Page: 3, Level: 1,
	})
	require.NoError(t, err)
	assert.Empty(t, users)

	f := form()
	assert.Equal(t, "ch6", f.Get("channelCode"))
	assert.Equal(t, "900", f.Get("dateTime"))
	assert.Equal(t, "3", f.Get("page"))
	assert.Equal(t, "1", f.Get("level"))
}

// --- Client extras ---------------------------------------------------------

type brokenBodyTransport struct{}

func (brokenBodyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(errReader{}),
	}, nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestExtra_Client_Do_BodyReadError(t *testing.T) {
	c := newTestClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	c.httpClient.Transport = brokenBodyTransport{}
	c.httpClient.Timeout = 0

	_, err := c.Do(context.Background(), "open/productList", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read response")
}

func TestExtra_Client_Do_RequestCreationError(t *testing.T) {
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(`[]`)})
	defer srv.Close()

	c := newTestClient(srv)
	// Control characters make the URL unparseable.
	_, err := c.Do(context.Background(), "op\x7fen/list", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func TestExtra_Client_Do_RateLimiterContextCancelled(t *testing.T) {
	srv := newTestServer(Response{Status: true, Data: json.RawMessage(`[]`)})
	defer srv.Close()

	c, err := NewClient(Config{
		OpenID:            "test",
		OpenKey:           "key",
		APIBaseURL:        srv.URL,
		RequestsPerMinute: 60,
	}, nil)
	require.NoError(t, err)

	// Drain the single burst token, then cancel the context so Wait fails.
	rl := c.rateLimiter
	select {
	case <-rl.tokens:
	default:
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = c.Do(ctx, "open/productList", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit wait")
}

// --- Provider extras --------------------------------------------------------

func TestExtra_Provider_Init_MissingOpenKey(t *testing.T) {
	p := NewProvider(nil)
	err := p.Init(context.Background(), provider.ProviderConfig{
		Config: map[string]interface{}{"open_id": "id"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open_key is required")
}

func TestExtra_Provider_Call_AdPlanRoutesAndErrorPropagation(t *testing.T) {
	okData := `{"success":1,"total":0,"list":[]}`
	srv, form := newCapturingServer(okData)
	defer srv.Close()

	p := NewProvider(nil)
	cfg := provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "id",
			"open_key":     "key",
			"api_base_url": srv.URL,
		},
	}
	require.NoError(t, p.Init(context.Background(), cfg))
	defer p.Close()

	t.Run("create_ad_plan full params", func(t *testing.T) {
		req, err := json.Marshal(map[string]interface{}{
			"product_code":     "p",
			"plans":            "planZ",
			"platform":         "WEB",
			"channel_id":       "77",
			"private_channel":  "pv",
			"media_app_id":     "app",
			"convert_sdk_type": "1",
			"covert_type":      "2",
			"group_id":         "gg",
			"ad_page_id":       "pg",
			"note":             "n",
			"active_node":      "PAY",
			"url_type":         "URL",
			"game_url":         "https://x.example.com",
			"package":          "pkg",
			"game_version_id":  "gv",
			"cps_list":         "cs",
		})
		require.NoError(t, err)

		out, err := p.Call(context.Background(), "create_ad_plan", req)
		require.NoError(t, err)
		var res CreateAdPlanResult
		require.NoError(t, json.Unmarshal(out, &res))
		assert.Equal(t, 1, res.Success)
		assert.Equal(t, "77", form().Get("channelId"))
	})

	t.Run("update_ad_plan returns status ok", func(t *testing.T) {
		out, err := p.Call(context.Background(), "update_ad_plan",
			[]byte(`{"product_code":"p","action":"FROM_CODE","url_type":"CLOUD"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"status":"ok"}`, string(out))
	})

	t.Run("push_message returns status ok", func(t *testing.T) {
		out, err := p.Call(context.Background(), "push_message",
			[]byte(`{"product_code":"p","channel_codes":"c1","gateway":"huawei","title":"t","body":"b"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"status":"ok"}`, string(out))
	})

	t.Run("ad_plan_list full params", func(t *testing.T) {
		req, err := json.Marshal(map[string]interface{}{
			"product_code": "p", "page": 2.0, "page_rows": 20.0,
			"status": "DEPLOY_FAILD", "channel_id": "31",
			"ad_group_id": "ag", "plan_code": "PC9",
		})
		require.NoError(t, err)

		out, err := p.Call(context.Background(), "ad_plan_list", req)
		require.NoError(t, err)
		var res AdPlanListResult
		require.NoError(t, json.Unmarshal(out, &res))
		f := form()
		assert.Equal(t, "20", f.Get("pageRows"))
		assert.Equal(t, "DEPLOY_FAILD", f.Get("status"))
		assert.Equal(t, "31", f.Get("channelId"))
		assert.Equal(t, "ag", f.Get("adGroupId"))
		assert.Equal(t, "PC9", f.Get("planCode"))
	})

	t.Run("inner api error propagates", func(t *testing.T) {
		badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(Response{Status: false, Message: "quota exceeded"})
		}))
		defer badSrv.Close()

		p2 := NewProvider(nil)
		require.NoError(t, p2.Init(context.Background(), provider.ProviderConfig{
			Enabled: true,
			Config: map[string]interface{}{
				"open_id": "id", "open_key": "key", "api_base_url": badSrv.URL,
			},
		}))

		_, err := p2.Call(context.Background(), "channel_list", []byte(`{"product_code":"p"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "quota exceeded")
	})
}
