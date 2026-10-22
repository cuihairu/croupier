# Resources and Operations API

This page describes the resource page generation API conventions of the C++ SDK under descriptor v2.

The current target model does not define a standalone resource runtime API. The SDK only reports FunctionContract; the Server normalizes it into ResourceCapability / CapabilitySemantics and then generates PageProposal. Page categories, dynamic labels, page types, and placement are decided only in Page Studio / PageSpec.

## Required Fields

Provide the following:

- `id`
- `version`
- `summary`
- `description`
- `resource`
- `operation`
- `capability`
- `execution` (`sync` / `task`)
- `risk`
- `input_schema` (member name; contract key `inputSchema`)
- `output_schema` (member name; contract key `outputSchema`)
- For high-risk operations: `approval_required` (contract key `approvalRequired`) and `approval_policy_key` (contract key `approvalPolicyKey`)

The SDK descriptor does not provide page schemas, component trees, page mappings, menus, category display names, page titles, button labels, or page placement.

## Keep Reading
