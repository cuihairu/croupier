// Public function/invocation contracts and internal descriptor types.
// Extracted from index.ts (LAY-009) as a pure type module; re-exported via the barrel.

import type { Readable } from "node:stream";

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
  connectTimeout?: number; // Connection timeout in milliseconds (default: 5000)
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
  /**
   * User-defined provider metadata (multiple key-value pairs), e.g.
   * `{ serverId: "s1" }`. Sent with ProviderConnectRequest and shown on the
   * dashboard SDK distribution page. Reserved keys (sdkLanguage/sdkVersion/
   * sdkName/gameId/env/protocol_version) are dropped by the agent.
   */
  instanceMetadata?: Record<string, string>;

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

  // === File Push (F：文件下发接收，hotpatch P1 传输层) ===
  /** 暂存目录：下发文件仅落盘至此（不自动应用），默认 ./croupier-staging */
  fileStagingDir?: string;

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

export interface ProviderFunctionDescriptor {
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

export interface TaskEvent {
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
