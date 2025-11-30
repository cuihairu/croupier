import http2, { Http2SecureServer, Http2Server } from 'node:http2';
import { randomUUID } from 'node:crypto';
import { gzipSync } from 'node:zlib';
import { TextDecoder, TextEncoder } from 'node:util';
import { createPromiseClient, ConnectError, Code } from '@connectrpc/connect';
import type { PromiseClient } from '@connectrpc/connect';
import { connectNodeAdapter, createGrpcTransport } from '@connectrpc/connect-node';
import { LocalControlService } from '../generated/croupier/agent/local/v1/local_connect.js';
import { FunctionService } from '../generated/croupier/function/v1/function_connect.js';
import { ControlService } from '../generated/croupier/control/v1/control_connect.js';
import type { InvokeResponse, JobEvent, StartJobResponse } from '../generated/croupier/function/v1/function_pb.js';

const encoder = new TextEncoder();
const decoder = new TextDecoder();

export interface FunctionDescriptor {
  id: string;
  version: string;
  name?: string;
  description?: string;
  input_schema?: Record<string, any>;
  output_schema?: Record<string, any>;
}

export interface FunctionHandler {
  (context: string, payload: string): Promise<string> | string;
}

export interface FileTransferConfig {
  agentAddr?: string;
  controlAddr?: string;
  timeout?: number;
  retryAttempts?: number;
  insecure?: boolean;
  serviceId?: string;
  serviceVersion?: string;
  localListen?: string;
  heartbeatIntervalSeconds?: number;
  providerLang?: string;
  providerSdk?: string;
}

export interface FileUploadRequest {
  filePath: string;
  content: Buffer | string;
  metadata?: Record<string, any>;
}

interface LocalFunctionDescriptor {
  id: string;
  version: string;
}

class JobState {
  private queue: JobEvent[] = [];
  private waiting: Array<(value: JobEvent | null) => void> = [];
  private closed = false;

  push(event: JobEvent, close = false): void {
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

  async *stream(): AsyncIterable<JobEvent> {
    while (true) {
      if (this.queue.length > 0) {
        yield this.queue.shift() as JobEvent;
        continue;
      }
      if (this.closed) {
        break;
      }
      const next = await new Promise<JobEvent | null>((resolve) => this.waiting.push(resolve));
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
  registerFunction(descriptor: FunctionDescriptor, handler: FunctionHandler): Promise<void>;
  uploadFile(request: FileUploadRequest): Promise<void>;
}

export class BasicClient implements CroupierClient {
  private readonly config: Required<FileTransferConfig>;

  private handlers: Map<string, FunctionHandler> = new Map();

  private descriptors: Map<string, FunctionDescriptor> = new Map();

  private jobStates: Map<string, JobState> = new Map();

  private localServer?: Http2Server | Http2SecureServer;

  private localAddress = '';

  private agentClient?: PromiseClient<typeof LocalControlService>;

  private transport?: ReturnType<typeof createGrpcTransport>;

  private heartbeatTimer?: NodeJS.Timeout;

  private sessionId = '';

  private connected = false;

  constructor(config: FileTransferConfig = {}) {
    this.config = {
      agentAddr: '127.0.0.1:19090',
      controlAddr: '',
      timeout: 30000,
      retryAttempts: 3,
      insecure: true,
      serviceId: `node-sdk-${randomUUID()}`,
      serviceVersion: '1.0.0',
      localListen: '127.0.0.1:0',
      heartbeatIntervalSeconds: 60,
      providerLang: 'node',
      providerSdk: 'croupier-js-sdk',
      ...config,
    };
  }

  async connect(): Promise<void> {
    if (this.connected) {
      return;
    }
    if (this.handlers.size === 0) {
      throw new Error('Register at least one function before connecting to the agent.');
    }

    await this.startLocalServer();
    await this.connectAgent();
    await this.registerWithAgent();
    await this.registerCapabilities();

    this.connected = true;
  }

  async disconnect(): Promise<void> {
    this.connected = false;
    this.sessionId = '';
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = undefined;
    }
    this.transport = undefined;
    if (this.localServer) {
      await new Promise<void>((resolve) => {
        this.localServer?.close(() => resolve());
      });
      this.localServer = undefined;
    }
  }

  async registerFunction(descriptor: FunctionDescriptor, handler: FunctionHandler): Promise<void> {
    if (this.connected) {
      throw new Error('Cannot register new functions while connected. Disconnect first.');
    }
    if (!descriptor.id || !descriptor.version) {
      throw new Error('Function descriptor must include id and version.');
    }
    this.descriptors.set(descriptor.id, descriptor);
    this.handlers.set(descriptor.id, handler);
  }

  async uploadFile(_request: FileUploadRequest): Promise<void> {
    throw new Error('File upload is not yet implemented for the Node.js SDK.');
  }

  private async startLocalServer(): Promise<void> {
    if (this.localServer) {
      return;
    }

    const [host, port] = this.parseAddress(this.config.localListen);
    const handler = connectNodeAdapter({
      routes: (router) => {
        router.service(FunctionService, {
          invoke: async (req: any) =>
            this.handleInvoke(req.functionId, req.metadata ?? {}, req.payload ?? new Uint8Array()),
          startJob: async (req: any) =>
            this.handleStartJob(req.functionId, req.metadata ?? {}, req.payload ?? new Uint8Array()),
          streamJob: (req: any) => this.handleStreamJob(req.jobId),
          cancelJob: async (req: any) => this.handleCancelJob(req.jobId),
        });
      },
    });
    this.localServer = http2.createServer({}, handler);

    await new Promise<void>((resolve, reject) => {
      this.localServer?.once('error', reject);
      this.localServer?.listen(port, host, () => resolve());
    });

    const addressInfo = this.localServer.address();
    if (typeof addressInfo === 'object' && addressInfo) {
      const address =
        addressInfo.address === '::' || addressInfo.address === '0.0.0.0'
          ? '127.0.0.1'
          : addressInfo.address;
      this.localAddress = `${address}:${addressInfo.port}`;
    } else {
      this.localAddress = this.config.localListen;
    }
  }

  private async connectAgent(): Promise<void> {
    const scheme = this.config.insecure ? 'http' : 'https';
    const baseUrl = `${scheme}://${this.normalizeAddress(this.config.agentAddr)}`;

    this.transport = createGrpcTransport({
      baseUrl,
      httpVersion: '2',
    });
    this.agentClient = createPromiseClient(LocalControlService, this.transport);
  }

  private async registerWithAgent(): Promise<void> {
    if (!this.agentClient) {
      throw new Error('Agent client not initialized');
    }
    const functions: LocalFunctionDescriptor[] = Array.from(this.descriptors.values()).map((desc) => ({
      id: desc.id,
      version: desc.version,
    }));

    const response = (await this.agentClient.registerLocal({
      serviceId: this.config.serviceId,
      version: this.config.serviceVersion,
      rpcAddr: this.localAddress,
      functions,
    })) as { sessionId: string };
    this.sessionId = response.sessionId;
    this.startHeartbeat();
  }

  private async registerCapabilities(): Promise<void> {
    if (!this.config.controlAddr) {
      return;
    }

    const manifest = this.buildManifest();
    const compressed = gzipSync(Buffer.from(JSON.stringify(manifest)));

    const transport = createGrpcTransport({
      baseUrl: this.normalizeAddressWithScheme(this.config.controlAddr),
      httpVersion: '2',
    });
    const controlClient = createPromiseClient(ControlService, transport);
    await controlClient.registerCapabilities({
      provider: {
        id: this.config.serviceId,
        version: this.config.serviceVersion,
        lang: this.config.providerLang ?? 'node',
        sdk: this.config.providerSdk ?? 'croupier-js-sdk',
      },
      manifestJsonGz: compressed,
    });
  }

  private startHeartbeat(): void {
    if (!this.agentClient || !this.sessionId) {
      return;
    }
    const intervalMs = (this.config.heartbeatIntervalSeconds ?? 60) * 1000;
    this.heartbeatTimer = setInterval(async () => {
      try {
        await this.agentClient?.heartbeat({
          serviceId: this.config.serviceId,
          sessionId: this.sessionId,
        });
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('[croupier] Heartbeat failed:', error);
      }
    }, intervalMs);
  }

  private async handleInvoke(
    functionId: string,
    metadata: Record<string, string>,
    payload: Uint8Array,
  ): Promise<InvokeResponse> {
    const handler = this.handlers.get(functionId);
    if (!handler) {
      throw new ConnectError(`Function ${functionId} not registered`, Code.NotFound);
    }
    const context = JSON.stringify(metadata ?? {});
    const payloadString = decoder.decode(payload ?? new Uint8Array());
    try {
      const result = await handler(context, payloadString);
      return {
        payload: encoder.encode(result ?? ''),
      } as InvokeResponse;
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Handler failed';
      throw new ConnectError(message, Code.Internal);
    }
  }

  private async handleStartJob(
    functionId: string,
    metadata: Record<string, string>,
    payload: Uint8Array,
  ): Promise<StartJobResponse> {
    const handler = this.handlers.get(functionId);
    if (!handler) {
      throw new ConnectError(`Function ${functionId} not registered`, Code.NotFound);
    }
    const jobId = `${functionId}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const state = new JobState();
    this.jobStates.set(jobId, state);
    state.push(
      {
        type: 'started',
        message: 'job started',
        progress: 0,
        payload: new Uint8Array(),
      } as JobEvent,
    );

    const context = JSON.stringify(metadata ?? {});
    const payloadString = decoder.decode(payload ?? new Uint8Array());

    setImmediate(async () => {
      try {
        const result = await handler(context, payloadString);
        state.push(
          {
            type: 'completed',
            message: 'job completed',
            progress: 100,
            payload: encoder.encode(result ?? ''),
          } as JobEvent,
          true,
        );
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Handler failed';
        state.push(
          {
            type: 'error',
            message,
            progress: 0,
            payload: new Uint8Array(),
          } as JobEvent,
          true,
        );
      } finally {
        this.jobStates.delete(jobId);
      }
    });

    return { jobId } as StartJobResponse;
  }

  private handleStreamJob(jobId: string): AsyncIterable<JobEvent> {
    if (!jobId) {
      throw new ConnectError('job_id is required', Code.InvalidArgument);
    }
    const state = this.jobStates.get(jobId);
    if (!state) {
      throw new ConnectError('job not found', Code.NotFound);
    }
    return state.stream();
  }

  private async handleCancelJob(jobId: string): Promise<StartJobResponse> {
    const state = this.jobStates.get(jobId);
    if (state) {
      state.push(
        {
          type: 'cancelled',
          message: 'job cancelled',
          progress: 0,
          payload: new Uint8Array(),
        } as JobEvent,
        true,
      );
      this.jobStates.delete(jobId);
    }
    return { jobId } as StartJobResponse;
  }

  private parseAddress(address: string): [string, number] {
    if (address.includes('/')) {
      const url = new URL(address);
      return [url.hostname, Number(url.port) || 0];
    }
    const [host, port] = address.split(':');
    return [host || '127.0.0.1', Number(port) || 0];
  }

  private normalizeAddress(address: string): string {
    if (address.startsWith('http://') || address.startsWith('https://')) {
      return address.replace(/^https?:\/\//, '');
    }
    return address;
  }

  private normalizeAddressWithScheme(address: string): string {
    if (address.startsWith('http://') || address.startsWith('https://')) {
      return address;
    }
    const scheme = this.config.insecure ? 'http' : 'https';
    return `${scheme}://${address}`;
  }

  private buildManifest() {
    const functions = Array.from(this.descriptors.values()).map((desc) => ({
      id: desc.id,
      version: desc.version || '1.0.0',
      category: desc.name,
      description: desc.description,
      input_schema: desc.input_schema,
      output_schema: desc.output_schema,
    }));

    return {
      provider: {
        id: this.config.serviceId,
        version: this.config.serviceVersion,
        lang: this.config.providerLang ?? 'node',
        sdk: this.config.providerSdk ?? 'croupier-js-sdk',
      },
      functions,
    };
  }
}

export function createClient(config?: FileTransferConfig): CroupierClient {
  return new BasicClient(config);
}

export { BasicClient as default };
