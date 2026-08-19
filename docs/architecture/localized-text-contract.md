# LocalizedText 契约

状态：强制（CI 由 `scripts/localized_text_guard.sh` 守护）

## 背景

本地化字段曾同时存在三种形态：后端下发的 BCP47 key（`{ "zh-CN": ..., "en-US": ... }`）、
前端两处 service 自造的短 key（`{ zh, en }`）、以及裸字符串。组件里存在八份以上
复制粘贴的取值链且回退顺序互相矛盾，最终导致调用页把 `{en-US, zh-CN}` 对象直接
渲染为 React child 而崩溃。本契约终结该漂移。

## 契约

1. **唯一定义**：`spec.LocalizedText = map[BCP47-locale]string`（Go，
   `internal/dashboard/spec/types.go`）与 `web/src/types/dashboard.ts` 的
   `LocalizedText`。key 必须是 `"zh-CN"` / `"en-US"`。
   禁止任何模块声明第二份本地化类型或自造短 key（`zh` / `en` / `zh_cn`）。

2. **唯一归一层**：service 边界统一经
   `normalizeLocalizedText`（`web/src/services/api/functions-enhanced.ts`）。
   任何输入形态（BCP47、遗留短 key、裸字符串）统一输出 `{ "zh-CN", "en-US" }`。
   遗留短 key 只允许在该函数内读取兜底，不允许在任何出口产生。

3. **唯一渲染路径**：组件渲染必须调用
   `web/src/utils/localizedText.ts` 的 `localizedText(value, locale, fallback)`。
   禁止在组件内内联 `value['zh-CN'] || ...` / `value?.zh || value?.en` 之类取值链。

## 守护

`scripts/localized_text_guard.sh` 在 CI（CI - Dashboard / CI - Core）中执行，失败即红：

- 重新定义 `LocalizedText` type/interface（`import type` 转发合法）
- 组件内的内联 locale 取值链（契约 util 与归一层自身除外）
- 在 service 出口伪造短 key

## Review 检查项

新增 API/DTO 的本地化字段按上述三条检查；出现第二份 LocalizedText 定义、
组件内取值链或新的 normalize 实现均视为 review failure。
