/**
 * Croupier JavaScript SDK
 *
 * Provides function registration and invocation for the Croupier platform.
 * Uses TCP session transport for bidirectional multiplexed communication.
 */

import { createHash, randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import { extname, basename } from "node:path";
import { Readable } from "node:stream";
import { gzipSync } from "node:zlib";
import { TextDecoder, TextEncoder } from "node:util";
import * as protobuf from "protobufjs";
import Ajv, { type ValidateFunction } from "ajv";
import { TCPTransport } from "./tcp_transport";
import {
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
  MSG_REGISTER_CAPABILITIES_REQ,
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_START_TASK_REQUEST,
  MSG_START_TASK_RESPONSE,
  MSG_TASK_EVENT,
  MSG_CANCEL_TASK_REQUEST,
  MSG_PROVIDER_DRAIN_REQUEST,
} from "./protocol";

const encoder = new TextEncoder();
const decoder = new TextDecoder();
const PROVIDER_PROTO = `
syntax = "proto3";
package croupier.sdk.v1;

message ProviderFunctionDescriptor {
  string id = 1;
  string version = 2;
  repeated string tags = 3;
  string summary = 4;
  string description = 5;
  string operation_id = 6;
  bool deprecated = 7;
  string input_schema = 8;
  string output_schema = 9;
  string resource = 10;
  string operation = 11;
  string risk = 12;
  bool enabled = 13;
  string permission = 14;
  string capability = 15;
  string execution = 16;
  bool approval_required = 17;
  string approval_policy_key = 18;
}

message ProviderConnectRequest {
  string service_id = 1;
  string version = 2;
  repeated ProviderFunctionDescriptor functions = 3;
  string sdk_language = 4;
  string sdk_version = 5;
  string sdk_name = 6;
  string protocol_version = 7;
  repeated string supported_capabilities = 8;
  string transport_security_mode = 9;
  repeated string supported_transports = 10;
  string game_id = 11;
  string env = 12;
}

message ProviderConnectResponse {
  string session_id = 1;
}

message HeartbeatRequest {
  string service_id = 1;
  string session_id = 2;
}

message HeartbeatResponse {}

message ProviderMeta {
  string id = 1;
  string version = 2;
  string lang = 3;
  string sdk = 4;
}

message RegisterCapabilitiesRequest {
  ProviderMeta provider = 1;
  bytes manifest_json_gz = 2;
}

message RegisterCapabilitiesResponse {}

// Inbound invocation messages (mirror proto/croupier/sdk/v1/invocation.proto).
message InvokeRequest {
  string function_id = 1;
  string idempotencyKey = 2;
  bytes payload = 3;
  map<string, string> metadata = 4;
}

message InvokeResponse {
  bytes payload = 1;
}

message StartTaskResponse {
  string task_id = 1;
}

message CancelTaskRequest {
  string task_id = 1;
}
`;
const providerRoot = protobuf.parse(PROVIDER_PROTO).root;
const ProviderConnectRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectRequest",
);
const ProviderConnectResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectResponse",
);
const HeartbeatRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.HeartbeatRequest",
);
const RegisterCapabilitiesRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.RegisterCapabilitiesRequest",
);
const RegisterCapabilitiesResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.RegisterCapabilitiesResponse",
);
const InvokeRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.InvokeRequest",
);
const InvokeResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.InvokeResponse",
);
const StartTaskResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.StartTaskResponse",
);
const CancelTaskRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.CancelTaskRequest",
);

/**
 * Configuration for retrying failed invocations with exponential backoff.
 *
 * When enabled, the client will automatically retry failed invocations
 * using an exponential backoff strategy with jitter to prevent thundering
 * herd problems.
 *
 * @example
 * ```typescript
 * const retryConfig: RetryConfig = {
 *   enabled: true,
 *   maxAttempts: 3,
 *   initialDelayMs: 100,
 *   maxDelayMs: 5000,
 *   backoffMultiplier: 2.0,
 *   jitterFactor: 0.1,
 *   retryableStatusCodes: [14, 13, 2, 10, 4]
 * };
 * ```
 */
export interface RetryConfig {
  /**
   * Whether retry is enabled.
   * @default true
   */
  enabled?: boolean;

  /**
   * Maximum number of retry attempts.
   * @default 3
   */
  maxAttempts?: number;

  /**
   * Initial retry delay in milliseconds.
   * @default 100
   */
  initialDelayMs?: number;

  /**
   * Maximum retry delay in milliseconds.
   * @default 5000
   */
  maxDelayMs?: number;

  /**
   * Backoff multiplier for exponential backoff.
   * @default 2.0
   */
  backoffMultiplier?: number;

  /**
   * Jitter factor (0-1) for adding randomness to delays.
   * @default 0.1
   */
  jitterFactor?: number;

  /**
   * List of status codes that should trigger a retry.
   * Default codes: UNAVAILABLE(14), INTERNAL(13), UNKNOWN(2), ABORTED(10), DEADLINE_EXCEEDED(4)
   */
  retryableStatusCodes?: number[];
}

/**
 * Configuration for automatic reconnection with exponential backoff.
 *
 * When enabled, the client will automatically attempt to reconnect
 * after connection failures using an exponential backoff strategy with
 * jitter to prevent thundering herd problems.
 *
 * @example
 * ```typescript
 * const reconnectConfig: ReconnectConfig = {
 *   enabled: true,
 *   maxAttempts: 0,  // Infinite retries
 *   initialDelayMs: 1000,  // 1 second
 *   maxDelayMs: 30000,  // 30 seconds
 *   backoffMultiplier: 2.0,
 *   jitterFactor: 0.2
 * };
 * ```
 */
export interface ReconnectConfig {
  /**
   * Whether automatic reconnection is enabled.
   * @default true
   */
  enabled?: boolean;

  /**
   * Maximum number of reconnection attempts (0 = infinite retries).
   * @default 0
   */
  maxAttempts?: number;

  /**
   * Initial reconnection delay in milliseconds.
   * @default 1000
   */
  initialDelayMs?: number;

  /**
   * Maximum reconnection delay in milliseconds.
   * @default 30000
   */
  maxDelayMs?: number;

  /**
   * Backoff multiplier for exponential backoff.
   * @default 2.0
   */
  backoffMultiplier?: number;

  /**
   * Jitter factor (0-1) for adding randomness to delays.
   * @default 0.2
   */
  jitterFactor?: number;
}

/**
 * Options for invoking a function.
 *
 * @example
 * ```typescript
 * const options: InvokeOptions = {
 *   idempotencyKey: 'unique-key-123',
 *   timeout: 5000,
 *   headers: {
 *     'X-Request-ID': 'req-456',
 *     'X-Game-ID': 'my-game'
 *   },
 *   retry: {
 *     maxAttempts: 3
 *   }
 * };
 * ```
 */
export interface InvokeOptions {
  /**
   * Unique key for idempotent requests.
   *
   * Requests with the same idempotency key will return the same result,
   * preventing duplicate executions.
   */
  idempotencyKey?: string;

  /**
   * Per-invocation timeout in milliseconds.
   *
   * If not specified, uses the client-level timeout.
   */
  timeout?: number;

  /**
   * Retry configuration for the invocation.
   */
  retry?: RetryConfig;

  /**
   * Custom headers/metadata for the invocation.
   */
  headers?: Record<string, string>;
}

export interface FunctionDescriptor {
  id: string;
  version: string;
  tags?: string[];
  name?: string;
  summary?: string;
  description?: string;
  operationId?: string;
  deprecated?: boolean;
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  resource?: string;
  risk?: string;
  operation?: string;
  capability?: string;
  execution?: string;
  approvalRequired?: boolean;
  approvalPolicyKey?: string;
  permission?: string;
  enabled?: boolean;
  /** Handler timeout in milliseconds (Descriptor v2 behavior). */
  timeoutMs?: number;
}

export interface FunctionHandler {
  (context: string, payload: string): Promise<string> | string;
}

export interface ClientConfig {
  // === Connection ===
  agentAddr?: string;
  timeout?: number;
  connectTimeout?: number;  // Connection timeout in milliseconds (default: 5000)
  controlAddr?: string;

  // === Service Identity ===
  agentId?: string;
  serviceId?: string;
  serviceVersion?: string;

  // === Game Context ===
  gameId?: string;
  env?: string;

  // === Heartbeat ===
  heartbeatIntervalSeconds?: number;

  // === Provider Info ===
  providerLang?: string;
  providerSdk?: string;

  // === TLS Configuration ===
  insecure?: boolean;
  certFile?: string;
  keyFile?: string;
  caFile?: string;
  serverName?: string;

  // === Authentication ===
  authToken?: string;
  headers?: Record<string, string>;

  // === Reconnection ===
  autoReconnect?: boolean;
  reconnectInterval?: number;
  reconnect?: ReconnectConfig;

  // === Retry ===
  retry?: RetryConfig;

  // === Inbound Payload Validation (F15) ===
  /** Provider 侧入站校验：按函数声明的 input schema 校验 invoke/startTask
   *  payload，失败回 {"error":"payload validation failed: ..."}。默认关闭。 */
  validateInputPayloads?: boolean;

  // === File Transfer ===
  enableFileTransfer?: boolean;
  maxFileSize?: number;
  allowedExtensions?: string[];
  allowedMimeTypes?: string[];
  uploadTimeout?: number;
  parallelUploads?: number;

  // === Logging ===
  disableLogging?: boolean;
  debugLogging?: boolean;
  logLevel?: "DEBUG" | "INFO" | "WARN" | "ERROR" | "OFF";
}

interface ProviderFunctionDescriptor {
  id: string;
  version: string;
  tags?: string[];
  summary?: string;
  description?: string;
  operationId?: string;
  deprecated?: boolean;
  inputSchema?: string;
  outputSchema?: string;
  resource?: string;
  risk?: string;
  operation?: string;
  capability?: string;
  execution?: string;
  permission?: string;
  enabled?: boolean;
}

interface TaskEvent {
  type: string;
  message?: string;
  progress?: number;
  payload: Uint8Array;
}

export interface FileUploadProgress {
  filePath: string;
  loaded: number;
  total?: number;
  percent?: number;
}

export interface FileUploadRequest {
  filePath: string;
  content?: string | Uint8Array | Buffer;
  metadata?: Record<string, string>;
  mimeType?: string;
  onProgress?: (progress: FileUploadProgress) => void;
}

export interface FileUploadStreamRequest {
  filePath: string;
  stream: Readable;
  metadata?: Record<string, string>;
  mimeType?: string;
  onProgress?: (progress: FileUploadProgress) => void;
}

export interface FileUploadResult {
  filePath: string;
  fileName: string;
  size: number;
  mimeType: string;
  sha256: string;
  metadata: Record<string, string>;
  status: "validated";
}

export interface FileUploadBatchItemResult {
  ok: boolean;
  filePath: string;
  result?: FileUploadResult;
  error?: string;
}

export interface FileUploadBatchResult {
  total: number;
  succeeded: number;
  failed: number;
  results: FileUploadResult[];
  items: FileUploadBatchItemResult[];
}

class TaskState {
  private queue: TaskEvent[] = [];
  private waiting: Array<(value: TaskEvent | null) => void> = [];
  private closed = false;

  push(event: TaskEvent, close = false): void {
    if (this.closed) {
      return;
    }
    if (this.waiting.length > 0) {
      const resolve = this.waiting.shift();
      resolve?.(event);
    } else {
      this.queue.push(event);
    }
    if (close) {
      this.close();
    }
  }

  close(): void {
    if (this.closed) {
      return;
    }
    this.closed = true;
    while (this.waiting.length > 0) {
      const resolve = this.waiting.shift();
      resolve?.(null);
    }
  }

  async *stream(): AsyncIterable<TaskEvent> {
    while (true) {
      if (this.queue.length > 0) {
        yield this.queue.shift() as TaskEvent;
        continue;
      }
      if (this.closed) {
        break;
      }
      const next = await new Promise<TaskEvent | null>((resolve) =>
        this.waiting.push(resolve),
      );
      if (!next) {
        break;
      }
      yield next;
    }
  }
}

export interface CroupierClient {
  connect(): Promise<void>;
  disconnect(): Promise<void>;
  registerFunction(
    descriptor: FunctionDescriptor,
    handler: FunctionHandler,
  ): void;
  invoke(
    functionId: string,
    payload: string,
    optionsOrMetadata?: InvokeOptions | Record<string, string>,
  ): Promise<string>;
  startTask(
    functionId: string,
    payload: string,
    optionsOrMetadata?: InvokeOptions | Record<string, string>,
  ): string;
  streamTask(taskId: string): AsyncIterable<TaskEvent>;
  cancelTask(taskId: string): boolean;
  uploadFile(request: FileUploadRequest): Promise<FileUploadResult>;
  uploadFileStream(request: FileUploadStreamRequest): Promise<FileUploadResult>;
  uploadFiles(requests: FileUploadRequest[]): Promise<FileUploadBatchResult>;
  serve(): Promise<void>;
  serveAsync(): Promise<void>;
}

export class BasicClient implements CroupierClient {
  private readonly config: Required<ClientConfig>;
  private handlers: Map<string, FunctionHandler> = new Map();
  // drain 状态：收到 ProviderDrainRequest 后置位——拒绝新 Invoke，
  // 在途调用清零后复用 scheduleReconnect 恢复会话（对齐 C# 参考实现）。
  private draining = false;
  private inflightCalls = 0;
  private descriptors: Map<string, FunctionDescriptor> = new Map();
  // F15：入站校验编译缓存（schema 原文 → 编译后的校验函数）
  private readonly inboundAjv = new Ajv({ allErrors: true });
  private readonly inboundSchemaCache = new Map<string, ValidateFunction | null>();
  private taskStates: Map<string, TaskState> = new Map();
  private transport: TCPTransport | null = null;
  private connected = false;
  private sessionId = "";
  private heartbeatTimer: NodeJS.Timeout | null = null;
  private reconnectPromise: Promise<void> | null = null;
  private stopRequested = false;

  constructor(config: ClientConfig = {}) {
    this.config = {
      // Connection
      agentAddr: "tcp://127.0.0.1:19091",
      timeout: 30000,
      connectTimeout: 5000,
      controlAddr: "",

      // Service Identity
      agentId: "",
      serviceId: `node-sdk-${randomUUID()}`,
      serviceVersion: "1.0.0",

      // Game Context
      gameId: "",
      env: "development",

      // Heartbeat
      heartbeatIntervalSeconds: 60,

      // Provider Info
      providerLang: "node",
      providerSdk: "croupier-js-sdk",

      // Inbound payload validation (opt-in)
      validateInputPayloads: false,

      // TLS (defaults: insecure local development)
      insecure: true,
      certFile: "",
      keyFile: "",
      caFile: "",
      serverName: "",

      // Authentication
      authToken: "",
      headers: {},

      // Reconnection
      autoReconnect: true,
      reconnectInterval: 5000,

      // Retry
      // File Transfer
      enableFileTransfer: false,
      maxFileSize: 10485760, // 10 MB default
      allowedExtensions: [],
      allowedMimeTypes: [],
      uploadTimeout: 300000,
      parallelUploads: 4,

      // Logging
      disableLogging: false,
      debugLogging: false,
      logLevel: "INFO",

      ...config,

      // Deep merge for nested objects
      reconnect: {
        enabled: true,
        maxAttempts: 0,
        initialDelayMs: 1000,
        maxDelayMs: 30000,
        backoffMultiplier: 2.0,
        jitterFactor: 0.2,
        ...config.reconnect,
      },
      retry: {
        enabled: true,
        maxAttempts: 3,
        initialDelayMs: 100,
        maxDelayMs: 5000,
        backoffMultiplier: 2.0,
        jitterFactor: 0.1,
        retryableStatusCodes: [14, 13, 2, 10, 4],
        ...config.retry,
      },
    };
  }

  async connect(): Promise<void> {
    if (this.connected) {
      return;
    }
    if (this.handlers.size === 0) {
      throw new Error("Register at least one function before connecting.");
    }

    this.stopRequested = false;
    await this.connectAndRegister();
    this.connected = true;
    this.startHeartbeatLoop();
  }

  async disconnect(): Promise<void> {
    this.stopRequested = true;
    this.stopHeartbeatLoop();
    if (this.transport) {
      this.transport.close();
      this.transport = null;
    }
    this.connected = false;
    this.sessionId = "";
    this.reconnectPromise = null;
  }

  async serve(): Promise<void> {
    // Connect if not already connected
    if (!this.connected) {
      await this.connect();
    }

    // Keep serving indefinitely
    return new Promise<void>((resolve, reject) => {
      // Handle graceful shutdown
      const cleanup = () => {
        this.disconnect().catch((err) => {
          console.error("Error during disconnect:", err);
        });
      };

      // Setup process handlers for graceful shutdown
      if (process.listenerCount("SIGINT") === 0) {
        process.once("SIGINT", () => {
          console.log("\nReceived SIGINT, shutting down gracefully...");
          cleanup();
          resolve();
        });
      }

      if (process.listenerCount("SIGTERM") === 0) {
        process.once("SIGTERM", () => {
          console.log("Received SIGTERM, shutting down gracefully...");
          cleanup();
          resolve();
        });
      }

      // Log that we're serving
      console.log(`Croupier client serving at ${this.config.agentAddr}`);
      console.log(
        `Service: ${this.config.serviceId} @ ${this.config.serviceVersion}`,
      );
      console.log(`Registered functions: ${this.handlers.size}`);
      console.log("Press Ctrl+C to stop...");
    });
  }

  async serveAsync(): Promise<void> {
    // Same as serve(), but with explicit async documentation
    return this.serve();
  }

  registerFunction(
    descriptor: FunctionDescriptor,
    handler: FunctionHandler,
  ): void {
    if (this.connected) {
      throw new Error(
        "Cannot register new functions while connected. Disconnect first.",
      );
    }
    if (!descriptor.id || !descriptor.version) {
      throw new Error("Function descriptor must include id and version.");
    }
    this.descriptors.set(descriptor.id, descriptor);
    this.handlers.set(descriptor.id, handler);
  }

  getFunctionDescriptor(
    functionId: string,
  ): ProviderFunctionDescriptor | undefined {
    const desc = this.descriptors.get(functionId);
    if (!desc) {
      return undefined;
    }
    return {
      id: desc.id,
      version: desc.version,
      tags: desc.tags,
      summary: desc.summary || desc.name,
      description: desc.description,
      operationId: desc.operationId || desc.id,
      deprecated: desc.deprecated,
      inputSchema: desc.inputSchema
        ? JSON.stringify(desc.inputSchema)
        : undefined,
      outputSchema: desc.outputSchema
        ? JSON.stringify(desc.outputSchema)
        : undefined,
      resource: desc.resource,
      risk: desc.risk,
      operation: desc.operation,
      capability: desc.capability,
      execution: desc.execution,
      permission: desc.permission,
      enabled: desc.enabled,
    };
  }

  getRegisterRequest() {
    return {
      serviceId: this.config.serviceId,
      version: this.config.serviceVersion,
      functions: Array.from(this.descriptors.values()).map((desc) => ({
        id: desc.id,
        version: desc.version,
        tags: desc.tags || [],
        summary: desc.summary || desc.name || "",
        description: desc.description || "",
        operationId: desc.operationId || desc.id,
        deprecated: desc.deprecated || false,
        inputSchema: desc.inputSchema
          ? JSON.stringify(desc.inputSchema)
          : "",
        outputSchema: desc.outputSchema
          ? JSON.stringify(desc.outputSchema)
          : "",
        resource: desc.resource || "",
        operation: desc.operation || "",
        capability: desc.capability || "",
        execution: desc.execution || "",
        approvalRequired: desc.approvalRequired ?? false,
        approvalPolicyKey: desc.approvalPolicyKey || "",
        risk: desc.risk || "",
        enabled: desc.enabled ?? true,
        permission: desc.permission || "",
      })),
    };
  }

  async invoke(
    functionId: string,
    payload: string,
    optionsOrMetadata: InvokeOptions | Record<string, string> = {},
  ): Promise<string> {
    const handler = this.handlers.get(functionId);
    if (!handler) {
      throw new Error(`Function ${functionId} not found`);
    }

    // Normalize options to InvokeOptions
    let options: InvokeOptions;
    if (this.isInvokeOptions(optionsOrMetadata)) {
      options = optionsOrMetadata;
    } else {
      options = { headers: optionsOrMetadata };
    }

    // Merge with client-level retry config
    const retryConfig = {
      ...this.config.retry,
      ...options.retry,
    };

    // Use per-invocation timeout if specified, otherwise use client-level timeout
    const timeout = options.timeout ?? this.config.timeout;

    const metadata = this.buildInvocationMetadata(options.headers);

    // If transport is connected, use remote invocation
    if (this.connected && this.transport) {
      const requestData = Buffer.from(
        JSON.stringify({
          function_id: functionId,
          payload: encoder.encode(payload),
          metadata,
          idempotencyKey: options.idempotencyKey,
          timeout,
          retry_config: retryConfig,
        }),
      );
      const [, responseData] = await this.transport.call(
        MSG_INVOKE_REQUEST,
        requestData,
      );
      return decoder.decode(responseData);
    }

    // Local invocation
    const context = JSON.stringify({
      ...metadata,
      ...(options.idempotencyKey && {
        idempotencyKey: options.idempotencyKey,
      }),
      timeout,
    });

    const result = await handler(context, payload);
    return result;
  }

  startTask(
    functionId: string,
    payload: string,
    optionsOrMetadata: InvokeOptions | Record<string, string> = {},
  ): string {
    const handler = this.handlers.get(functionId);
    if (!handler) {
      throw new Error(`Function ${functionId} not found`);
    }

    // Normalize options to InvokeOptions
    let options: InvokeOptions;
    if (this.isInvokeOptions(optionsOrMetadata)) {
      options = optionsOrMetadata;
    } else {
      options = { headers: optionsOrMetadata };
    }

    // Merge with client-level retry config
    const retryConfig = {
      ...this.config.retry,
      ...options.retry,
    };

    // Use per-invocation timeout if specified, otherwise use client-level timeout
    const timeout = options.timeout ?? this.config.timeout;

    const metadata = this.buildInvocationMetadata(options.headers);

    const taskId = `${functionId}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const state = new TaskState();
    this.taskStates.set(taskId, state);

    state.push({
      type: "started",
      message: "task started",
      progress: 0,
      payload: new Uint8Array(),
    });

    const context = JSON.stringify({
      ...metadata,
      ...(options.idempotencyKey && {
        idempotencyKey: options.idempotencyKey,
      }),
      timeout,
    });

    setImmediate(async () => {
      try {
        const result = await handler(context, payload);
        state.push(
          {
            type: "completed",
            message: "task completed",
            progress: 100,
            payload: encoder.encode(result ?? ""),
          },
          true,
        );
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "Handler failed";
        state.push(
          {
            type: "error",
            message,
            progress: 0,
            payload: new Uint8Array(),
          },
          true,
        );
      } finally {
        this.taskStates.delete(taskId);
      }
    });

    return taskId;
  }

  streamTask(taskId: string): AsyncIterable<TaskEvent> {
    const state = this.taskStates.get(taskId);
    if (!state) {
      throw new Error(`Task ${taskId} not found`);
    }
    return state.stream();
  }

  cancelTask(taskId: string): boolean {
    const state = this.taskStates.get(taskId);
    if (state) {
      state.push(
        {
          type: "cancelled",
          message: "task cancelled",
          progress: 0,
          payload: new Uint8Array(),
        },
        true,
      );
      this.taskStates.delete(taskId);
      return true;
    }
    return false;
  }

  async uploadFile(request: FileUploadRequest): Promise<FileUploadResult> {
    this.ensureFileTransferEnabled();

    const filePath = this.normalizeUploadPath(request.filePath);
    const content =
      request.content !== undefined
        ? this.normalizeUploadContent(request.content)
        : new Uint8Array(await readFile(filePath));

    this.emitUploadProgress(request.onProgress, {
      filePath,
      loaded: content.byteLength,
      total: content.byteLength,
      percent: 100,
    });

    return this.buildUploadResult({
      filePath,
      content,
      metadata: request.metadata,
      mimeType: request.mimeType,
    });
  }

  async uploadFileStream(
    request: FileUploadStreamRequest,
  ): Promise<FileUploadResult> {
    this.ensureFileTransferEnabled();

    const filePath = this.normalizeUploadPath(request.filePath);
    const content = await this.readStreamWithTimeout({
      filePath,
      stream: request.stream,
      total: this.parseUploadTotal(request.metadata?.size),
      onProgress: request.onProgress,
    });

    return this.buildUploadResult({
      filePath,
      content,
      metadata: request.metadata,
      mimeType: request.mimeType,
    });
  }

  async uploadFiles(
    requests: FileUploadRequest[],
  ): Promise<FileUploadBatchResult> {
    this.ensureFileTransferEnabled();
    if (requests.length === 0) {
      return {
        total: 0,
        succeeded: 0,
        failed: 0,
        results: [],
        items: [],
      };
    }

    const items: FileUploadBatchItemResult[] = new Array(requests.length);
    const concurrency = Math.max(1, this.config.parallelUploads);
    let nextIndex = 0;

    const worker = async (): Promise<void> => {
      while (nextIndex < requests.length) {
        const current = nextIndex;
        nextIndex += 1;
        const request = requests[current];
        const filePath = this.normalizeUploadPath(request.filePath);

        try {
          const result = await this.uploadFile(request);
          items[current] = {
            ok: true,
            filePath,
            result,
          };
        } catch (error) {
          items[current] = {
            ok: false,
            filePath,
            error:
              error instanceof Error ? error.message : "Unknown upload error.",
          };
        }
      }
    };

    const workers = Array.from(
      { length: Math.min(concurrency, requests.length) },
      () => worker(),
    );
    await Promise.all(workers);

    const results = items
      .filter(
        (
          item,
        ): item is FileUploadBatchItemResult & { result: FileUploadResult } =>
          item.ok === true && item.result !== undefined,
      )
      .map((item) => item.result);

    return {
      total: requests.length,
      succeeded: results.length,
      failed: items.filter((item) => !item.ok).length,
      results,
      items,
    };
  }

  /**
   * Type guard to check if an object is InvokeOptions.
   *
   * Checks for presence of InvokeOptions-specific fields (retry, idempotencyKey, timeout)
   * that would not typically be in a simple metadata object.
   */
  private isInvokeOptions(
    obj: InvokeOptions | Record<string, string>,
  ): obj is InvokeOptions {
    if (!obj || typeof obj !== "object") {
      return false;
    }
    // Check if any InvokeOptions-specific field is present
    return (
      "retry" in obj ||
      "idempotencyKey" in obj ||
      "timeout" in obj ||
      "headers" in obj
    );
  }

  private buildInvocationMetadata(
    invocationHeaders?: Record<string, string>,
  ): Record<string, string> {
    const metadata: Record<string, string> = {
      ...this.config.headers,
      ...(invocationHeaders || {}),
    };

    if (this.config.authToken && !metadata.Authorization) {
      metadata.Authorization = `Bearer ${this.config.authToken}`;
    }
    if (this.config.gameId && !metadata["X-Game-ID"]) {
      metadata["X-Game-ID"] = this.config.gameId;
    }
    if (this.config.env && !metadata["X-Env"]) {
      metadata["X-Env"] = this.config.env;
    }

    return metadata;
  }

  private ensureFileTransferEnabled(): void {
    if (!this.config.enableFileTransfer) {
      throw new Error(
        "File transfer is disabled. Set enableFileTransfer=true to use upload APIs.",
      );
    }
  }

  private normalizeUploadPath(filePath: string): string {
    const normalized = filePath.trim();
    if (!normalized) {
      throw new Error("File upload request must include a non-empty filePath.");
    }
    return normalized;
  }

  private normalizeUploadContent(
    content: string | Uint8Array | Buffer,
  ): Uint8Array {
    if (typeof content === "string") {
      return encoder.encode(content);
    }
    return content instanceof Uint8Array ? content : new Uint8Array(content);
  }

  private emitUploadProgress(
    callback: ((progress: FileUploadProgress) => void) | undefined,
    progress: FileUploadProgress,
  ): void {
    callback?.(progress);
  }

  private parseUploadTotal(totalValue?: string): number | undefined {
    if (!totalValue) {
      return undefined;
    }
    const parsed = Number(totalValue);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return undefined;
    }
    return parsed;
  }

  private async readStreamWithTimeout(params: {
    filePath: string;
    stream: Readable;
    total?: number;
    onProgress?: (progress: FileUploadProgress) => void;
  }): Promise<Uint8Array> {
    const chunks: Buffer[] = [];
    let total = 0;

    const readPromise = new Promise<Uint8Array>((resolve, reject) => {
      params.stream.on("data", (chunk: Buffer | string) => {
        const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
        total += buffer.byteLength;
        if (total > this.config.maxFileSize) {
          reject(
            new Error(
              `File exceeds maxFileSize limit (${this.config.maxFileSize} bytes).`,
            ),
          );
          params.stream.destroy();
          return;
        }
        chunks.push(buffer);
        this.emitUploadProgress(params.onProgress, {
          filePath: params.filePath,
          loaded: total,
          total: params.total,
          percent: params.total
            ? Math.min(100, (total / params.total) * 100)
            : undefined,
        });
      });
      params.stream.on("error", reject);
      params.stream.on("end", () => {
        this.emitUploadProgress(params.onProgress, {
          filePath: params.filePath,
          loaded: total,
          total: params.total,
          percent: params.total ? 100 : undefined,
        });
        resolve(new Uint8Array(Buffer.concat(chunks)));
      });
    });

    const timeoutPromise = new Promise<never>((_, reject) => {
      setTimeout(() => {
        reject(
          new Error(
            `File upload stream timed out after ${this.config.uploadTimeout}ms.`,
          ),
        );
      }, this.config.uploadTimeout);
    });

    return Promise.race([readPromise, timeoutPromise]);
  }

  private buildUploadResult(params: {
    filePath: string;
    content: Uint8Array;
    metadata?: Record<string, string>;
    mimeType?: string;
  }): FileUploadResult {
    const fileName = basename(params.filePath);
    const extension = extname(params.filePath).toLowerCase();
    const size = params.content.byteLength;
    const mimeType = params.mimeType || this.guessMimeType(extension);

    if (size === 0) {
      throw new Error("File upload content cannot be empty.");
    }
    if (size > this.config.maxFileSize) {
      throw new Error(
        `File exceeds maxFileSize limit (${this.config.maxFileSize} bytes).`,
      );
    }
    if (
      this.config.allowedExtensions.length > 0 &&
      !this.config.allowedExtensions.includes(extension)
    ) {
      throw new Error(
        `File extension ${extension || "<none>"} is not allowed.`,
      );
    }
    if (
      this.config.allowedMimeTypes.length > 0 &&
      !this.config.allowedMimeTypes.includes(mimeType)
    ) {
      throw new Error(`MIME type ${mimeType} is not allowed.`);
    }

    return {
      filePath: params.filePath,
      fileName,
      size,
      mimeType,
      sha256: createHash("sha256")
        .update(Buffer.from(params.content))
        .digest("hex"),
      metadata: { ...(params.metadata || {}) },
      status: "validated",
    };
  }

  private guessMimeType(extension: string): string {
    switch (extension) {
      case ".json":
        return "application/json";
      case ".js":
        return "text/javascript";
      case ".ts":
        return "text/typescript";
      case ".txt":
      case ".md":
        return "text/plain";
      case ".zip":
        return "application/zip";
      case ".png":
        return "image/png";
      case ".jpg":
      case ".jpeg":
        return "image/jpeg";
      default:
        return "application/octet-stream";
    }
  }

  private async connectAndRegister(): Promise<void> {
    const transport = new TCPTransport({
      address: this.config.agentAddr,
      timeoutMs: this.config.timeout,
      connectTimeoutMs: this.config.connectTimeout,
      tlsEnabled: !this.config.insecure,
      tlsCertFile: this.config.certFile,
      tlsKeyFile: this.config.keyFile,
      tlsCaFile: this.config.caFile,
      tlsServerName: this.config.serverName,
      tlsInsecureSkipVerify: this.config.insecure,
    });
    await transport.connect();

    try {
      const [, responseData] = await transport.call(
        MSG_PROVIDER_CONNECT_REQUEST,
        this.serializeProviderConnectRequest(this.getRegisterRequest()),
      );
      const response = this.parseProviderConnectResponse(responseData);
      if (!response.sessionId) {
        throw new Error("ProviderConnect returned empty session_id");
      }

      if (this.transport) {
        this.transport.close();
      }
      this.transport = transport;
      this.sessionId = response.sessionId;
      // 挂入站处理器：Agent 推送的 InvokeRequest/StartTask/CancelTask 经此分发。
      // 首连与重连都走本函数，一处挂载覆盖两条路径。
      transport.setHandler(this.handleInboundRequest);
      await this.maybeRegisterCapabilities();
    } catch (error) {
      transport.close();
      throw error;
    }
  }

  private async maybeRegisterCapabilities(): Promise<void> {
    if (!this.config.controlAddr) {
      return;
    }

    const controlTransport = new TCPTransport({
      address: this.config.controlAddr,
      timeoutMs: this.config.timeout,
      connectTimeoutMs: this.config.connectTimeout,
      tlsEnabled: !this.config.insecure,
      tlsCertFile: this.config.certFile,
      tlsKeyFile: this.config.keyFile,
      tlsCaFile: this.config.caFile,
      tlsServerName: this.config.serverName,
      tlsInsecureSkipVerify: this.config.insecure,
    });
    await controlTransport.connect();

    try {
      const [, responseData] = await controlTransport.call(
        MSG_REGISTER_CAPABILITIES_REQ,
        this.serializeRegisterCapabilitiesRequest(),
      );
      this.parseRegisterCapabilitiesResponse(responseData);
    } catch (error) {
      // Capabilities upload is best-effort and must not break registration.
      if (!this.config.disableLogging) {
        console.warn("Failed to register capabilities:", error);
      }
    } finally {
      controlTransport.close();
    }
  }

  // === Inbound dispatch (Agent -> Provider push) ===

  /**
   * 处理 Agent 主动推送的入站请求。TCPTransport 的读循环/worker 会调用本回调，
   * 返回值作为响应帧回写；抛出的异常会被 transport 捕获并静默（不回帧）。
   */
  private handleInboundRequest = async (
    msgId: number,
    _reqId: number,
    body: Buffer,
  ): Promise<Buffer> => {
    if (msgId === MSG_INVOKE_REQUEST) {
      return this.handleInboundInvoke(body);
    }
    if (msgId === MSG_START_TASK_REQUEST) {
      return this.handleInboundStartTask(body);
    }
    if (msgId === MSG_CANCEL_TASK_REQUEST) {
      return this.handleInboundCancelTask(body);
    }
    if (msgId === MSG_PROVIDER_DRAIN_REQUEST) {
      return this.handleDrainRequest(body);
    }
    // 未知入站消息：回空响应，避免 Agent 侧挂起等待超时。
    return Buffer.alloc(0);
  };

  /**
   * 处理 Agent 的优雅下线请求：置位 drain 状态（拒绝新 Invoke）、
   * 异步等待在途调用清零后恢复会话；协议规定立即回空 ProviderDrainResponse 确认。
   * 幂等：重复 drain 只回确认。
   */
  private handleDrainRequest(_body: Buffer): Buffer {
    if (!this.draining) {
      this.draining = true;
      void this.drainAndRecover();
    }
    return Buffer.alloc(0);
  }

  private async drainAndRecover(): Promise<void> {
    try {
      // 等待在途调用完成（最多 30 秒，超时后仅记录并继续）。
      const deadline = Date.now() + 30_000;
      while (this.inflightCalls > 0 && Date.now() < deadline) {
        await this.delay(100);
      }
      if (this.inflightCalls > 0) {
        console.warn(
          `[croupier] Drain timeout with ${this.inflightCalls} in-flight call(s) still running`,
        );
      }
      if (
        this.config.autoReconnect &&
        this.config.reconnect.enabled &&
        !this.stopRequested
      ) {
        await this.scheduleReconnect();
      } else {
        this.connected = false;
        this.stopHeartbeatLoop();
        this.transport?.close();
        this.transport = null;
      }
    } finally {
      this.draining = false;
    }
  }

  private decodeInvokeRequest(body: Buffer): {
    functionId: string;
    idempotencyKey: string;
    payload: string;
    metadata: Record<string, string>;
  } {
    const decoded = InvokeRequestMessage.toObject(
      InvokeRequestMessage.decode(body),
      { defaults: true },
    ) as {
      functionId?: string;
      idempotencyKey?: string;
      payload?: Uint8Array;
      metadata?: Record<string, string>;
    };
    return {
      functionId: decoded.functionId || "",
      idempotencyKey: decoded.idempotencyKey || "",
      payload: decoder.decode(decoded.payload ?? new Uint8Array()),
      metadata: decoded.metadata ?? {},
    };
  }

  private encodeInvokeResponse(payload: Uint8Array): Buffer {
    return Buffer.from(
      InvokeResponseMessage.encode(
        InvokeResponseMessage.create({ payload }),
      ).finish(),
    );
  }

  private async handleInboundInvoke(body: Buffer): Promise<Buffer> {
    // drain 期间拒绝新调用，等待 Agent 停止投递。
    if (this.draining) {
      return this.encodeInvokeResponse(
        encoder.encode(JSON.stringify({ error: "provider is draining" })),
      );
    }
    this.inflightCalls += 1;
    try {
      return await this.invokeInbound(body);
    } finally {
      this.inflightCalls -= 1;
    }
  }

  /**
   * F15：按函数声明的 input schema 校验入站 payload。开关关闭 / 未注册 /
   * schema 缺失或非法时跳过（服务端仍是权威校验方）；失败返回 Error。
   */
  private validateInboundPayload(functionId: string, payload: string): Error | null {
    if (!this.config.validateInputPayloads) return null;
    const descriptor = this.descriptors.get(functionId);
    const schema = descriptor?.inputSchema as Record<string, unknown> | undefined;
    if (!schema || typeof schema !== "object") return null;
    const cacheKey = JSON.stringify(schema);
    if (!this.inboundSchemaCache.has(cacheKey)) {
      try {
        this.inboundSchemaCache.set(cacheKey, this.inboundAjv.compile(schema));
      } catch {
        this.inboundSchemaCache.set(cacheKey, null); // 非法 schema：跳过
      }
    }
    const validate = this.inboundSchemaCache.get(cacheKey);
    if (!validate) return null;
    let value: unknown;
    try {
      value = JSON.parse(payload || "{}");
    } catch (error) {
      return new Error(
        `payload must be valid JSON: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
    const ok = validate(value);
    if (!ok) {
      const messages = (validate.errors ?? [])
        .map((item) => `${item.instancePath || "/"} ${item.message ?? ""}`.trim())
        .join("; ");
      return new Error(`payload validation failed: ${messages}`);
    }
    return null;
  }

  private async invokeInbound(body: Buffer): Promise<Buffer> {
    const req = this.decodeInvokeRequest(body);
    const handler = this.handlers.get(req.functionId);
    if (!handler) {
      // 未注册函数：回空 payload，Agent 侧按失败处理并 failover。
      return this.encodeInvokeResponse(new Uint8Array());
    }
    const validationError = this.validateInboundPayload(req.functionId, req.payload);
    if (validationError) {
      return this.encodeInvokeResponse(
        encoder.encode(JSON.stringify({ error: validationError.message })),
      );
    }
    const context = JSON.stringify({
      ...req.metadata,
      ...(req.idempotencyKey ? { idempotencyKey: req.idempotencyKey } : {}),
    });
    try {
      const result = await handler(context, req.payload);
      return this.encodeInvokeResponse(encoder.encode(result ?? ""));
    } catch (error) {
      const message = error instanceof Error ? error.message : "Handler failed";
      return this.encodeInvokeResponse(
        encoder.encode(JSON.stringify({ error: message })),
      );
    }
  }

  private handleInboundStartTask(body: Buffer): Buffer {
    const req = this.decodeInvokeRequest(body);
    let taskId = "";
    const validationError = this.validateInboundPayload(req.functionId, req.payload);
    if (!validationError && this.handlers.has(req.functionId)) {
      try {
        taskId = this.startTask(req.functionId, req.payload, req.metadata);
      } catch {
        // handler 启动失败：回空 task_id，Agent 侧按失败处理。
        taskId = "";
      }
    }
    return Buffer.from(
      StartTaskResponseMessage.encode(
        StartTaskResponseMessage.create({ taskId }),
      ).finish(),
    );
  }

  private handleInboundCancelTask(body: Buffer): Buffer {
    const decoded = CancelTaskRequestMessage.toObject(
      CancelTaskRequestMessage.decode(body),
      { defaults: true },
    ) as { taskId?: string };
    if (decoded.taskId) {
      this.cancelTask(decoded.taskId);
    }
    return this.encodeInvokeResponse(new Uint8Array());
  }

  private startHeartbeatLoop(): void {
    this.stopHeartbeatLoop();
    const intervalMs = Math.max(this.config.heartbeatIntervalSeconds, 1) * 1000;
    this.heartbeatTimer = setInterval(() => {
      void this.sendHeartbeat().catch(() => {
        void this.scheduleReconnect();
      });
    }, intervalMs);
  }

  private stopHeartbeatLoop(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private async sendHeartbeat(): Promise<void> {
    if (!this.transport || !this.connected || !this.sessionId) {
      throw new Error("Client is not registered");
    }

    await this.transport.call(
      MSG_PROVIDER_HEARTBEAT_REQUEST,
      this.serializeHeartbeatRequest({
        serviceId: this.config.serviceId,
        sessionId: this.sessionId,
      }),
    );
  }

  private async scheduleReconnect(): Promise<void> {
    if (
      this.stopRequested ||
      !this.config.autoReconnect ||
      !this.config.reconnect.enabled
    ) {
      return;
    }
    if (this.reconnectPromise) {
      return this.reconnectPromise;
    }

    this.connected = false;
    this.stopHeartbeatLoop();
    if (this.transport) {
      this.transport.close();
      this.transport = null;
    }

    this.reconnectPromise = this.reconnectLoop().finally(() => {
      this.reconnectPromise = null;
    });
    return this.reconnectPromise;
  }

  private async reconnectLoop(): Promise<void> {
    let attempt = 0;

    while (!this.stopRequested) {
      attempt += 1;

      try {
        await this.connectAndRegister();
        this.connected = true;
        this.startHeartbeatLoop();
        return;
      } catch {
        const maxAttempts = this.config.reconnect.maxAttempts ?? 0;
        if (maxAttempts > 0 && attempt >= maxAttempts) {
          throw new Error("Max reconnect attempts reached");
        }
        await this.delay(this.calculateReconnectDelay(attempt));
      }
    }
  }

  private calculateReconnectDelay(attempt: number): number {
    const initialDelayMs = this.config.reconnect.initialDelayMs ?? 1000;
    const maxDelayMs = this.config.reconnect.maxDelayMs ?? 30000;
    const backoffMultiplier = this.config.reconnect.backoffMultiplier ?? 2.0;
    const jitterFactor = this.config.reconnect.jitterFactor ?? 0.2;
    const exponentialDelay = Math.min(
      initialDelayMs * Math.pow(backoffMultiplier, Math.max(attempt - 1, 0)),
      maxDelayMs,
    );

    if (jitterFactor <= 0) {
      return exponentialDelay;
    }

    const jitter = (Math.random() * 2 - 1) * jitterFactor * exponentialDelay;
    return Math.max(0, Math.round(exponentialDelay + jitter));
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  private serializeProviderConnectProtobufRequest(
    request: ReturnType<BasicClient["getRegisterRequest"]>,
  ): Buffer {
    const payload = ProviderConnectRequestMessage.create({
      serviceId: request.serviceId,
      version: request.version,
      functions: request.functions.map((fn) => ({
        id: fn.id,
        version: fn.version,
        summary: fn.summary,
        description: fn.description,
        inputSchema: fn.inputSchema,
        outputSchema: fn.outputSchema,
        resource: fn.resource,
        operation: fn.operation,
        capability: fn.capability,
        execution: fn.execution,
        approvalRequired: fn.approvalRequired,
        approvalPolicyKey: fn.approvalPolicyKey,
        risk: fn.risk,
        enabled: fn.enabled,
        permission: fn.permission,
      })),
      sdkLanguage: this.config.providerLang || "javascript",
      sdkVersion: "1.0.0",
      sdkName: "croupier-js-sdk",
      protocolVersion: "1.0.0",
      transportSecurityMode: this.config.insecure ? "plaintext" : "tls",
    });

    return Buffer.from(ProviderConnectRequestMessage.encode(payload).finish());
  }

  private parseProviderConnectProtobufResponse(data: Buffer): { sessionId: string } {
    const decoded = ProviderConnectResponseMessage.decode(data);
    const object = ProviderConnectResponseMessage.toObject(decoded, {
      defaults: true,
    }) as { sessionId?: string };

    return {
      sessionId: object.sessionId || "",
    };
  }

  private serializeProviderConnectRequest(
    request: ReturnType<BasicClient["getRegisterRequest"]>,
  ): Buffer {
    return this.serializeProviderConnectProtobufRequest(request);
  }

  private parseProviderConnectResponse(data: Buffer): { sessionId: string } {
    try {
      const obj = JSON.parse(decoder.decode(data));
      return {
        sessionId: obj.session_id || "",
      };
    } catch {
      // Fallback to protobuf
      const decoded = ProviderConnectResponseMessage.decode(data);
      const object = ProviderConnectResponseMessage.toObject(decoded, {
        defaults: true,
      }) as { sessionId?: string };
      return {
        sessionId: object.sessionId || "",
      };
    }
  }

  private serializeHeartbeatRequest(request: {
    serviceId: string;
    sessionId: string;
  }): Buffer {
    const payload = HeartbeatRequestMessage.create({
      serviceId: request.serviceId,
      sessionId: request.sessionId,
    });
    return Buffer.from(HeartbeatRequestMessage.encode(payload).finish());
  }

  private serializeRegisterCapabilitiesRequest(): Buffer {
    const payload = RegisterCapabilitiesRequestMessage.create({
      provider: {
        id: this.config.serviceId,
        version: this.config.serviceVersion,
        lang: this.config.providerLang,
        sdk: this.config.providerSdk,
      },
      manifestJsonGz: this.getManifestGzipped(),
    });

    return Buffer.from(
      RegisterCapabilitiesRequestMessage.encode(payload).finish(),
    );
  }

  private parseRegisterCapabilitiesResponse(data: Buffer): void {
    RegisterCapabilitiesResponseMessage.decode(data);
  }

  buildManifest() {
    const functions = Array.from(this.descriptors.values()).map((desc) => ({
      id: desc.id,
      version: desc.version || "1.0.0",
      resource: desc.resource,
      operation: desc.operation,
      risk: desc.risk,
      permission: desc.permission,
      description: desc.description,
      inputSchema: desc.inputSchema,
      outputSchema: desc.outputSchema,
    }));

    return {
      provider: {
        id: this.config.serviceId,
        version: this.config.serviceVersion,
        lang: this.config.providerLang,
        sdk: this.config.providerSdk,
      },
      functions,
    };
  }

  getManifestGzipped(): Buffer {
    const manifest = this.buildManifest();
    return gzipSync(Buffer.from(JSON.stringify(manifest)));
  }
}


/**
 * F14：向函数描述符的 input schema 注入 x-ui 呈现 hints 的便捷层
 * （契约见 docs/architecture/presentation-hints.md）。
 */
export function setFieldHint(
  descriptor: FunctionDescriptor,
  field: string,
  hint: string,
  value: unknown,
): FunctionDescriptor {
  if (!field || !field.trim()) {
    throw new Error("field key is required for setFieldHint");
  }
  const normalized = normalizeHintKey(hint);
  if (!normalized) {
    throw new Error(`hint "${hint}" must be an x- extension key (e.g. x-widget)`);
  }
  const schema = { type: "object", ...(descriptor.inputSchema ?? {}) } as Record<string, unknown>;
  const properties = { ...((schema.properties as Record<string, unknown>) ?? {}) };
  const property = { ...((properties[field] as Record<string, unknown>) ?? {}) };
  property[normalized] = value;
  properties[field] = property;
  schema.properties = properties;
  return { ...descriptor, inputSchema: schema };
}

export function setFieldWidget(
  descriptor: FunctionDescriptor,
  field: string,
  widget: string,
): FunctionDescriptor {
  if (!widget || !widget.trim()) {
    throw new Error("widget is required for setFieldWidget");
  }
  return setFieldHint(descriptor, field, "x-widget", widget);
}

function normalizeHintKey(hint: string): string | null {
  const trimmed = hint.trim();
  if (!trimmed) return null;
  const lower = trimmed.toLowerCase();
  if (lower.startsWith("x_")) return `x-${trimmed.slice(2)}`;
  if (lower.startsWith("x-")) return trimmed;
  return null;
}

export function createClient(config?: ClientConfig): CroupierClient {
  return new BasicClient(config);
}

// Export protocol and transport
export * from "./protocol";
export { TCPTransport } from "./tcp_transport";
export { traceParentFromContext, traceIdFromContext } from "./trace";

// L3 Invoker (independent caller-side module over HTTP)
export {
  Invoker,
  createInvoker,
  InvokerError,
  InvokerEventSource,
  type InvokerConfig,
  type InvokeTaskOptions,
  type RetryConfig as InvokerRetryConfig,
  type TaskEvent,
  type TaskStatus,
  type InvokeResult,
} from "./invoker";

export {
  registerFromOpenAPI,
  type ImportOptions,
  type HandlerResolver as OpenAPIHandlerResolver,
  type RegistrationTarget as OpenAPIRegistrationTarget,
} from "./openapi";

export { BasicClient as default };
