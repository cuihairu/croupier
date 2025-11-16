# HTTP Adapter (Generic)

The `http-adapter` exposes one function `http.generic_invoke` that forwards a generic HTTP request and returns the response body (best-effort JSON passthrough).

Usage
- Function: `http.generic_invoke`
- Request schema: `{ method, url, headers, body }`
- Output views (pack example):
  - `json.view` to preview raw response
  - `table.basic` to render array responses as a table

Examples
- List JSON array
  - URL: https://example.com/api/items
  - Response: `[ { "id": 1, "name": "foo" }, { "id": 2, "name": "bar" } ]`
  - The built-in `table.basic` view works directly with `transform.expr: '$'`.
- Nested data array
  - Response: `{ "data": { "items": [ { "id": 1 }, { "id": 2 } ] } }`
  - Update the view transform to `expr: '$.data.items'` or use `template`:
```
{
  "id": "table",
  "renderer": "table.basic",
  "transform": { "expr": "$.data.items" }
}
```
- Timeseries
  - Response: `{ "series": [ { "name": "cpu", "data": [[1719916800000, 0.5], ...] } ] }`
  - Add a chart view:
```
{
  "id": "chart",
  "renderer": "echarts.line",
  "transform": { "expr": "$.series" }
}
```

Notes
- The adapter does not transform JSON by itself. Packs (descriptors) can shape data for views via `outputs.views[].transform` (see `docs/ui-and-views.md`).
- For authenticated APIs, set headers in the request (e.g., `Authorization`).
- For large payloads, prefer `GET` or compressed responses; current timeout defaults to 15s.

Built-in function (example)
- The HTTP adapter also exposes a convenience function:
  - `alertmanager.list_alerts`: map simple params `{ base_url, silenced?, inhibited?, active? }` to Alertmanager `GET /api/v2/alerts`.
  - A sample pack `packs/alertmanager` renders alerts as a table (columns: name, severity, status, startsAt, summary).
  - Usage flow: import `alertmanager.pack.tgz` → select `alertmanager.list_alerts` → fill base URL and filters → view table/json.
