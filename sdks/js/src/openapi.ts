/**
 * OpenAPI 3 import helpers — Descriptor v2 (mirrors Go RegisterFromOpenAPI).
 *
 * Parses an OpenAPI 3 spec locally (no server connection), converts every
 * operation into a FunctionDescriptor and registers it with a
 * caller-supplied handler.
 */

import type { FunctionDescriptor, FunctionHandler } from "./index";

/** Controls OpenAPI import behaviour (mirrors Go ImportOptions). */
export interface ImportOptions {
  /** Prefix prepended to every imported resource (e.g. "game"). */
  resourcePrefix?: string;
  /** Prefix prepended to every imported tag. */
  tagPrefix?: string;
  /** Default handler timeout (ms) applied to every imported function. */
  defaultTimeoutMs?: number;
  /** Keep importing remaining operations when one fails. */
  continueOnError?: boolean;
}

/** Resolves a handler for a derived function ID; return undefined to skip. */
export type HandlerResolver = (
  functionId: string,
) => FunctionHandler | undefined;

/** Minimal registration surface; BasicClient satisfies it structurally. */
export interface RegistrationTarget {
  registerFunction(
    descriptor: FunctionDescriptor,
    handler: FunctionHandler,
  ): void;
}

/** Controlled capability vocabulary (Descriptor v2 contract). */
const CAPABILITIES = [
  "collection_query",
  "item_query",
  "create",
  "update",
  "delete",
  "action",
  "task",
  "report",
] as const;

/** Controlled execution vocabulary (Descriptor v2 contract). */
const EXECUTIONS = ["sync", "task"] as const;

/** Risk vocabulary safe|warning|high|danger with deprecated aliases. */
const RISK_ALIASES: Record<string, string> = {
  safe: "safe",
  low: "safe",
  warning: "warning",
  medium: "warning",
  moderate: "warning",
  high: "high",
  danger: "danger",
  critical: "danger",
};

const OPERATION_METHODS = [
  "get",
  "put",
  "post",
  "delete",
  "options",
  "head",
  "patch",
  "trace",
] as const;

type JsonRecord = Record<string, unknown>;

function isRecord(value: unknown): value is JsonRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function deriveOperationId(operation: JsonRecord, path: string): string {
  const operationId = operation.operationId;
  if (typeof operationId === "string" && operationId) {
    return operationId;
  }
  if (path) {
    const segments = path
      .split("/")
      .filter((segment) => segment !== "");
    if (segments.length > 0) {
      return segments.join(".");
    }
  }
  return "unknown.function";
}

function toTitleCase(value: string): string {
  return value
    .split("_")
    .map((word) => (word ? word[0].toUpperCase() + word.slice(1).toLowerCase() : word))
    .join(" ");
}

function deriveName(operation: JsonRecord, operationId: string): string {
  const summary = operation.summary;
  if (typeof summary === "string" && summary) {
    return summary;
  }
  if (operationId && operationId !== "unknown.function") {
    return toTitleCase(operationId);
  }
  return "Unnamed Function";
}

/** Shallow OpenAPI-schema -> JSON-Schema conversion (Go parity). */
function schemaToJsonSchema(schema: unknown): Record<string, unknown> | undefined {
  if (!isRecord(schema) || Object.keys(schema).length === 0) {
    return undefined;
  }
  const result: Record<string, unknown> = {};
  if (typeof schema.type === "string" && schema.type) {
    result.type = schema.type;
  }
  if (typeof schema.description === "string" && schema.description) {
    result.description = schema.description;
  }
  if (isRecord(schema.properties)) {
    const props: Record<string, unknown> = {};
    for (const [name, rawProp] of Object.entries(schema.properties)) {
      if (isRecord(rawProp)) {
        const entry: Record<string, unknown> = {
          type: typeof rawProp.type === "string" ? rawProp.type : "object",
        };
        if (typeof rawProp.description === "string" && rawProp.description) {
          entry.description = rawProp.description;
        }
        props[name] = entry;
      }
    }
    result.properties = props;
  }
  if (Array.isArray(schema.required) && schema.required.length > 0) {
    result.required = schema.required;
  }
  return Object.keys(result).length > 0 ? result : undefined;
}

function jsonContentSchema(holder: unknown): Record<string, unknown> | undefined {
  if (!isRecord(holder)) return undefined;
  const content = holder.content;
  if (!isRecord(content)) return undefined;
  const media = content["application/json"];
  if (!isRecord(media)) return undefined;
  return schemaToJsonSchema(media.schema);
}

function extractExtension(operation: JsonRecord, key: string): string {
  const value = operation[key];
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  if (typeof value === "boolean") return value ? "true" : "false";
  return JSON.stringify(value);
}

function parseCapability(value: string, functionId: string): string {
  const normalized = value.trim().toLowerCase();
  if (!(CAPABILITIES as readonly string[]).includes(normalized)) {
    throw new Error(
      `invalid x-capability "${value}" for ${functionId}: expected one of ${CAPABILITIES.join("|")}`,
    );
  }
  return normalized;
}

function parseExecution(value: string, functionId: string): string {
  const normalized = value.trim().toLowerCase();
  if (!(EXECUTIONS as readonly string[]).includes(normalized)) {
    throw new Error(
      `invalid x-execution "${value}" for ${functionId}: expected sync|task`,
    );
  }
  return normalized;
}

function parseRiskLevel(value: string, functionId: string): string {
  const normalized = value.trim().toLowerCase();
  const risk = RISK_ALIASES[normalized];
  if (!risk) {
    throw new Error(
      `invalid x-risk "${value}" for ${functionId}: expected safe|warning|high|danger`,
    );
  }
  return risk;
}

function applyApproval(
  descriptor: FunctionDescriptor,
  operation: JsonRecord,
): void {
  const value = operation["x-approval"];
  if (value === undefined || value === null) return;
  if (!isRecord(value)) {
    throw new Error(
      `x-approval for ${descriptor.id} must be an object: { required, policyKey }`,
    );
  }
  const required = value.required;
  if (required !== undefined && typeof required !== "boolean") {
    throw new Error(`x-approval.required for ${descriptor.id} must be a boolean`);
  }
  const policyKey = value.policyKey;
  if (policyKey !== undefined && typeof policyKey !== "string") {
    throw new Error(`x-approval.policyKey for ${descriptor.id} must be a string`);
  }
  descriptor.approvalRequired = required === true;
  if (policyKey) {
    descriptor.approvalPolicyKey = policyKey;
  }
}

function operationToDescriptor(
  path: string,
  operation: JsonRecord,
  options?: ImportOptions,
): FunctionDescriptor {
  const functionId = deriveOperationId(operation, path);
  const rawTags = Array.isArray(operation.tags) ? operation.tags : [];
  const tags = rawTags.filter((tag): tag is string => typeof tag === "string");
  const name = deriveName(operation, functionId);

  const descriptor: FunctionDescriptor = {
    id: functionId,
    version: "1.0.0",
    name,
    summary: name,
    description: typeof operation.description === "string" ? operation.description : undefined,
    tags,
    resource: extractExtension(operation, "x-resource") || undefined,
    operation: extractExtension(operation, "x-operation") || undefined,
    permission: extractExtension(operation, "x-permission") || undefined,
  };

  descriptor.inputSchema = jsonContentSchema(operation.requestBody);
  const responses = operation.responses;
  if (isRecord(responses) && isRecord(responses["200"])) {
    descriptor.outputSchema = jsonContentSchema(responses["200"]);
  }

  const capability = extractExtension(operation, "x-capability");
  if (capability) descriptor.capability = parseCapability(capability, functionId);

  const execution = extractExtension(operation, "x-execution");
  if (execution) descriptor.execution = parseExecution(execution, functionId);

  const risk = extractExtension(operation, "x-risk");
  descriptor.risk = risk ? parseRiskLevel(risk, functionId) : "warning";

  applyApproval(descriptor, operation);

  if (options) {
    if (options.resourcePrefix && descriptor.resource) {
      descriptor.resource = `${options.resourcePrefix}.${descriptor.resource}`;
    }
    if (options.tagPrefix) {
      descriptor.tags = tags.map((tag) => options.tagPrefix! + tag);
    }
    if (options.defaultTimeoutMs && options.defaultTimeoutMs > 0) {
      descriptor.timeoutMs = options.defaultTimeoutMs;
    }
  }

  return descriptor;
}

function* iterOperations(spec: JsonRecord): Generator<[string, JsonRecord]> {
  const paths = spec.paths;
  if (!isRecord(paths)) return;
  for (const [path, rawItem] of Object.entries(paths)) {
    if (!isRecord(rawItem)) continue;
    for (const method of OPERATION_METHODS) {
      const operation = rawItem[method];
      if (isRecord(operation)) {
        yield [path, operation];
      }
    }
  }
}

/**
 * Import an OpenAPI 3 spec and register every operation on `client`.
 *
 * Handlers come from either `handlerResolver` (called with the derived
 * function ID) or the `handlers` mapping. Returns the list of registered
 * function IDs. Throws on invalid specs, invalid Descriptor v2 extension
 * values or missing handlers (unless `options.continueOnError` is set).
 */
export function registerFromOpenAPI(
  client: RegistrationTarget,
  spec: string | JsonRecord,
  options?: ImportOptions,
  handlerResolver?: HandlerResolver,
  handlers?: Map<string, FunctionHandler>,
): string[] {
  let document: JsonRecord;
  if (typeof spec === "string") {
    try {
      document = JSON.parse(spec) as JsonRecord;
    } catch (error) {
      throw new Error(
        `load OpenAPI spec failed: ${(error as Error).message}`,
      );
    }
  } else {
    document = spec;
  }
  if (!isRecord(document) || !isRecord(document.paths)) {
    throw new Error("OpenAPI spec must be an object containing 'paths'");
  }

  const resolver: HandlerResolver =
    handlerResolver ??
    (handlers
      ? (functionId) => handlers.get(functionId)
      : () => undefined);

  const registered: string[] = [];
  for (const [path, operation] of iterOperations(document)) {
    let descriptor: FunctionDescriptor;
    try {
      descriptor = operationToDescriptor(path, operation, options);
    } catch (error) {
      if (options?.continueOnError) continue;
      throw new Error(
        `convert operation ${deriveOperationId(operation, path)} failed: ${(error as Error).message}`,
      );
    }
    const handler = resolver(descriptor.id);
    if (!handler) {
      if (options?.continueOnError) continue;
      throw new Error(`no handler provided for function: ${descriptor.id}`);
    }
    try {
      client.registerFunction(descriptor, handler);
    } catch (error) {
      if (options?.continueOnError) continue;
      throw new Error(
        `register function ${descriptor.id} failed: ${(error as Error).message}`,
      );
    }
    registered.push(descriptor.id);
  }
  return registered;
}
