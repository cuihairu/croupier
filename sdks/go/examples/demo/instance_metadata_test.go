package main

import "testing"

// CROUPIER_INSTANCE_METADATA 是 demo 侧唯一的实例元数据入口：
// JSON 对象与 k=v 简写都要能解析，残缺输入必须报错而不是静默丢一半。
func TestParseInstanceMetadata(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "empty means no metadata",
			raw:  "   ",
			want: nil,
		},
		{
			name: "json object",
			raw:  `{"serverId":"s1","pod":"game-7c4d"}`,
			want: map[string]string{"serverId": "s1", "pod": "game-7c4d"},
		},
		{
			name: "key value shorthand",
			raw:  "serverId=s1, pod=game-7c4d ,region=cn-north",
			want: map[string]string{"serverId": "s1", "pod": "game-7c4d", "region": "cn-north"},
		},
		{
			name: "value keeps inner equals sign",
			raw:  "dsn=user=alice@/db",
			want: map[string]string{"dsn": "user=alice@/db"},
		},
		{
			name:    "pair without value is rejected",
			raw:     "serverId",
			wantErr: true,
		},
		{
			name:    "malformed json is rejected",
			raw:     `{"serverId":`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseInstanceMetadata(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %v", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatalf("key %q = %q, want %q", key, got[key], want)
				}
			}
		})
	}
}
