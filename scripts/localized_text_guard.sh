#!/usr/bin/env bash
# LocalizedText contract guard.
#
# Contract (see CLAUDE.md "Localized Text Contract (Mandatory)" and
# docs/architecture/localized-text-contract.md): localized fields are keyed by
# BCP47 locale ("zh-CN"/"en-US"), normalized once at the service boundary
# (normalizeLocalizedText) and rendered only through
# web/src/utils/localizedText.ts#localizedText.
#
# This guard fails when:
#   1. a second LocalizedText type/interface definition appears outside
#      web/src/types/dashboard.ts (self-invented localizations drift back in)
#   2. a component renders localized values through inline key chains
#      (e.g. value['zh-CN'] || ..., value?.zh || value?.en) instead of the util
#   3. a module other than the sanctioned normalizers writes short keys
#      ({ zh: ..., en: ... }) into localized DTO fields
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
FAIL=0

note() { echo "LOCALIZED_TEXT_GUARD: $1"; }
fail() { echo "LOCALIZED_TEXT_GUARD_FAILED: $1" >&2; FAIL=1; }

# ---------------------------------------------------------------------------
# 1. Only one LocalizedText definition.
# ---------------------------------------------------------------------------
# `import type` / re-export 转发合法；仅禁止 type/interface 重新定义
while IFS=: read -r file line _match; do
  case "$file" in
    web/src/types/dashboard.ts) continue ;;
    *) fail "$file:$line redefines LocalizedText; the only definition lives in web/src/types/dashboard.ts" ;;
  esac
done < <(rg -n "^\s*(export )?type LocalizedText = \{|^\s*(export )?interface LocalizedText\b" web/src --glob '!*.test.*' || true)

# I18N-style aliases count as second definitions too (Directory's I18N is the
# sanctioned alias; anything else must import LocalizedText instead).
while IFS=: read -r file line _match; do
  case "$file" in
    web/src/pages/Functions/Directory/types.ts) continue ;;
    *) fail "$file:$line declares a standalone localization type ($(_match)); alias or import LocalizedText instead" ;;
  esac
done < <(rg -n "^\s*(export )?type \w*I18N\w* = \{[^}]*zh\??:" web/src --glob '!*.test.*' || true)

# ---------------------------------------------------------------------------
# 2. No inline locale-key render chains outside the util itself.
# ---------------------------------------------------------------------------
while IFS=: read -r file line match; do
  case "$file" in
    # 契约 util 自身与 service 归一层是仅有的合法直接读取点
    web/src/utils/localizedText.ts) continue ;;
    web/src/services/api/functions-enhanced.ts) continue ;;
    *) fail "$file:$line inline localized pick ($match); use localizedText(value, locale, fallback) from web/src/utils/localizedText" ;;
  esac
done < <(rg -n "\['zh-CN'\] \|\||\['en-US'\] \|\||\?\.zh \|\||\?\.en \|\|" \
          web/src/pages web/src/components web/src/services web/src/utils \
          --glob '!*.test.*' --glob '!localizedText.ts' 2>/dev/null || true)

# ---------------------------------------------------------------------------
# 3. Short keys must not be written into localized DTO fields.
#    The sanctioned normalizer is functions-enhanced.ts (legacy input only);
#    string->both-locales expansion is allowed there and nowhere else.
# ---------------------------------------------------------------------------
while IFS=: read -r file line match; do
  case "$file" in
    web/src/services/api/functions-enhanced.ts) continue ;;
    *) fail "$file:$line fabricates short localized keys ($match); emit { 'zh-CN', 'en-US' } via normalizeLocalizedText" ;;
  esac
done < <(rg -n "\{\s*(\.\.\.\()?\{?\s*zh:|\{ zh: '[^']*', en: " \
          web/src/services --glob '!*.test.*' --glob '!functions-enhanced.ts' 2>/dev/null || true)

if [ "$FAIL" -ne 0 ]; then
  echo "LocalizedText contract violations found. See docs/architecture/localized-text-contract.md"
  exit 1
fi
note "PASS: single LocalizedText definition, single render path, no short-key fabrication"
