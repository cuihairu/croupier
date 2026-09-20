#!/usr/bin/env python3
"""六语言 demo 契约互比（基准 = Go demo sdks/go/examples/demo/main.go）。

背景（2026-09-20 线上事故）：六语言 demo 容器向同一 (game_id, env) scope 注册
同名函数，registry 最新注册胜出（UpsertContract 仅在语义相等时跳过写）。任一
语言的 schema 缺约束（如 js demo 的 i() 产出裸 {"type":"integer"} 缺 minimum）
都会在其他语言重连时把正确 schema 覆盖掉，已发布页面冻结的 digest 随之失配，
input_schema_stale 每次重启复现。40ea68422 / e8d0fb5a7 两轮人工对齐均漏槽位，
故收敛为机器互比：本脚本从六份 demo 源码解析契约槽位（input/output schema +
version/resource/operation/capability/execution/risk/approval），以 Go 为基准
逐一比对，任何漂移即非零退出。

解析器与源码形态耦合（有意的）：各 demo 的 schema 段保持「可文本解析」的
字面量/常量折叠形态是约定的一部分——把 schema 藏进运行时构造会让互比失明。
退出码：0 = 六语言逐槽一致；1 = 存在漂移；2 = 解析失败（文件缺失或某语言
解析出 0 个函数，视为解析器腐化，同样拦截）。
"""

from __future__ import annotations

import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

GO_DEMO = REPO / "sdks/go/examples/demo/main.go"
PY_DEMO = REPO / "sdks/python/examples/game_demo.py"
JS_DEMO = REPO / "sdks/js/examples/game_demo.ts"
JAVA_DEMO = REPO / "sdks/java/examples/game-demo/src/main/java/com/croupier/sdk/examples/GameDemo.java"
CS_DEMO = REPO / "sdks/csharp/examples/GameDemo/Program.cs"
CPP_DEMO = REPO / "sdks/cpp/examples/game_demo.cpp"


@dataclass
class Contract:
    version: str
    resource: str
    operation: str
    capability: str
    execution: str
    risk: str
    approval_required: bool
    approval_policy_key: str
    input_schema: object
    output_schema: object


# ---------------------------------------------------------------- 公共工具

def split_concat(expr: str) -> list[str]:
    """按顶层 '+' 切分字符串拼接表达式（引号内的 + 不切）。"""
    parts, buf, in_str, escape = [], [], False, False
    for ch in expr:
        if in_str:
            buf.append(ch)
            if escape:
                escape = False
            elif ch == "\\":
                escape = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
            buf.append(ch)
            continue
        if ch == "+":
            parts.append("".join(buf).strip())
            buf = []
            continue
        buf.append(ch)
    parts.append("".join(buf).strip())
    return [p for p in parts if p]


def unquote(tok: str) -> str:
    """C 系转义的双引号字符串字面量 → 原文。"""
    tok = tok.strip()
    if tok.startswith("`"):  # Go raw string
        return tok[1:-1]
    if not (tok.startswith('"') and tok.endswith('"')):
        raise ValueError(f"not a quoted literal: {tok[:60]}")
    body = tok[1:-1]
    return body.replace('\\"', '"').replace("\\\\", "\\")


def fold(expr: str, env: dict[str, str]) -> str:
    """常量折叠字符串拼接：字面量 + 标识符（查 env）。"""
    out = []
    for tok in split_concat(expr):
        if tok.startswith('"') or tok.startswith("`"):
            out.append(unquote(tok))
        else:
            ident = tok.strip()
            if ident not in env:
                raise ValueError(f"unknown identifier in concat: {ident}")
            out.append(env[ident])
    return "".join(out)


def parse_schema(text: str, where: str):
    try:
        return json.loads(text)
    except json.JSONDecodeError as e:
        raise ValueError(f"{where}: invalid JSON schema: {e}") from e


def join_statement(text: str, start: int) -> tuple[str, int]:
    """从 start 起取到语句结束分号（跨行拼接），返回 (语句, 结束位置)。"""
    end = text.index(";", start)
    return text[start:end], end


def balanced_span(text: str, start: int, open_ch: str, close_ch: str, where: str) -> str:
    """text[start:] 必须以 open_ch 开头，返回匹配的平衡括号内容（不含两端括号）。

    跳过字符串字面量与转义，嵌套计数——正则的懒惰匹配会在嵌套
    逗号/括号处错误切分（csharp switch 臂、cpp 多行 slot 实证），必须用扫描。
    """
    if not text.startswith(open_ch, start):
        raise ValueError(f"{where}: expected '{open_ch}' at offset {start}")
    depth, i, in_str, esc = 1, start + 1, False, False
    body_start = i
    while depth:
        if i >= len(text):
            raise ValueError(f"{where}: unbalanced '{open_ch}{close_ch}'")
        ch = text[i]
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
        else:
            if ch == '"':
                in_str = True
            elif ch == open_ch:
                depth += 1
            elif ch == close_ch:
                depth -= 1
                if depth == 0:
                    return text[body_start:i]
        i += 1
    raise ValueError(f"{where}: unbalanced '{open_ch}{close_ch}'")


def balanced_parens(text: str, start: int) -> str:
    return balanced_span(text, start, "(", ")", "parens")


def split_top_commas(s: str) -> list[str]:
    """按顶层逗号切分（括号深度 0、不在字符串内）。"""
    parts, buf, depth, in_str, esc = [], [], 0, False, False
    for ch in s:
        if in_str:
            buf.append(ch)
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
            buf.append(ch)
            continue
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append("".join(buf).strip())
            buf = []
            continue
        buf.append(ch)
    parts.append("".join(buf).strip())
    return parts


# ---------------------------------------------------------------- Go（基准）

GO_DESC = re.compile(
    r'ID:\s*"(?P<id>[\w.]+)",\s*Version:\s*"(?P<version>[^"]+)",\s*'
    r'Resource:\s*"(?P<resource>[\w.]+)",\s*Operation:\s*"(?P<op>[\w.]+)",\s*'
    r'Capability:\s*"(?P<cap>[\w_]+)",\s*Execution:\s*"(?P<exec>\w+)",\s*'
    r'Risk:\s*"(?P<risk>\w+)",\s*Enabled:\s*true,\s*'
    r'(?:ApprovalRequired:\s*true,\s*ApprovalPolicyKey:\s*"(?P<policy>[^"]+)",\s*)?'
    r'InputSchema:\s*`(?P<input>[^`]*)`,\s*'
    r'OutputSchema:\s*(?P<out>demoCollectionSchema\(\w+\)|`[^`]*`|\w+)',
)


def extract_go(text: str) -> dict[str, Contract]:
    env: dict[str, str] = {}
    for m in re.finditer(r'^\t(\w+)\s*=\s*(`[^`]*`)', text, re.M):
        env[m.group(1)] = unquote(m.group(2))

    out: dict[str, Contract] = {}
    for m in GO_DESC.finditer(text):
        schema = m.group("out")
        if schema.startswith("`"):
            output = unquote(schema)
        elif schema.startswith("demoCollectionSchema("):
            item = schema[len("demoCollectionSchema("):-1]
            output = fold(
                '`{"type":"object","properties":{"items":{"type":"array","items":`'
                f"+ {item} + "
                '`},"total":{"type":"integer"},"page":{"type":"integer"},"pageSize":{"type":"integer"}},"required":["items","total","page","pageSize"]}`',
                env,
            )
        else:
            output = env[schema]
        out[m.group("id")] = Contract(
            version=m.group("version"),
            resource=m.group("resource"),
            operation=m.group("op"),
            capability=m.group("cap"),
            execution=m.group("exec"),
            risk=m.group("risk"),
            approval_required=bool(m.group("policy")),
            approval_policy_key=m.group("policy") or "",
            input_schema=parse_schema(m.group("input"), f"go:{m.group('id')}.input"),
            output_schema=parse_schema(output, f"go:{m.group('id')}.output"),
        )
    return out


# ---------------------------------------------------------------- Python

PY_SLOT = re.compile(
    r'"(?P<id>[\w.]+)":\s*\{\s*"input":\s*json\.loads\(\'\'\'(?P<input>.*?)\'\'\'\),\s*'
    r'"output":\s*json\.loads\(\'\'\'(?P<output>.*?)\'\'\'\),?\s*\}',
    re.S,
)
PY_TUPLE = re.compile(
    r'\("(?P<id>[\w.]+)",\s*"(?P<resource>[\w.]+)",\s*"(?P<risk>\w+)",\s*"(?P<op>[\w.]+)",\s*'
    r'"(?P<cap>[\w_]+)",\s*"(?P<exec>\w+)",\s*(?P<policy>None|"[^"]*"),\s*\w+\(',
)


def extract_python(text: str) -> dict[str, Contract]:
    slots = {m.group("id"): (m.group("input"), m.group("output")) for m in PY_SLOT.finditer(text)}
    out: dict[str, Contract] = {}
    for m in PY_TUPLE.finditer(text):
        fn = m.group("id")
        if fn not in slots:
            continue
        policy = m.group("policy")
        policy = json.loads(policy) if policy != "None" else ""
        out[fn] = Contract(
            version="1.0.0",
            resource=m.group("resource"),
            operation=m.group("op"),
            capability=m.group("cap"),
            execution=m.group("exec"),
            risk=m.group("risk"),
            approval_required=bool(policy),
            approval_policy_key=policy,
            input_schema=parse_schema(slots[fn][0], f"python:{fn}.input"),
            output_schema=parse_schema(slots[fn][1], f"python:{fn}.output"),
        )
    return out


# ---------------------------------------------------------------- JS（与 Python 同构：JSON.parse 字面量槽位）

JS_CONST = re.compile(r"(?:export )?const ([A-Z_]+): Record<string, unknown> = JSON\.parse\('([^']*)'\);")
JS_EXPR_IN = re.compile(r'"(?P<id>[\w.]+)":\s*\{\s*input:\s*(?P<input>JSON\.parse\(\'[^\']*\'\)|[A-Z_]+|COLLECTION\([A-Z_]+\)),\s*'
                        r'output:\s*(?P<output>JSON\.parse\(\'[^\']*\'\)|[A-Z_]+|COLLECTION\([A-Z_]+\)),?\s*\}')
JS_TUPLE = re.compile(r'\["(?P<id>[\w.]+)",\s*"(?P<resource>[\w.]+)",\s*"(?P<risk>\w+)",\s*"(?P<op>[\w.]+)",\s*"(?P<cap>[\w_]+)",\s*\w+\(')


def extract_js(text: str) -> dict[str, Contract]:
    env: dict[str, str] = {}
    for m in JS_CONST.finditer(text):
        env[m.group(1)] = m.group(2)
    # COLLECTION(item) 折叠模板（与 Go demoCollectionSchema / C# Collection 同构；
    # js 运行时收 schema 对象经 JSON.stringify 嵌入，折叠时直接取常量文本）
    coll = re.search(
        r"const COLLECTION = \(item: Record<string, unknown>\) => JSON\.parse\('(.*?)' \+ JSON\.stringify\(item\) \+ '(.*)'\);",
        text,
    )
    if not coll:
        raise ValueError("js: COLLECTION helper not found")

    def eval_expr(expr: str) -> str:
        if expr.startswith("JSON.parse('"):
            return expr[len("JSON.parse('"):-2]
        if expr.startswith("COLLECTION("):
            inner = expr[len("COLLECTION("):-1]
            return coll.group(1) + env[inner] + coll.group(2)
        return env[expr]

    out: dict[str, Contract] = {}
    for m in JS_EXPR_IN.finditer(text):
        fn = m.group("id")
        out[fn] = Contract(
            version="1.0.0",
            resource="",  # 元组侧补齐
            operation="",
            capability="",
            execution="sync",  # js 注册循环固定 sync
            risk="",
            approval_required=False,
            approval_policy_key="",
            input_schema=parse_schema(eval_expr(m.group("input")), f"js:{fn}.input"),
            output_schema=parse_schema(eval_expr(m.group("output")), f"js:{fn}.output"),
        )
    for m in JS_TUPLE.finditer(text):
        fn = m.group("id")
        if fn not in out:
            continue
        risk = m.group("risk")
        # js 注册循环从 risk 推导 approval（danger → {id}.double_check），与 Go 显式声明等价
        out[fn].resource = m.group("resource")
        out[fn].operation = m.group("op")
        out[fn].capability = m.group("cap")
        out[fn].risk = risk
        out[fn].approval_required = risk == "danger"
        out[fn].approval_policy_key = f"{fn}.double_check" if risk == "danger" else ""
    return out


# ---------------------------------------------------------------- Java

JAVA_SLOT = re.compile(r'Map\.entry\("(?P<id>[\w.]+)",\s*new String\[\]\{\s*"(?P<input>(?:[^"\\]|\\.)*)",\s*"(?P<output>(?:[^"\\]|\\.)*)"\s*\}\)', re.S)
JAVA_TUPLE = re.compile(
    r'new Fn\("(?P<id>[\w.]+)",\s*"(?P<risk>\w+)",\s*"(?P<resource>[\w.]+)",\s*"(?P<op>[\w.]+)",\s*'
    r'"(?P<cap>[\w_]+)",\s*"(?P<exec>\w+)",\s*(?P<policy>null|"(?:[^"\\]|\\.)*"),\s*\w+\(',
)


def extract_java(text: str) -> dict[str, Contract]:
    slots = {m.group("id"): (unquote(f'"{m.group("input")}"'), unquote(f'"{m.group("output")}"')) for m in JAVA_SLOT.finditer(text)}
    out: dict[str, Contract] = {}
    for m in JAVA_TUPLE.finditer(text):
        fn = m.group("id")
        if fn not in slots:
            continue
        policy = "" if m.group("policy") == "null" else unquote(m.group("policy"))
        out[fn] = Contract(
            version="1.0.0",
            resource=m.group("resource"),
            operation=m.group("op"),
            capability=m.group("cap"),
            execution=m.group("exec"),
            risk=m.group("risk"),
            approval_required=bool(policy),
            approval_policy_key=policy,
            input_schema=parse_schema(slots[fn][0], f"java:{fn}.input"),
            output_schema=parse_schema(slots[fn][1], f"java:{fn}.output"),
        )
    return out


# ---------------------------------------------------------------- C#

def extract_csharp(text: str) -> dict[str, Contract]:
    env: dict[str, str] = {}
    for m in re.finditer(r'static readonly string (\w+) =', text):
        stmt, _ = join_statement(text, m.end())
        env[m.group(1)] = fold(stmt, env)

    def call(name: str, args: str) -> str:
        if name == "BuildObj":
            parts = re.findall(r'"((?:[^"\\]|\\.)*)"|new\[\]\s*\{([^}]*)\}|(\w+)', args)
            # 简化：第一参数是字符串表达式（可能为标识符），第二参数（可选）为 new[] {...}
            m2 = re.match(r'^(.*?)(?:,\s*new\[\]\s*\{([^}]*)\})?$', args, re.S)
            props_expr, req = m2.group(1), m2.group(2)
            props = fold(props_expr, env)
            schema = '{"type":"object","properties":' + props + '}'
            if req:
                keys = re.findall(r'"([\w.]+)"', req)
                schema = schema[:-1] + ',"required":' + json.dumps(keys, separators=(",", ":")) + '}'
            return schema
        if name == "Collection":
            # 参数可能是嵌套调用（Collection(BuildObj(X))）——递归求值
            item = eval_csharp_expr(args, call, env)
            return (
                '{"type":"object","properties":{"items":{"type":"array","items":' + item +
                '},"total":{"type":"integer"},"page":{"type":"integer"},"pageSize":{"type":"integer"}},"required":["items","total","page","pageSize"]}'
            )
        raise ValueError(f"csharp: unknown helper {name}")

    slots: dict[str, tuple[str, str]] = {}
    switch = re.search(r'SchemasFor\(string id\) => id switch\s*\{(.*)\n    \};', text, re.S)
    if not switch:
        raise ValueError("csharp: SchemasFor switch not found")
    # 臂内表达式含嵌套括号/字符串逗号，懒惰正则会错误切分——按平衡括号扫描取参
    for m in re.finditer(r'"(?P<id>[\w.]+)" => \(', switch.group(1)):
        args = split_top_commas(balanced_parens(switch.group(1), m.end() - 1))
        if len(args) != 2:
            raise ValueError(f"csharp: switch arm {m.group('id')} 应为 (input, output)")
        slots[m.group("id")] = (
            eval_csharp_expr(args[0], call, env),
            eval_csharp_expr(args[1], call, env),
        )

    out: dict[str, Contract] = {}
    for m in re.finditer(
        r'\("(?P<id>[\w.]+)",\s*"(?P<risk>\w+)",\s*"(?P<resource>[\w.]+)",\s*"(?P<op>[\w.]+)",\s*'
        r'"(?P<cap>[\w_]+)",\s*"(?P<exec>\w+)",\s*(?:async|H\.|\w+\()', text,
    ):
        fn = m.group("id")
        if fn not in slots:
            continue
        risk = m.group("risk")
        # csharp 注册循环从 risk 推导 approval（danger → {id}.double_check）
        out[fn] = Contract(
            version="1.0.0",
            resource=m.group("resource"),
            operation=m.group("op"),
            capability=m.group("cap"),
            execution=m.group("exec"),
            risk=risk,
            approval_required=risk == "danger",
            approval_policy_key=f"{fn}.double_check" if risk == "danger" else "",
            input_schema=parse_schema(slots[fn][0], f"csharp:{fn}.input"),
            output_schema=parse_schema(slots[fn][1], f"csharp:{fn}.output"),
        )
    return out


def eval_csharp_expr(expr: str, call, env) -> str:
    expr = expr.strip()
    m = re.match(r'^(BuildObj|Collection)\((.*)\)$', expr, re.S)
    if m:
        return call(m.group(1), m.group(2))
    if expr.startswith('"'):
        return unquote(expr)
    return env[expr]


# ---------------------------------------------------------------- C++

def extract_cpp(text: str) -> dict[str, Contract]:
    env: dict[str, str] = {}
    for m in re.finditer(r'static const char\* (\w+) = ("(?:[^"\\]|\\.)*");', text):
        env[m.group(1)] = unquote(m.group(2))

    def cpp_fold(expr: str, extra: dict[str, str] | None = None) -> str:
        merged = dict(env)
        if extra:
            merged.update(extra)
        # std::string(X) 是 C++ 显式构造，折叠时视同标识符 X
        return fold(re.sub(r"std::string\((\w+)\)", r"\1", expr), merged)

    # 布尔参数化辅助：player_fields_schema(bool) —— 局部 required 变量 + 三元 + 返回拼接
    pfm = re.search(r"static std::string player_fields_schema\(bool \w+\) \{(.*?)\n\}", text, re.S)
    if not pfm:
        raise ValueError("cpp: player_fields_schema not found")
    pf_body = pfm.group(1)
    tern = re.search(r'std::string (\w+) = \w+ \? ("(?:[^"\\]|\\.)*") : ("(?:[^"\\]|\\.)*");', pf_body)
    if not tern:
        raise ValueError("cpp: player_fields_schema 三元未匹配")
    pf_var = tern.group(1)
    # 外层捕获已吃掉结尾 \n}，return 模板直接取 return 之后到语句尾
    ret_i = pf_body.find("return")
    if ret_i < 0:
        raise ValueError("cpp: player_fields_schema return 未找到")
    pf_template = pf_body[ret_i + len("return"):].strip().rstrip(";").strip()

    # demo_schema_for 函数体内的 const std::string 常量与无参 lambda
    fnbody = re.search(r"demo_schema_for\(const std::string& id\) \{(.*?)\n\}", text, re.S)
    if not fnbody:
        raise ValueError("cpp: demo_schema_for not found")
    body_text = fnbody.group(1)
    for m in re.finditer(r"const std::string (\w+) = (.*?);(?=\n)", body_text, re.S):
        env[m.group(1)] = cpp_fold(m.group(2))
    for m in re.finditer(r"auto (\w+) = \[\]\(\) \{\s*return (.*?);\s*\};", body_text, re.S):
        env[m.group(1)] = cpp_fold(m.group(2))

    # collection(item) lambda：带参模板
    coll = re.search(r"auto collection = \[\]\(const std::string& (\w+)\) \{\s*return (.*?);\s*\};", body_text, re.S)
    if not coll:
        raise ValueError("cpp: collection lambda not found")

    def eval_expr(expr: str) -> str:
        ex = expr.strip()
        m = re.match(r"^(\w+)\((.*)\)$", ex, re.S)
        if m:
            name, arg = m.group(1), m.group(2).strip()
            if name == "collection":
                return cpp_fold(coll.group(2), {coll.group(1): eval_expr(arg)})
            if name == "player_fields_schema":
                lit = tern.group(2) if arg == "true" else tern.group(3)
                return cpp_fold(pf_template, {pf_var: unquote(lit)})
            if name in env:  # 无参 lambda 以 () 调用
                return env[name]
            raise ValueError(f"cpp: unknown call {name}({arg[:40]})")
        # 单字面量 / 跨常量拼接 / 裸常量标识符统一走折叠
        return cpp_fold(ex)

    # slot 对是多行 {input, output}，字符串里满是花括号/逗号——必须平衡扫描
    slots: dict[str, tuple[str, str]] = {}
    for m in re.finditer(r'if \(id == "(?P<id>[\w.]+)"\) return \{', body_text):
        pair = split_top_commas(balanced_span(body_text, m.end() - 1, "{", "}", f"cpp slot {m.group('id')}"))
        if len(pair) != 2:
            raise ValueError(f"cpp: slot {m.group('id')} 应为 (input, output)")
        slots[m.group("id")] = (eval_expr(pair[0]), eval_expr(pair[1]))

    out: dict[str, Contract] = {}
    for m in re.finditer(
        r'reg\("(?P<id>[\w.]+)",\s*"(?P<risk>\w+)",\s*"(?P<resource>[\w.]+)",\s*"(?P<op>[\w.]+)",\s*'
        r'"(?P<cap>[\w_]+)",\s*"(?P<exec>\w+)",', text,
    ):
        fn = m.group("id")
        if fn not in slots:
            continue
        # approval 来自 reg() 显式尾参（非 risk 推导）：平衡扫描取整个调用，
        # 尾部若为字符串字面量即 approval_policy
        call_args = balanced_parens(text, m.start() + len("reg"))
        pm = re.search(r',\s*"(?P<policy>(?:[^"\\]|\\.)*)"\s*$', call_args)
        policy = pm.group("policy") if pm else ""
        out[fn] = Contract(
            version="1.0.0",
            resource=m.group("resource"),
            operation=m.group("op"),
            capability=m.group("cap"),
            execution=m.group("exec"),
            risk=m.group("risk"),
            approval_required=bool(policy),
            approval_policy_key=policy,
            input_schema=parse_schema(slots[fn][0], f"cpp:{fn}.input"),
            output_schema=parse_schema(slots[fn][1], f"cpp:{fn}.output"),
        )
    return out


# ---------------------------------------------------------------- 比对

FIELDS = [
    ("version", lambda c: c.version),
    ("resource", lambda c: c.resource),
    ("operation", lambda c: c.operation),
    ("capability", lambda c: c.capability),
    ("execution", lambda c: c.execution),
    ("risk", lambda c: c.risk),
    ("approvalRequired", lambda c: c.approval_required),
    ("approvalPolicyKey", lambda c: c.approval_policy_key),
    ("inputSchema", lambda c: c.input_schema),
    ("outputSchema", lambda c: c.output_schema),
]

LANGS = [
    ("go", GO_DEMO, extract_go),
    ("python", PY_DEMO, extract_python),
    ("js", JS_DEMO, extract_js),
    ("java", JAVA_DEMO, extract_java),
    ("csharp", CS_DEMO, extract_csharp),
    ("cpp", CPP_DEMO, extract_cpp),
]


def fmt(v) -> str:
    if isinstance(v, (dict, list)):
        s = json.dumps(v, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        return s if len(s) <= 160 else s[:157] + "..."
    return str(v)


def main() -> int:
    extracted: dict[str, dict[str, Contract]] = {}
    parse_failed = False
    for name, path, fn in LANGS:
        if not path.exists():
            print(f"[parse-error] {name}: {path} 不存在", file=sys.stderr)
            parse_failed = True
            continue
        try:
            contracts = fn(path.read_text(encoding="utf-8"))
        except (ValueError, AttributeError) as e:
            print(f"[parse-error] {name}: {e}", file=sys.stderr)
            parse_failed = True
            continue
        if not contracts:
            print(f"[parse-error] {name}: 解析出 0 个函数——解析器与源码形态脱节，禁止忽略", file=sys.stderr)
            parse_failed = True
            continue
        extracted[name] = contracts
        print(f"[ok] {name}: {len(contracts)} 个契约槽位")

    if parse_failed:
        return 2

    baseline = extracted["go"]
    failures = 0
    # 函数集完整性：被 ≥2 个语言注册的函数视为共享契约槽位，必须六语言同步
    # 存在——两方同名不同形正是契约振荡源；单语言独有无振荡面，仅记 note。
    all_fns = sorted(set().union(*(set(c) for c in extracted.values())))
    for fn in all_fns:
        holders = [name for name, _, _ in LANGS if fn in extracted[name]]
        if len(holders) == 1:
            print(f"[note] {fn}: 仅 {holders[0]} 存在（单侧独有，无振荡面）")
        elif len(holders) < len(LANGS):
            failures += 1
            missing = ",".join(name for name, _, _ in LANGS if name not in holders)
            print(f"[drift] {fn}: 共享槽位必须六语言同步——已注册 {','.join(holders)}；缺失 {missing}")
    for name, _, _ in LANGS[1:]:
        lang = extracted[name]
        shared = sorted(set(baseline) & set(lang))
        print(f"\n=== {name} vs go：共同 {len(shared)} 槽位")
        for fn in shared:
            for field, get in FIELDS:
                b, l = get(baseline[fn]), get(lang[fn])
                if b != l:
                    failures += 1
                    print(f"[drift] {name}/{fn}.{field}\n  go : {fmt(b)}\n  {name}: {fmt(l)}")

    if failures:
        print(f"\nFAIL: {failures} 处契约漂移——改任何一份 demo 契约必须六语言同步"
              f"（基准 = sdks/go/examples/demo/main.go）")
        return 1
    print("\nPASS: 六语言 demo 契约逐槽一致")
    return 0


if __name__ == "__main__":
    sys.exit(main())
