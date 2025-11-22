/**
 * Croupier Node.js SDK
 *
 * A powerful SDK for registering game functions with Croupier's distributed GM backend system.
 * Supports file transfer for server-side hot reload.
 */

// Basic function registration interfaces
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

// File transfer interfaces for server hot reload support
export interface FileTransferConfig {
  agentAddr?: string;
  timeout?: number;
  retryAttempts?: number;
}

export interface FileUploadRequest {
  filePath: string;
  content: Buffer | string;
  metadata?: Record<string, any>;
}

// Basic client interface (to be implemented)
export interface CroupierClient {
  connect(): Promise<void>;
  disconnect(): Promise<void>;
  registerFunction(descriptor: FunctionDescriptor, handler: FunctionHandler): Promise<void>;
  uploadFile(request: FileUploadRequest): Promise<void>;
}

// Placeholder implementations will be added in future releases
export class BasicClient implements CroupierClient {
  private config: FileTransferConfig;

  constructor(config: FileTransferConfig = {}) {
    this.config = {
      agentAddr: '127.0.0.1:19090',
      timeout: 30000,
      retryAttempts: 3,
      ...config
    };
  }

  async connect(): Promise<void> {
    throw new Error('BasicClient not yet implemented. Please use the gRPC client directly.');
  }

  async disconnect(): Promise<void> {
    throw new Error('BasicClient not yet implemented. Please use the gRPC client directly.');
  }

  async registerFunction(descriptor: FunctionDescriptor, handler: FunctionHandler): Promise<void> {
    throw new Error('BasicClient not yet implemented. Please use the gRPC client directly.');
  }

  async uploadFile(request: FileUploadRequest): Promise<void> {
    throw new Error('File upload not yet implemented.');
  }
}

// Factory function
export function createClient(config?: FileTransferConfig): CroupierClient {
  return new BasicClient(config);
}

// Re-exports
export { BasicClient as default };