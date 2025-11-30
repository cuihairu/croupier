import type { GenFile, GenMessage, GenService } from "@bufbuild/protobuf/codegenv2";
import type { Message } from "@bufbuild/protobuf";
/**
 * Describes the file croupier/function/v1/function.proto.
 */
export declare const file_croupier_function_v1_function: GenFile;
/**
 * InvokeRequest carries a function invocation.
 *
 * @generated from message croupier.function.v1.InvokeRequest
 */
export type InvokeRequest = Message<"croupier.function.v1.InvokeRequest"> & {
    /**
     * e.g. "player.ban"
     *
     * @generated from field: string function_id = 1;
     */
    functionId: string;
    /**
     * client-supplied idempotency key
     *
     * @generated from field: string idempotency_key = 2;
     */
    idempotencyKey: string;
    /**
     * serialized request (JSON/Proto), agreed by descriptor
     *
     * @generated from field: bytes payload = 3;
     */
    payload: Uint8Array;
    /**
     * optional k/v metadata
     *
     * @generated from field: map<string, string> metadata = 4;
     */
    metadata: {
        [key: string]: string;
    };
};
/**
 * Describes the message croupier.function.v1.InvokeRequest.
 * Use `create(InvokeRequestSchema)` to create a new message.
 */
export declare const InvokeRequestSchema: GenMessage<InvokeRequest>;
/**
 * @generated from message croupier.function.v1.InvokeResponse
 */
export type InvokeResponse = Message<"croupier.function.v1.InvokeResponse"> & {
    /**
     * serialized response body
     *
     * @generated from field: bytes payload = 1;
     */
    payload: Uint8Array;
};
/**
 * Describes the message croupier.function.v1.InvokeResponse.
 * Use `create(InvokeResponseSchema)` to create a new message.
 */
export declare const InvokeResponseSchema: GenMessage<InvokeResponse>;
/**
 * @generated from message croupier.function.v1.StartJobResponse
 */
export type StartJobResponse = Message<"croupier.function.v1.StartJobResponse"> & {
    /**
     * @generated from field: string job_id = 1;
     */
    jobId: string;
};
/**
 * Describes the message croupier.function.v1.StartJobResponse.
 * Use `create(StartJobResponseSchema)` to create a new message.
 */
export declare const StartJobResponseSchema: GenMessage<StartJobResponse>;
/**
 * @generated from message croupier.function.v1.JobStreamRequest
 */
export type JobStreamRequest = Message<"croupier.function.v1.JobStreamRequest"> & {
    /**
     * @generated from field: string job_id = 1;
     */
    jobId: string;
};
/**
 * Describes the message croupier.function.v1.JobStreamRequest.
 * Use `create(JobStreamRequestSchema)` to create a new message.
 */
export declare const JobStreamRequestSchema: GenMessage<JobStreamRequest>;
/**
 * @generated from message croupier.function.v1.JobEvent
 */
export type JobEvent = Message<"croupier.function.v1.JobEvent"> & {
    /**
     * "progress" | "log" | "done" | "error"
     *
     * @generated from field: string type = 1;
     */
    type: string;
    /**
     * free text for log/error
     *
     * @generated from field: string message = 2;
     */
    message: string;
    /**
     * 0..100 when type == progress
     *
     * @generated from field: int32 progress = 3;
     */
    progress: number;
    /**
     * optional final result
     *
     * @generated from field: bytes payload = 4;
     */
    payload: Uint8Array;
};
/**
 * Describes the message croupier.function.v1.JobEvent.
 * Use `create(JobEventSchema)` to create a new message.
 */
export declare const JobEventSchema: GenMessage<JobEvent>;
/**
 * @generated from message croupier.function.v1.CancelJobRequest
 */
export type CancelJobRequest = Message<"croupier.function.v1.CancelJobRequest"> & {
    /**
     * @generated from field: string job_id = 1;
     */
    jobId: string;
};
/**
 * Describes the message croupier.function.v1.CancelJobRequest.
 * Use `create(CancelJobRequestSchema)` to create a new message.
 */
export declare const CancelJobRequestSchema: GenMessage<CancelJobRequest>;
/**
 * @generated from service croupier.function.v1.FunctionService
 */
export declare const FunctionService: GenService<{
    /**
     * @generated from rpc croupier.function.v1.FunctionService.Invoke
     */
    invoke: {
        methodKind: "unary";
        input: typeof InvokeRequestSchema;
        output: typeof InvokeResponseSchema;
    };
    /**
     * @generated from rpc croupier.function.v1.FunctionService.StartJob
     */
    startJob: {
        methodKind: "unary";
        input: typeof InvokeRequestSchema;
        output: typeof StartJobResponseSchema;
    };
    /**
     * @generated from rpc croupier.function.v1.FunctionService.StreamJob
     */
    streamJob: {
        methodKind: "server_streaming";
        input: typeof JobStreamRequestSchema;
        output: typeof JobEventSchema;
    };
    /**
     * @generated from rpc croupier.function.v1.FunctionService.CancelJob
     */
    cancelJob: {
        methodKind: "unary";
        input: typeof CancelJobRequestSchema;
        output: typeof StartJobResponseSchema;
    };
}>;
//# sourceMappingURL=function_pb.d.ts.map