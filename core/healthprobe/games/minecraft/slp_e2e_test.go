package minecraft

// 真实服务端 E2E（todo:13 余量验收）：docker 起官方 vanilla 服务端
// （online-mode=false，零账号），SLP Checker 对真服务端验证——mock SLP
// 服务器只证协议实现自洽，真服务端才证「公开协议规范没读错」。
//
// 门禁：MC_E2E=1 显式开启（CI 与默认 go test 不跑——要拉镜像、下 server.jar、
// 起真服务端，属环境重依赖；migrations_integration_test 同款 opt-in 先例）。
// 覆盖范围 = SLP status 层；online-mode 加密链（Login 级）仍待账号（§6 卡点）。

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	e2eManifestURL = "https://launchermeta.mojang.com/mc/game/version_manifest_v2.json"
	// 容器内固定端口，宿主侧随机映射（并行跑不撞端口）。
	e2eContainerPort = "25565"
	e2eImagePrefix   = "eclipse-temurin:"
)

type e2eVersionMeta struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type e2eVersionDetail struct {
	JavaVersion struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
	Downloads struct {
		Server struct {
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
		} `json:"server"`
	} `json:"downloads"`
}

// TestSLPCheckerRealServerE2E 官方 vanilla 服务端（offline-mode）SLP 全链验证。
func TestSLPCheckerRealServerE2E(t *testing.T) {
	if os.Getenv("MC_E2E") != "1" {
		t.Skip("MC_E2E=1 to run the real vanilla server e2e (needs docker + network)")
	}
	for _, bin := range []string{"docker", "curl"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not on PATH: %v", bin, err)
		}
	}

	ctx := context.Background()
	versionID, javaMajor := e2eResolveVersion(t)
	jar := e2eEnsureServerJar(t, versionID)
	addr, stop := e2eStartVanilla(t, jar, versionID, javaMajor)
	defer stop()

	// Boot 完成 = 真服务端就绪；Checker 超时给足（首次 world gen 后仍可能慢）。
	chk := NewSLPChecker(addr, 10*time.Second)
	require.NoError(t, chk.Check(ctx), "SLP against real vanilla server must verify")

	d := chk.Details()
	assert.Equal(t, "done", d["phase"])
	assert.Equal(t, versionID, d["version"], "vanilla reports exact version id in status")
	assert.NotEmpty(t, d["serverProtocol"], "real server reports its protocol number")
	assert.Contains(t, d["players"], "/", "players summary form online/max")
	// 握手声明 47（1.8.9）而服务端是最新版：vanilla status 不校验版本（§5.5.6
	// 边界③的真服务端实证）。
	assert.NotEqual(t, "47", d["serverProtocol"], "modern server protocol != declared 47")
}

// ---- 环境装配 helpers（全部可独立失败，报错带环境诊断） ----

func e2eResolveVersion(t *testing.T) (string, int) {
	t.Helper()
	if v := os.Getenv("MC_E2E_VERSION"); v != "" {
		return v, e2eJavaMajor(t, v)
	}
	meta := e2eFetchJSON(t, e2eManifestURL)
	var manifest struct {
		Latest struct {
			Release string `json:"release"`
		} `json:"latest"`
		Versions []e2eVersionMeta `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(meta, &manifest))
	require.NotEmpty(t, manifest.Latest.Release, "manifest latest release missing")
	id := manifest.Latest.Release
	return id, e2eJavaMajor(t, id)
}

func e2eJavaMajor(t *testing.T, versionID string) int {
	t.Helper()
	meta := e2eFetchJSON(t, e2eVersionURL(t, versionID))
	var detail e2eVersionDetail
	require.NoError(t, json.Unmarshal(meta, &detail))
	require.NotZero(t, detail.JavaVersion.MajorVersion, "javaVersion missing for %s", versionID)
	return detail.JavaVersion.MajorVersion
}

func e2eVersionURL(t *testing.T, versionID string) string {
	t.Helper()
	meta := e2eFetchJSON(t, e2eManifestURL)
	var manifest struct {
		Versions []e2eVersionMeta `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(meta, &manifest))
	for _, v := range manifest.Versions {
		if v.ID == versionID && v.Type == "release" {
			return v.URL
		}
	}
	t.Fatalf("release version %q not in manifest", versionID)
	return ""
}

// e2eEnsureServerJar 下载官方 server.jar 到用户缓存目录（跨次复用，sha1 校验）。
// 下载走 Range 断点续传循环：本环境中间盒会掐 ~10MB 以上的长流（JSON 小包
// 正常、60MB 整包必然 unexpected EOF），按 Content-Range total 续传收尾，
// 每 8MB 一段天然落在掐断阈值内。
func e2eEnsureServerJar(t *testing.T, versionID string) string {
	t.Helper()
	meta := e2eFetchJSON(t, e2eVersionURL(t, versionID))
	var detail e2eVersionDetail
	require.NoError(t, json.Unmarshal(meta, &detail))
	require.NotEmpty(t, detail.Downloads.Server.URL, "server download url missing")
	wantSHA1 := detail.Downloads.Server.SHA1

	base, err := os.UserCacheDir()
	if err != nil {
		base = t.TempDir()
	}
	dir := filepath.Join(base, "croupier-mc-e2e", versionID)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	jar := filepath.Join(dir, "server.jar")

	if b, err := os.ReadFile(jar); err == nil {
		if sum := sha1.Sum(b); hex.EncodeToString(sum[:]) == wantSHA1 {
			t.Logf("using cached server.jar %s", jar)
			return jar
		}
		t.Log("cached server.jar sha1 mismatch, redownloading")
	}

	tmp := jar + ".part"
	_ = os.Remove(tmp)
	e2eDownloadResume(t, detail.Downloads.Server.URL, tmp, 8*1024*1024)

	body, err := os.ReadFile(tmp)
	require.NoError(t, err)
	sum := sha1.Sum(body)
	require.Equal(t, wantSHA1, hex.EncodeToString(sum[:]), "server.jar sha1 mismatch (piston meta pin)")
	require.NoError(t, os.Rename(tmp, jar))
	return jar
}

// e2eDownloadResume 分段 Range 下载到 dst（piston-data 支持字节区间）：
// 逐段请求 bytes=<off>-<off+chunk-1>，段内被掐断就从当前偏移续下一段；
// 末段以 206 Content-Range total 对齐收尾，最多 40 段重试后放弃。
func e2eDownloadResume(t *testing.T, url, dst string, chunk int64) {
	t.Helper()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	client := &http.Client{Timeout: 120 * time.Second}
	var total int64
	for attempt := 0; attempt < 40; attempt++ {
		off, err := f.Seek(0, io.SeekEnd)
		require.NoError(t, err)
		if total > 0 && off >= total {
			return // 已收齐
		}
		end := off + chunk - 1
		if total > 0 && end >= total {
			end = total - 1
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))
		resp, err := client.Do(req) //nolint:noctx // e2e helper, opt-in only
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		switch resp.StatusCode {
		case http.StatusPartialContent:
			start, last, size, ok := parseContentRange(resp.Header.Get("Content-Range"))
			if !ok || start != off {
				_ = resp.Body.Close()
				t.Fatalf("unexpected Content-Range %q at offset %d", resp.Header.Get("Content-Range"), off)
			}
			total = size
			_, _ = io.Copy(f, resp.Body)
			_ = resp.Body.Close()
			if last == off+resp.ContentLength-1 {
				// 整段读全（approx）；下一轮 Seek 校正真实落盘偏移
				continue
			}
		case http.StatusOK:
			if off != 0 {
				_ = resp.Body.Close()
				t.Fatal("server ignored Range on resume; cannot continue partial")
			}
			total = resp.ContentLength
			_, _ = io.Copy(f, resp.Body)
			_ = resp.Body.Close()
		case http.StatusRequestedRangeNotSatisfiable:
			// 偏移已过 total：要么收齐要么状态错乱，交给下一轮 Seek/total 判定
			_ = resp.Body.Close()
		default:
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
			t.Fatalf("download %s: status %d: %s", url, resp.StatusCode, body)
		}
	}
	t.Fatalf("download %s: resume attempts exhausted at %d/%d bytes", dst, mustSize(t, dst), total)
}

func parseContentRange(v string) (start, last, total int64, ok bool) {
	// bytes start-last/total
	var s, l, tot int64
	if _, err := fmt.Sscanf(v, "bytes %d-%d/%d", &s, &l, &tot); err != nil {
		return 0, 0, 0, false
	}
	return s, l, tot, true
}

func mustSize(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	require.NoError(t, err)
	return st.Size()
}

// e2eStartVanilla docker 起 offline-mode vanilla 服务端，等 boot 完成返回宿主地址。
func e2eStartVanilla(t *testing.T, jar, versionID string, javaMajor int) (string, func()) {
	t.Helper()

	// 宿主侧先占一个空闲端口再释放，给 -p 映射（窗口内被抢的概率可忽略）。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	hostPort := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	// 运行目录只放服务端写入物；server.jar 单文件只读绑定（缓存目录复用）。
	runDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "eula.txt"),
		[]byte("eula=true\n"), 0o644))
	props := "online-mode=false\nserver-port=" + e2eContainerPort +
		"\nlevel-type=minecraft\\:flat\ngenerate-structures=false\nview-distance=2\n" +
		"simulation-distance=1\nmotd=croupier slp e2e " + versionID + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "server.properties"),
		[]byte(props), 0o644))

	name := fmt.Sprintf("croupier-mc-e2e-%d-%d", os.Getpid(), time.Now().UnixNano())
	image := os.Getenv("MC_E2E_JAVA_IMAGE")
	if image == "" {
		image = fmt.Sprintf("%s%d-jre", e2eImagePrefix, javaMajor)
	}
	// --entrypoint java：兼容自带 ENTRYPOINT 的 JDK 镜像（如 gradle:*），
	// 无 ENTRYPOINT 的镜像同样适用；--user 宿主 uid/gid：容器内写的 world
	// 文件归测试用户，t.TempDir 清理才不会因 root 属主 unlinkat 失败。
	args := []string{"run", "--rm", "-d", "--name", name,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "HOME=/data",
		"-p", fmt.Sprintf("127.0.0.1:%d:%s", hostPort, e2eContainerPort),
		"-v", runDir + ":/data",
		"-v", jar + ":/data/" + filepath.Base(jar) + ":ro",
		"-w", "/data", "--entrypoint", "java", image,
		"-jar", "/data/" + filepath.Base(jar), "nogui"}
	cmd := exec.Command("docker", args...)
	out, err := execCmd(cmd)
	require.NoError(t, err, "docker run failed: %s", out)

	bootTimeout := 5 * time.Minute
	if v := os.Getenv("MC_E2E_BOOT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			bootTimeout = d
		}
	}
	deadline := time.Now().Add(bootTimeout)
	ready := false
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		logs, _ := execCmd(exec.Command("docker", "logs", "--tail", "50", name))
		if strings.Contains(logs, "Done (") {
			ready = true
			break
		}
		if strings.Contains(logs, "Failed to start") ||
			strings.Contains(logs, "Exception in thread \"main\"") {
			t.Fatalf("vanilla server failed during boot:\n%s", logs)
		}
		st, _ := execCmd(exec.Command("docker", "inspect", "-f", "{{.State.Status}}", name))
		if strings.TrimSpace(st) != "running" {
			logs, _ := execCmd(exec.Command("docker", "logs", "--tail", "100", name))
			t.Fatalf("vanilla server container %s before boot done (status=%s):\n%s", name, st, logs)
		}
	}
	if !ready {
		logs, _ := execCmd(exec.Command("docker", "logs", "--tail", "100", name))
		t.Fatalf("vanilla server not ready within %s:\n%s", bootTimeout, logs)
	}
	t.Logf("vanilla %s ready at 127.0.0.1:%d", versionID, hostPort)

	return fmt.Sprintf("127.0.0.1:%d", hostPort), func() {
		_, _ = execCmd(exec.Command("docker", "rm", "-f", name))
	}
}

func e2eFetchJSON(t *testing.T, url string) []byte {
	t.Helper()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url) //nolint:noctx // e2e helper, opt-in only
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s", url)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}

func execCmd(cmd *exec.Cmd) (string, error) {
	out, err := cmd.CombinedOutput()
	return string(out), err
}
