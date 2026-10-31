#!/usr/bin/env python3
"""Validate source-reviewed architecture assets, scoped links and TODO DAG."""
from pathlib import Path
import re
import xml.etree.ElementTree as ET
R=Path(__file__).resolve().parents[2];errors=[]
for p in (R/'docs/public/architecture').glob('*.svg'):
 try:ET.parse(p)
 except ET.ParseError as e:errors.append(str(e))
for lang in ('en','zh'):
 base=R/('docs/architecture' if lang=='en' else 'docs/zh/architecture')
 analytics=R/('docs/analytics' if lang=='en' else 'docs/zh/analytics')
 for p in (base/'index.md',base/'runtime-topology.md',analytics/'index.md',analytics/'core-and-external.md'):
  for u in re.findall(r'\]\(([^)#]+)(?:#[^)]*)?\)',p.read_text()):
   if u.startswith(('http:','https:')):continue
   target=R/'docs/public'/u.lstrip('/') if u.endswith('.svg') else p.parent/u
   if not any(x.exists() for x in [target,Path(str(target)+'.md'),target/'index.md']):errors.append(f'{p}: broken {u}')
 overview=(analytics/'index.md').read_text()
 if f'/architecture/analytics-{lang}.svg' not in overview or '```mermaid' in overview:errors.append(str(analytics)+': missing SVG or legacy Mermaid overview')
 for p in (R/('docs/index.md' if lang=='en' else 'docs/zh/index.md'),base/'index.md',base/'runtime-topology.md'):
  if f'<ArchitectureDiagram lang="{lang}" />' not in p.read_text():errors.append(str(p)+': inconsistent diagram')
 for src in re.findall(r'`((?:internal|core|agents|cmd|tools|mobile|web|docker|configs)/[^`\s]+)`',(base/'runtime-topology.md').read_text()):
  src=src.split(':')[0];variants=[src]
  if '{' in src:
   pre,rest=src.split('{',1);choices,suffix=rest.split('}',1);variants=[pre+x+suffix for x in choices.split(',')]
  for v in variants:
   if '*' not in v and not (R/v).exists():errors.append('Missing evidence '+v)
 readme=R/('README.md' if lang=='en' else 'README.zh.md')
 if f'docs/public/architecture/system-{lang}.svg' not in readme.read_text():errors.append(str(readme)+': missing SVG')
text=(R/'todo.md').read_text();parts=re.split(r'(?m)^- \[([ x])\] \*\*',text)[1:];graph={}
for i in range(0,len(parts),2):
 state,body=parts[i:i+2];m=re.match(r'([A-Z]+-\d{3}) · P[012] · [^·]+ ·',body)
 if not m:errors.append('Task structure: '+body[:60]);continue
 key=m[1]
 if key in graph:errors.append('Duplicate '+key)
 d=re.search(r'依赖：([^。]+)',body);graph[key]=re.findall(r'[A-Z]+-\d{3}',d[1]) if d else []
 for field in ('依赖：','交付与验收：','状态／条件：'):
  if field not in body:errors.append(key+' lacks '+field)
 if state=='x':
  evidence=re.search(r'完成证据：([^\n]+)',body)
  if not evidence or 'LOCAL VERIFIED' not in body:errors.append(key+' lacks completion evidence')
  elif not any((R/src).is_file() for src in re.findall(r'`([^`]+)`',evidence[1])):errors.append(key+' lacks existing evidence file')
seen=set()
def visit(k,stack):
 if k in stack:errors.append('Cycle '+str(stack+[k]));return
 if k in seen:return
 for d in graph[k]:
  if d not in graph:errors.append(k+' unknown '+d)
  else:visit(d,stack+[k])
 seen.add(k)
for k in graph:visit(k,[])
# 计数只数结构化任务项（`- [x] **ARCH-...`），与 graph 解析口径一致；非任务的
# 子项 checkbox（如 审核发现： 行）是合法内容，不应破坏任务 DAG 计数。
if len(re.findall(r'(?m)^\s*- \[[ x]\] \*\*',text))!=len(graph):errors.append('Unstructured checkbox')
if errors:raise SystemExit('\n'.join(errors))
print(f'PASS: SVG XML, scoped links/evidence, diagram references, {len(graph)} unique TODO IDs and acyclic dependencies')
