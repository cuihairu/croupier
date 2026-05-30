package io.github.cuihairu.croupier.sdk.wire;

import com.google.protobuf.CodedInputStream;
import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.WireFormat;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Minimal protobuf encoders/decoders for SDK wire messages.
 */
public final class SdkWireMessages {
    private SdkWireMessages() {
    }

    public static byte[] encodeInvokeRequest(InvokeRequest message) {
        return encode(out -> {
            writeString(out, 1, message.functionId);
            writeString(out, 2, message.idempotencyKey);
            writeBytes(out, 3, message.payload);
            for (Map.Entry<String, String> entry : message.metadata.entrySet()) {
                writeMessage(out, 4, encodeMapEntry(entry.getKey(), entry.getValue()));
            }
        });
    }

    public static InvokeRequest decodeInvokeRequest(byte[] data) {
        String functionId = "";
        String idempotencyKey = "";
        byte[] payload = new byte[0];
        Map<String, String> metadata = new LinkedHashMap<>();
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                switch (WireFormat.getTagFieldNumber(tag)) {
                    case 1 -> functionId = input.readString();
                    case 2 -> idempotencyKey = input.readString();
                    case 3 -> payload = input.readByteArray();
                    case 4 -> readMapEntry(input.readByteArray(), metadata);
                    default -> input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode InvokeRequest", e);
        }
        return new InvokeRequest(functionId, idempotencyKey, payload, metadata);
    }

    public static byte[] encodeInvokeResponse(InvokeResponse message) {
        return encode(out -> writeBytes(out, 1, message.payload));
    }

    public static InvokeResponse decodeInvokeResponse(byte[] data) {
        byte[] payload = new byte[0];
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                if (WireFormat.getTagFieldNumber(tag) == 1) {
                    payload = input.readByteArray();
                } else {
                    input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode InvokeResponse", e);
        }
        return new InvokeResponse(payload);
    }

    public static byte[] encodeStartJobResponse(StartJobResponse message) {
        return encode(out -> writeString(out, 1, message.jobId));
    }

    public static StartJobResponse decodeStartJobResponse(byte[] data) {
        String jobId = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                if (WireFormat.getTagFieldNumber(tag) == 1) {
                    jobId = input.readString();
                } else {
                    input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode StartJobResponse", e);
        }
        return new StartJobResponse(jobId);
    }

    public static byte[] encodeJobStreamRequest(JobStreamRequest message) {
        return encode(out -> writeString(out, 1, message.jobId));
    }

    public static JobStreamRequest decodeJobStreamRequest(byte[] data) {
        String jobId = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                if (WireFormat.getTagFieldNumber(tag) == 1) {
                    jobId = input.readString();
                } else {
                    input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode JobStreamRequest", e);
        }
        return new JobStreamRequest(jobId);
    }

    public static byte[] encodeJobEvent(JobEvent message) {
        return encode(out -> {
            writeString(out, 1, message.type);
            writeString(out, 2, message.message);
            writeInt32(out, 3, message.progress);
            writeBytes(out, 4, message.payload);
        });
    }

    public static JobEvent decodeJobEvent(byte[] data) {
        String type = "";
        String message = "";
        int progress = 0;
        byte[] payload = new byte[0];
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                switch (WireFormat.getTagFieldNumber(tag)) {
                    case 1 -> type = input.readString();
                    case 2 -> message = input.readString();
                    case 3 -> progress = input.readInt32();
                    case 4 -> payload = input.readByteArray();
                    default -> input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode JobEvent", e);
        }
        return new JobEvent(type, message, progress, payload);
    }

    public static byte[] encodeCancelJobRequest(CancelJobRequest message) {
        return encode(out -> writeString(out, 1, message.jobId));
    }

    public static CancelJobRequest decodeCancelJobRequest(byte[] data) {
        String jobId = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                if (WireFormat.getTagFieldNumber(tag) == 1) {
                    jobId = input.readString();
                } else {
                    input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode CancelJobRequest", e);
        }
        return new CancelJobRequest(jobId);
    }

    public static byte[] encodeRegisterLocalRequest(RegisterLocalRequest message) {
        return encode(out -> {
            writeString(out, 1, message.serviceId);
            writeString(out, 2, message.version);
            writeString(out, 3, message.rpcAddr);
            for (LocalFunctionDescriptor function : message.functions) {
                writeMessage(out, 4, encodeLocalFunctionDescriptor(function));
            }
        });
    }

    public static RegisterLocalRequest decodeRegisterLocalRequest(byte[] data) {
        String serviceId = "";
        String version = "";
        String rpcAddr = "";
        java.util.List<LocalFunctionDescriptor> functions = new java.util.ArrayList<>();
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                switch (WireFormat.getTagFieldNumber(tag)) {
                    case 1 -> serviceId = input.readString();
                    case 2 -> version = input.readString();
                    case 3 -> rpcAddr = input.readString();
                    case 4 -> functions.add(decodeLocalFunctionDescriptor(input.readByteArray()));
                    default -> input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode RegisterLocalRequest", e);
        }
        return new RegisterLocalRequest(serviceId, version, rpcAddr, functions);
    }

    public static byte[] encodeRegisterLocalResponse(RegisterLocalResponse message) {
        return encode(out -> writeString(out, 1, message.sessionId));
    }

    public static RegisterLocalResponse decodeRegisterLocalResponse(byte[] data) {
        String sessionId = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                if (WireFormat.getTagFieldNumber(tag) == 1) {
                    sessionId = input.readString();
                } else {
                    input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode RegisterLocalResponse", e);
        }
        return new RegisterLocalResponse(sessionId);
    }

    public static byte[] encodeHeartbeatRequest(HeartbeatRequest message) {
        return encode(out -> {
            writeString(out, 1, message.serviceId);
            writeString(out, 2, message.sessionId);
        });
    }

    public static HeartbeatRequest decodeHeartbeatRequest(byte[] data) {
        String serviceId = "";
        String sessionId = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                switch (WireFormat.getTagFieldNumber(tag)) {
                    case 1 -> serviceId = input.readString();
                    case 2 -> sessionId = input.readString();
                    default -> input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode HeartbeatRequest", e);
        }
        return new HeartbeatRequest(serviceId, sessionId);
    }

    private static byte[] encode(Encoder encoder) {
        try {
            ByteArrayOutputStream output = new ByteArrayOutputStream();
            CodedOutputStream codedOutput = CodedOutputStream.newInstance(output);
            encoder.writeTo(codedOutput);
            codedOutput.flush();
            return output.toByteArray();
        } catch (IOException e) {
            throw new IllegalStateException("Failed to encode protobuf message", e);
        }
    }

    private static CodedInputStream newInput(byte[] data) {
        return CodedInputStream.newInstance(data == null ? new byte[0] : data);
    }

    private static void writeString(CodedOutputStream out, int fieldNumber, String value) throws IOException {
        if (value != null && !value.isEmpty()) {
            out.writeString(fieldNumber, value);
        }
    }

    private static void writeBytes(CodedOutputStream out, int fieldNumber, byte[] value) throws IOException {
        if (value != null && value.length > 0) {
            out.writeByteArray(fieldNumber, value);
        }
    }

    private static void writeInt32(CodedOutputStream out, int fieldNumber, int value) throws IOException {
        if (value != 0) {
            out.writeInt32(fieldNumber, value);
        }
    }

    private static void writeBool(CodedOutputStream out, int fieldNumber, boolean value) throws IOException {
        if (value) {
            out.writeBool(fieldNumber, true);
        }
    }

    private static void writeMessage(CodedOutputStream out, int fieldNumber, byte[] value) throws IOException {
        if (value != null && value.length > 0) {
            out.writeByteArray(fieldNumber, value);
        }
    }

    private static byte[] encodeMapEntry(String key, String value) {
        return encode(out -> {
            writeString(out, 1, key);
            writeString(out, 2, value);
        });
    }

    private static byte[] encodeLocalFunctionDescriptor(LocalFunctionDescriptor message) {
        return encode(out -> {
            writeString(out, 1, message.id);
            writeString(out, 2, message.version);
            for (String tag : message.tags) {
                writeString(out, 3, tag);
            }
            writeString(out, 4, message.summary);
            writeString(out, 5, message.description);
            writeString(out, 6, message.operationId);
            writeBool(out, 7, message.deprecated);
            writeString(out, 8, message.inputSchema);
            writeString(out, 9, message.outputSchema);
            writeString(out, 10, message.category);
            writeString(out, 11, message.risk);
            writeString(out, 12, message.entity);
            writeString(out, 13, message.operation);
        });
    }

    private static LocalFunctionDescriptor decodeLocalFunctionDescriptor(byte[] data) {
        String id = "";
        String version = "";
        java.util.List<String> tags = new java.util.ArrayList<>();
        String summary = "";
        String description = "";
        String operationId = "";
        boolean deprecated = false;
        String inputSchema = "";
        String outputSchema = "";
        String category = "";
        String risk = "";
        String entity = "";
        String operation = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                switch (WireFormat.getTagFieldNumber(tag)) {
                    case 1 -> id = input.readString();
                    case 2 -> version = input.readString();
                    case 3 -> tags.add(input.readString());
                    case 4 -> summary = input.readString();
                    case 5 -> description = input.readString();
                    case 6 -> operationId = input.readString();
                    case 7 -> deprecated = input.readBool();
                    case 8 -> inputSchema = input.readString();
                    case 9 -> outputSchema = input.readString();
                    case 10 -> category = input.readString();
                    case 11 -> risk = input.readString();
                    case 12 -> entity = input.readString();
                    case 13 -> operation = input.readString();
                    default -> input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode LocalFunctionDescriptor", e);
        }
        return new LocalFunctionDescriptor(
            id,
            version,
            tags,
            summary,
            description,
            operationId,
            deprecated,
            inputSchema,
            outputSchema,
            category,
            risk,
            entity,
            operation
        );
    }

    private static void readMapEntry(byte[] data, Map<String, String> target) {
        String key = "";
        String value = "";
        CodedInputStream input = newInput(data);
        try {
            while (!input.isAtEnd()) {
                int tag = input.readTag();
                if (tag == 0) {
                    break;
                }
                switch (WireFormat.getTagFieldNumber(tag)) {
                    case 1 -> key = input.readString();
                    case 2 -> value = input.readString();
                    default -> input.skipField(tag);
                }
            }
        } catch (IOException e) {
            throw new IllegalArgumentException("Failed to decode metadata entry", e);
        }
        if (!key.isEmpty()) {
            target.put(key, value);
        }
    }

    private static byte[] copy(byte[] value) {
        return value == null ? new byte[0] : Arrays.copyOf(value, value.length);
    }

    @FunctionalInterface
    private interface Encoder {
        void writeTo(CodedOutputStream out) throws IOException;
    }

    public static final class InvokeRequest {
        public final String functionId;
        public final String idempotencyKey;
        public final byte[] payload;
        public final Map<String, String> metadata;

        public InvokeRequest(String functionId, String idempotencyKey, byte[] payload, Map<String, String> metadata) {
            this.functionId = functionId == null ? "" : functionId;
            this.idempotencyKey = idempotencyKey == null ? "" : idempotencyKey;
            this.payload = copy(payload);
            this.metadata = metadata == null ? Map.of() : Map.copyOf(metadata);
        }
    }

    public static final class InvokeResponse {
        public final byte[] payload;

        public InvokeResponse(byte[] payload) {
            this.payload = copy(payload);
        }

        public String payloadUtf8() {
            return new String(payload, StandardCharsets.UTF_8);
        }
    }

    public static final class StartJobResponse {
        public final String jobId;

        public StartJobResponse(String jobId) {
            this.jobId = jobId == null ? "" : jobId;
        }
    }

    public static final class JobStreamRequest {
        public final String jobId;

        public JobStreamRequest(String jobId) {
            this.jobId = jobId == null ? "" : jobId;
        }
    }

    public static final class JobEvent {
        public final String type;
        public final String message;
        public final int progress;
        public final byte[] payload;

        public JobEvent(String type, String message, int progress, byte[] payload) {
            this.type = type == null ? "" : type;
            this.message = message == null ? "" : message;
            this.progress = progress;
            this.payload = copy(payload);
        }

        public String payloadUtf8() {
            return new String(payload, StandardCharsets.UTF_8);
        }
    }

    public static final class CancelJobRequest {
        public final String jobId;

        public CancelJobRequest(String jobId) {
            this.jobId = jobId == null ? "" : jobId;
        }
    }

    public static final class RegisterLocalRequest {
        public final String serviceId;
        public final String version;
        public final String rpcAddr;
        public final java.util.List<LocalFunctionDescriptor> functions;

        public RegisterLocalRequest(String serviceId, String version, String rpcAddr,
                                    java.util.List<LocalFunctionDescriptor> functions) {
            this.serviceId = serviceId == null ? "" : serviceId;
            this.version = version == null ? "" : version;
            this.rpcAddr = rpcAddr == null ? "" : rpcAddr;
            this.functions = functions == null ? java.util.List.of() : java.util.List.copyOf(functions);
        }
    }

    public static final class RegisterLocalResponse {
        public final String sessionId;

        public RegisterLocalResponse(String sessionId) {
            this.sessionId = sessionId == null ? "" : sessionId;
        }
    }

    public static final class HeartbeatRequest {
        public final String serviceId;
        public final String sessionId;

        public HeartbeatRequest(String serviceId, String sessionId) {
            this.serviceId = serviceId == null ? "" : serviceId;
            this.sessionId = sessionId == null ? "" : sessionId;
        }
    }

    public static final class LocalFunctionDescriptor {
        public final String id;
        public final String version;
        public final java.util.List<String> tags;
        public final String summary;
        public final String description;
        public final String operationId;
        public final boolean deprecated;
        public final String inputSchema;
        public final String outputSchema;
        public final String category;
        public final String risk;
        public final String entity;
        public final String operation;

        public LocalFunctionDescriptor(String id, String version, java.util.List<String> tags, String summary,
                                       String description, String operationId, boolean deprecated,
                                       String inputSchema, String outputSchema, String category, String risk,
                                       String entity, String operation) {
            this.id = id == null ? "" : id;
            this.version = version == null ? "" : version;
            this.tags = tags == null ? java.util.List.of() : java.util.List.copyOf(tags);
            this.summary = summary == null ? "" : summary;
            this.description = description == null ? "" : description;
            this.operationId = operationId == null ? "" : operationId;
            this.deprecated = deprecated;
            this.inputSchema = inputSchema == null ? "" : inputSchema;
            this.outputSchema = outputSchema == null ? "" : outputSchema;
            this.category = category == null ? "" : category;
            this.risk = risk == null ? "" : risk;
            this.entity = entity == null ? "" : entity;
            this.operation = operation == null ? "" : operation;
        }
    }
}
