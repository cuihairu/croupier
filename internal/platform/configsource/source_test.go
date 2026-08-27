package configsource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/cuihairu/croupier/internal/model"
)

func TestCleanPath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: ""},
		{in: "/", want: ""},
		{in: "a/b", want: "a/b"},
		{in: "/a//b/", want: "a/b"},
		{in: "a/../b", wantErr: true},
		{in: "a\\b", wantErr: true},
		{in: "a\x00b", wantErr: true},
	}
	for _, c := range cases {
		got, err := cleanPath(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("cleanPath(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("cleanPath(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestMaskSecrets(t *testing.T) {
	cfg := `{"addr":"1.2.3.4:6379","password":"hunter2","token":"abc","dsn":"user:pass@tcp(1.2.3.4)/db"}`
	masked := MaskSecrets(cfg)
	out := map[string]interface{}{}
	if err := json.Unmarshal([]byte(masked), &out); err != nil {
		t.Fatal(err)
	}
	if out["password"] != "******" || out["token"] != "******" {
		t.Errorf("credentials not masked: %s", masked)
	}
	dsn, _ := out["dsn"].(string)
	if dsn == "user:pass@tcp(1.2.3.4)/db" {
		t.Errorf("dsn password not masked: %s", dsn)
	}
	if addr, _ := out["addr"].(string); addr != "1.2.3.4:6379" {
		t.Errorf("non-secret field should survive masking: %s", masked)
	}
}

func TestRedisSource_ListReadWrite(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	mr.Set("cfg:gameplay/item", `{"id":1}`)
	mr.Set("cfg:gameplay/hero/skin", "1")
	mr.Set("cfg:runtime/switch", "on")
	mr.Set("other:key", "x")

	src, err := New(testBinding("redis", fmt.Sprintf(`{"addr":"%s","prefix":"cfg:"}`, mr.Addr())))
	if err != nil {
		t.Fatal(err)
	}
	root, err := src.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	// 根目录: gameplay/, runtime/（hero 在 gameplay 之下，不在根）
	wantDirs := map[string]bool{}
	for _, e := range root {
		if e.Dir {
			wantDirs[e.Name] = true
		}
	}
	if !wantDirs["gameplay"] || !wantDirs["runtime"] || wantDirs["hero"] {
		t.Errorf("root dirs wrong: %+v", root)
	}

	sub, err := src.List(ctx, "gameplay")
	if err != nil {
		t.Fatal(err)
	}
	// gameplay/ 下：目录 hero/ + 文件 item
	if len(sub) != 2 || !sub[0].Dir || sub[0].Name != "hero" || sub[1].Name != "item" || sub[1].Dir {
		t.Errorf("gameplay children = %+v", sub)
	}

	val, err := src.Read(ctx, "runtime/switch")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "on" {
		t.Errorf("read = %q", val)
	}

	// 应急写回
	ws := src.(WritableSource)
	if err := ws.Write(ctx, "runtime/switch", []byte("off"), "test"); err != nil {
		t.Fatal(err)
	}
	val, err = src.Read(ctx, "runtime/switch")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "off" {
		t.Errorf("after write read = %q", val)
	}

	// prefix 外的 key 不可见
	if _, err := src.Read(ctx, "../other/key"); err == nil {
		t.Errorf("traversal should be rejected")
	}
}

func TestNacosSource_ListReadWrite(t *testing.T) {
	items := []nacosConfigItem{
		{DataID: "gameplay/item.json", Group: "DEFAULT_GROUP"},
		{DataID: "gameplay/hero.json", Group: "DEFAULT_GROUP"},
		{DataID: "runtime/switch.yaml", Group: "DEFAULT_GROUP"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v1/cs/configs":
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"pageItems": items})
				return
			}
			// publish
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(400)
				return
			}
			if r.Form.Get("dataId") == "runtime/switch.yaml" && r.Form.Get("content") != "" {
				_, _ = w.Write([]byte("true"))
				return
			}
			w.WriteHeader(500)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	src, err := New(testBinding("nacos", fmt.Sprintf(`{"endpoint":%q}`, srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	root, err := src.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	dirs := map[string]bool{}
	for _, e := range root {
		if e.Dir {
			dirs[e.Name] = true
		}
	}
	if !dirs["gameplay"] || !dirs["runtime"] {
		t.Errorf("root dirs = %+v", root)
	}

	sub, err := src.List(context.Background(), "gameplay")
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 2 {
		t.Errorf("gameplay children = %+v", sub)
	}

	ws := src.(WritableSource)
	if err := ws.Write(context.Background(), "runtime/switch.yaml", []byte("on: true"), "test"); err != nil {
		t.Fatal(err)
	}
}

func testBinding(typ, cfg string) *model.ConfigSourceBinding {
	return &model.ConfigSourceBinding{Type: typ, Config: cfg, GameID: "demo", Env: "prod"}
}
