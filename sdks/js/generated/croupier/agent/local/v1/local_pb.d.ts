import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import type { Message } from "@bufbuild/protobuf";
/**
 * Describes the file croupier/agent/local/v1/local.proto.
 */
export declare const file_croupier_agent_local_v1_local: GenFile;
/**
 * Local function registration from game servers to Agent
 *
 * @generated from message croupier.agent.local.v1.LocalFunctionDescriptor
 */
export type LocalFunctionDescriptor = Message<"croupier.agent.local.v1.LocalFunctionDescriptor"> & {
    /**
     * @generated from field: string id = 1;
     */
    id: string;
    /**
     * @generated from field: string version = 2;
     */
    version: string;
};
/**
 * Describes the message croupier.agent.local.v1.LocalFunctionDescriptor.
 * Use `create(LocalFunctionDescriptorSchema)` to create a new message.
 */
export declare const LocalFunctionDescriptorSchema: GenMessage<LocalFunctionDescriptor>;
/**
 * @generated from message croupier.agent.local.v1.RegisterLocalRequest
 */
export type RegisterLocalRequest = Message<"croupier.agent.local.v1.RegisterLocalRequest"> & {
    /**
     * @generated from field: string service_id = 1;
     */
    serviceId: string;
    /**
     * @generated from field: string version = 2;
     */
    version: string;
    /**
     * @generated from field: string rpc_addr = 3;
     */
    rpcAddr: string;
    /**
     * @generated from field: repeated croupier.agent.local.v1.LocalFunctionDescriptor functions = 4;
     */
    functions: LocalFunctionDescriptor[];
};
/**
 * Describes the message croupier.agent.local.v1.RegisterLocalRequest.
 * Use `create(RegisterLocalRequestSchema)` to create a new message.
 */
export declare const RegisterLocalRequestSchema: GenMessage<RegisterLocalRequest>;
/**
 * @generated from message croupier.agent.local.v1.RegisterLocalResponse
 */
export type RegisterLocalResponse = Message<"croupier.agent.local.v1.RegisterLocalResponse"> & {
    /**
     * @generated from field: string session_id = 1;
     */
    sessionId: string;
};
/**
 * Describes the message croupier.agent.local.v1.RegisterLocalResponse.
 * Use `create(RegisterLocalResponseSchema)` to create a new message.
 */
export declare const RegisterLocalResponseSchema: GenMessage<RegisterLocalResponse>;
/**
 * @generated from message croupier.agent.local.v1.HeartbeatRequest
 */
export type HeartbeatRequest = Message<"croupier.agent.local.v1.HeartbeatRequest"> & {
    /**
     * @generated from field: string service_id = 1;
     */
    serviceId: string;
    /**
     * @generated from field: string session_id = 2;
     */
    sessionId: string;
};
/**
 * Describes the message croupier.agent.local.v1.HeartbeatRequest.
 * Use `create(HeartbeatRequestSchema)` to create a new message.
 */
export declare const HeartbeatRequestSchema: GenMessage<HeartbeatRequest>;
/**
 * @generated from message croupier.agent.local.v1.HeartbeatResponse
 */
export type HeartbeatResponse = Message<"croupier.agent.local.v1.HeartbeatResponse"> & {};
/**
 * Describes the message croupier.agent.local.v1.HeartbeatResponse.
 * Use `create(HeartbeatResponseSchema)` to create a new message.
 */
export declare const HeartbeatResponseSchema: GenMessage<HeartbeatResponse>;
/**
 * Query local registered instances
 *
 * @generated from message croupier.agent.local.v1.LocalInstance
 */
export type LocalInstance = Message<"croupier.agent.local.v1.LocalInstance"> & {
    /**
     * @generated from field: string service_id = 1;
     */
    serviceId: string;
    /**
     * @generated from field: string addr = 2;
     */
    addr: string;
    /**
     * @generated from field: string version = 3;
     */
    version: string;
    /**
     * @generated from field: string last_seen = 4;
     */
    lastSeen: string;
};
/**
 * Describes the message croupier.agent.local.v1.LocalInstance.
 * Use `create(LocalInstanceSchema)` to create a new message.
 */
export declare const LocalInstanceSchema: GenMessage<LocalInstance>;
/**
 * @generated from message croupier.agent.local.v1.LocalFunction
 */
export type LocalFunction = Message<"croupier.agent.local.v1.LocalFunction"> & {
    /**
     * @generated from field: string id = 1;
     */
    id: string;
    /**
     * @generated from field: repeated croupier.agent.local.v1.LocalInstance instances = 2;
     */
    instances: LocalInstance[];
};
/**
 * Describes the message croupier.agent.local.v1.LocalFunction.
 * Use `create(LocalFunctionSchema)` to create a new message.
 */
export declare const LocalFunctionSchema: GenMessage<LocalFunction>;
/**
 * @generated from message croupier.agent.local.v1.ListLocalRequest
 */
export type ListLocalRequest = Message<"croupier.agent.local.v1.ListLocalRequest"> & {};
/**
 * Describes the message croupier.agent.local.v1.ListLocalRequest.
 * Use `create(ListLocalRequestSchema)` to create a new message.
 */
export declare const ListLocalRequestSchema: GenMessage<ListLocalRequest>;
/**
 * @generated from message croupier.agent.local.v1.ListLocalResponse
 */
export type ListLocalResponse = Message<"croupier.agent.local.v1.ListLocalResponse"> & {
    /**
     * @generated from field: repeated croupier.agent.local.v1.LocalFunction functions = 1;
     */
    functions: LocalFunction[];
};
/**
 * Describes the message croupier.agent.local.v1.ListLocalResponse.
 * Use `create(ListLocalResponseSchema)` to create a new message.
 */
export declare const ListLocalResponseSchema: GenMessage<ListLocalResponse>;
/**
 * Query job result from Agent (best-effort)
 *
 * @generated from message croupier.agent.local.v1.GetJobResultRequest
 */
export type GetJobResultRequest = Message<"croupier.agent.local.v1.GetJobResultRequest"> & {
    /**
     * @generated from field: string job_id = 1;
     */
    jobId: string;
};
/**
 * Describes the message croupier.agent.local.v1.GetJobResultRequest.
 * Use `create(GetJobResultRequestSchema)` to create a new message.
 */
export declare const GetJobResultRequestSchema: GenMessage<GetJobResultRequest>;
/**
 * @generated from message croupier.agent.local.v1.GetJobResultResponse
 */
export type GetJobResultResponse = Message<"croupier.agent.local.v1.GetJobResultResponse"> & {
    /**
     * @generated from field: string state = 1;
     */
    state: string;
    /**
     * @generated from field: bytes payload = 2;
     */
    payload: Uint8Array;
    /**
     * @generated from field: string error = 3;
     */
    error: string;
};
/**
 * Describes the message croupier.agent.local.v1.GetJobResultResponse.
 * Use `create(GetJobResultResponseSchema)` to create a new message.
 */
export declare const GetJobResultResponseSchema: GenMessage<GetJobResultResponse>;
/**
 * @generated from service croupier.agent.local.v1.LocalControlService
 */
export declare const LocalControlService: GenService<{
    /**
     * @generated from rpc croupier.agent.local.v1.LocalControlService.RegisterLocal
     */
    registerLocal: {
        methodKind: "unary";
        input: typeof RegisterLocalRequestSchema;
        output: typeof RegisterLocalResponseSchema;
    };
    /**
     * @generated from rpc croupier.agent.local.v1.LocalControlService.Heartbeat
     */
    heartbeat: {
        methodKind: "unary";
        input: typeof HeartbeatRequestSchema;
        output: typeof HeartbeatResponseSchema;
    };
    /**
     * @generated from rpc croupier.agent.local.v1.LocalControlService.ListLocal
     */
    listLocal: {
        methodKind: "unary";
        input: typeof ListLocalRequestSchema;
        output: typeof ListLocalResponseSchema;
    };
    /**
     * @generated from rpc croupier.agent.local.v1.LocalControlService.GetJobResult
     */
    getJobResult: {
        methodKind: "unary";
        input: typeof GetJobResultRequestSchema;
        output: typeof GetJobResultResponseSchema;
    };
}>;
//# sourceMappingURL=local_pb.d.ts.map