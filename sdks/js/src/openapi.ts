/**
 * OpenAPI 3 import helpers — mirrors the Go SDK's RegisterFromOpenAPI.
 *
 * Parses an OpenAPI 3 spec, converts every operation into a
 * FunctionDescriptor and registers it on a CroupierClient with a
 * caller-supplied handler.
 */

import type { BasicClient, FunctionDescriptor, FunctionHandler } from "./index";

/** Controls OpenAPI import behaviour (mirrors Go ImportOptions). */
export interface ImportOptions {
  /** Prefix prepended to every imported resource (e.g. "game"). */
  resourcePrefix?: string;
  /** Prefix prepended to every imported tag. */
  tagPrefix?: string;
  /** Keep importing remaining operations when one fails. */
  continueOnError?: boolean;
}

/** Resolves a handler for a derived function ID; return undefined to skip. */
export type HandlerResolver = (
  functionId: string,
) => FunctionHandler | undefined;

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

function deriveSummary(operation: JsonRecord, operationId: string): string {
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

function parseRiskLevel(level: string): string {
  const normalized = level.toLowerCase();
  if (normalized === "low" || normalized === "safe") return "low";
  if (normalized === "high") return "high";
  if (normalized === "danger" || normalized === "critical") return "danger";
  return "medium";
}

function operationToDescriptor(
  path: string,
  operation: JsonRecord,
  options?: ImportOptions,
): FunctionDescriptor {
  const functionId = deriveOperationId(operation, path);
  const rawTags = Array.isArray(operation.tags) ? operation.tags : [];
  const tags = rawTags.filter((tag): tag is string => typeof tag === "string");

  const descriptor: FunctionDescriptor = {
    id: functionId,
    version: "1.0.0",
    summary: deriveSummary(operation, functionId),
    description: typeof operation.description === "string" ? operation.description : undefined,
    tags,
    resource: extractExtension(operation, "x-resource") || undefined,
    operation: extractExtension(operation, "x-operation") || undefined,
    permission: extractExtension(operation, "x-permission") || undefined,
  };

  descriptor.input_schema = jsonContentSchema(operation.requestBody);
  const responses = operation.responses;
  if (isRecord(responses) && isRecord(responses["200"])) {
    descriptor.output_schema = jsonContentSchema(responses["200"]);
  }

  const risk = extractExtension(operation, "x-risk");
  descriptor.risk = risk ? parseRiskLevel(risk) : "medium";

  if (options) {
    if (options.resourcePrefix && descriptor.resource) {
      descriptor.resource = `${options.resourcePrefix}.${descriptor.resource}`;
    }
    if (options.tagPrefix) {
      descriptor.tags = tags.map((tag) => options.tagPrefix! + tag);
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
 * function IDs. Throws on invalid specs or when a handler is missing (unless
 * `options.continueOnError` is set).
 */
export function registerFromOpenAPI(
  client: BasicClient,
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
    const descriptor = operationToDescriptor(path, operation, options);
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
