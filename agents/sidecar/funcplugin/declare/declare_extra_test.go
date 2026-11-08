package declare

// declare.go 补测：ParseFunctionOwner 的全形态表（非插件前缀/空尾段/
// 无点分隔/正常提取）、ValidatePluginName 的连字符与边界长度闭集、
// Validate 的插件名校验透传、validateObjectSchema 的非对象顶层 JSON
// （json.Valid 放行、结构化解码拒绝）、Metadata 的校验透传/默认低危/
// 版本号兜底，以及 risk→FunctionSecurity 枚举与 behaviorMode 的四档
// 映射（全经 Metadata 端到端断言）。
//
// 无诚实跳过：本包是纯函数契约面，全臂可达。

import (
	"strings"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

func TestParseFunctionOwnerShapes(t *testing.T) {
	for _, pair := range []struct {
		in, want string
	}{
		{"game.item.get", ""},   // 非插件前缀
		{"pluginology.x.y", ""}, // 前缀非完整段（CutPrefix 是纯串前缀）
		{"plugin.", ""},         // rest 为空
		{"plugin.shop", ""},     // 无点分隔
		{"plugin..x", ""},       // 空段名
		{"plugin.shop.item.get", "shop"},
	} {
		if got := ParseFunctionOwner(pair.in); got != pair.want {
			t.Fatalf("ParseFunctionOwner(%q) = %q, want %q", pair.in, got, pair.want)
		}
	}
}

func TestValidatePluginNameBoundaries(t *testing.T) {
	for _, name := range []string{"ab", "9lives", "shop-api", strings.Repeat("a", 64)} {
		if err := ValidatePluginName(name); err != nil {
			t.Fatalf("ValidatePluginName(%q) = %v", name, err)
		}
	}
	for _, name := range []string{
		"a",                     // 过短
		strings.Repeat("a", 65), // 过长
		"Ab",                    // 大写
		"-ab",                   // 前导连字符
		"shop_api",              // 下划线不在字符集
		"sh op",                 // 空格
	} {
		if err := ValidatePluginName(name); err == nil {
			t.Fatalf("ValidatePluginName(%q) accepted", name)
		}
	}
}

func TestValidatePropagatesPluginNameCheck(t *testing.T) {
	// 合法描述符 + 非法插件名 → 插件名校验先行收口
	if err := Validate("AB", validDescriptor()); err == nil ||
		!strings.Contains(err.Error(), "must match [a-z0-9][a-z0-9-]*") {
		t.Fatalf("validate plugin name = %v", err)
	}
}

func TestValidateObjectSchemaRejectsNonObjectJSON(t *testing.T) {
	// json.Valid 放行的数组字面量：结构化解码进 type 头时拒绝
	d := validDescriptor()
	d.ParamSchema = "[]"
	if err := Validate("httpget", d); err == nil ||
		!strings.Contains(err.Error(), "param_schema: json") {
		t.Fatalf("array param schema = %v", err)
	}
}

func TestMetadataPropagatesValidationFailure(t *testing.T) {
	d := validDescriptor()
	d.ID = "game.item.get" // 越出插件命名空间
	if _, err := Metadata("httpget", "1.0.0", d); err == nil ||
		!strings.Contains(err.Error(), "plugin.httpget.") {
		t.Fatalf("metadata validate passthrough = %v", err)
	}
}

func TestMetadataDefaultsRiskAndVersion(t *testing.T) {
	d := validDescriptor()
	d.RiskLevel = ""
	md, err := Metadata("httpget", "", d)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if md.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_LOW {
		t.Fatalf("default risk = %v", md.Security.RiskLevel)
	}
	if md.Behavior.Mode != functionv1.FunctionBehavior_MODE_QUERY || !md.Behavior.Idempotent {
		t.Fatalf("low risk behavior = %v/%v", md.Behavior.Mode, md.Behavior.Idempotent)
	}
	if md.Version != "0.0.0" {
		t.Fatalf("default version = %q", md.Version)
	}
	if !md.Security.AuditLog {
		t.Fatal("plugin functions must always be audit-logged")
	}
}

func TestMetadataMapsRiskLevelsEndToEnd(t *testing.T) {
	for _, pair := range []struct {
		risk string
		want functionv1.FunctionSecurity_RiskLevel
	}{
		{RiskMedium, functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
		{RiskHigh, functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		{RiskDanger, functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
	} {
		d := validDescriptor()
		d.RiskLevel = pair.risk
		md, err := Metadata("httpget", "1.0.0", d)
		if err != nil {
			t.Fatalf("metadata(%s): %v", pair.risk, err)
		}
		if md.Security.RiskLevel != pair.want {
			t.Fatalf("risk %s mapped to %v, want %v", pair.risk, md.Security.RiskLevel, pair.want)
		}
		// 非低危 → 非幂等 command（保守默认，直到描述符长出显式 behavior 字段）
		if md.Behavior.Mode != functionv1.FunctionBehavior_MODE_COMMAND || md.Behavior.Idempotent {
			t.Fatalf("risk %s behavior = %v/%v", pair.risk, md.Behavior.Mode, md.Behavior.Idempotent)
		}
	}
}
