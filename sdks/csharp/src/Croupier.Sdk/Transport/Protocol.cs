// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

namespace Croupier.Sdk.Transport;

/// <summary>
/// Croupier wire protocol implementation for TCP transport.
///
/// Message Format:
/// Header (8 bytes):
///   ┌─────────┬──────────┬─────────────────┐
///   │ Version │ MsgID    │ RequestID       │
///   │ (1B)    │ (3B)     │ (4B)            │
///   └─────────┴──────────┴─────────────────┘
/// Body: protobuf serialized message
///
/// Request messages have odd MsgID, Response messages have even MsgID.
/// </summary>
public static class Protocol
{
    /// <summary>
    /// Protocol version 1.
    /// </summary>
    public const byte Version1 = 0x01;

    /// <summary>
    /// Header size: Version(1) + MsgID(3) + RequestID(4).
    /// </summary>
    public const int HeaderSize = 8;

    // Message type constants (24 bits)

    // ControlService (0x01xx)
    public const int MsgRegisterRequest = 0x010101;
    public const int MsgRegisterResponse = 0x010102;
    public const int MsgHeartbeatRequest = 0x010103;
    public const int MsgHeartbeatResponse = 0x010104;
    public const int MsgRegisterCapabilitiesReq = 0x010105;
    public const int MsgRegisterCapabilitiesResp = 0x010106;

    // ClientService (0x02xx)
    public const int MsgRegisterClientRequest = 0x020101;
    public const int MsgRegisterClientResponse = 0x020102;
    public const int MsgClientHeartbeatRequest = 0x020103;
    public const int MsgClientHeartbeatResponse = 0x020104;

    // InvokerService (0x03xx)
    public const int MsgInvokeRequest = 0x030101;
    public const int MsgInvokeResponse = 0x030102;
    public const int MsgStartTaskRequest = 0x030103;
    public const int MsgStartTaskResponse = 0x030104;
    public const int MsgStreamTaskRequest = 0x030105;
    public const int MsgTaskEvent = 0x030106;
    public const int MsgCancelTaskRequest = 0x030107;
    public const int MsgCancelTaskResponse = 0x030108;

    // ProviderService (0x05xx) - New session-based protocol
    public const int MsgProviderConnectRequest = 0x050101;
    public const int MsgProviderConnectResponse = 0x050102;
    public const int MsgProviderHeartbeatRequest = 0x050103;
    public const int MsgProviderHeartbeatResponse = 0x050104;
    public const int MsgProviderDrainRequest = 0x050105;
    public const int MsgProviderDrainResponse = 0x050106;
    public const int MsgGetTaskResultRequest = 0x050107;
    public const int MsgGetTaskResultResponse = 0x050108;

    // Legacy aliases — prefer MsgProvider* canonical names.
    // Do not use in new code; will be removed once all call sites migrate.
    public const int MsgRegisterLocalRequest = MsgProviderConnectRequest;
    public const int MsgRegisterLocalResponse = MsgProviderConnectResponse;
    public const int MsgHeartbeatLocalRequest = MsgProviderHeartbeatRequest;
    public const int MsgHeartbeatLocalResponse = MsgProviderHeartbeatResponse;
    public const int MsgListLocalRequest = 0x050109;
    public const int MsgListLocalResponse = 0x05010a;

    // Legacy Job aliases for backward compatibility
    public const int MsgStartJobRequest = MsgStartTaskRequest;
    public const int MsgStartJobResponse = MsgStartTaskResponse;
    public const int MsgStreamJobRequest = MsgStreamTaskRequest;
    public const int MsgJobEvent = MsgTaskEvent;
    public const int MsgCancelJobRequest = MsgCancelTaskRequest;
    public const int MsgCancelJobResponse = MsgCancelTaskResponse;

    /// <summary>
    /// Encode a 24-bit MsgID into 3 bytes (big-endian).
    /// </summary>
    public static void PutMsgId(byte[] buf, int offset, int msgId)
    {
        buf[offset] = (byte)((msgId >> 16) & 0xFF);
        buf[offset + 1] = (byte)((msgId >> 8) & 0xFF);
        buf[offset + 2] = (byte)(msgId & 0xFF);
    }

    /// <summary>
    /// Decode a 24-bit MsgID from 3 bytes (big-endian).
    /// </summary>
    public static int GetMsgId(byte[] buf, int offset)
    {
        return ((buf[offset] & 0xFF) << 16) |
               ((buf[offset + 1] & 0xFF) << 8) |
               (buf[offset + 2] & 0xFF);
    }

    /// <summary>
    /// Create a new message with protocol header and body.
    /// </summary>
    public static byte[] NewMessage(int msgId, int reqId, byte[]? body)
    {
        var message = new byte[HeaderSize + (body?.Length ?? 0)];

        // Header
        message[0] = Version1;
        PutMsgId(message, 1, msgId);

        // RequestID (big-endian)
        message[4] = (byte)((reqId >> 24) & 0xFF);
        message[5] = (byte)((reqId >> 16) & 0xFF);
        message[6] = (byte)((reqId >> 8) & 0xFF);
        message[7] = (byte)(reqId & 0xFF);

        // Body
        if (body != null && body.Length > 0)
        {
            Array.Copy(body, 0, message, HeaderSize, body.Length);
        }

        return message;
    }

    /// <summary>
    /// Parsed message components.
    /// </summary>
    public readonly struct ParsedMessage
    {
        public readonly byte Version;
        public readonly int MsgId;
        public readonly int ReqId;
        public readonly byte[] Body;

        public ParsedMessage(byte version, int msgId, int reqId, byte[]? body)
        {
            Version = version;
            MsgId = msgId;
            ReqId = reqId;
            // Ensure Body is never null - use empty array if null is provided
            Body = body ?? Array.Empty<byte>();
        }
    }

    /// <summary>
    /// Parse a received message.
    /// </summary>
    public static ParsedMessage ParseMessage(byte[] data)
    {
        if (data.Length < HeaderSize)
        {
            throw new ArgumentException($"Message too short: {data.Length}", nameof(data));
        }

        var version = data[0];
        var msgId = GetMsgId(data, 1);
        var reqId = ((data[4] & 0xFF) << 24) |
                    ((data[5] & 0xFF) << 16) |
                    ((data[6] & 0xFF) << 8) |
                    (data[7] & 0xFF);

        var body = new byte[data.Length - HeaderSize];
        Array.Copy(data, HeaderSize, body, 0, body.Length);

        return new ParsedMessage(version, msgId, reqId, body);
    }

    /// <summary>
    /// Check if the MsgID indicates a request message.
    /// </summary>
    public static bool IsRequest(int msgId)
    {
        return msgId % 2 == 1 && msgId != MsgTaskEvent;
    }

    /// <summary>
    /// Check if the MsgID indicates a response message.
    /// </summary>
    public static bool IsResponse(int msgId)
    {
        return msgId % 2 == 0 && msgId != MsgTaskEvent;
    }

    /// <summary>
    /// Get the response MsgID for a given request MsgID.
    /// </summary>
    public static int GetResponseMsgId(int reqMsgId)
    {
        return reqMsgId + 1;
    }

    /// <summary>
    /// Get human-readable string for MsgID.
    /// </summary>
    public static string MsgIdString(int msgId)
    {
        return msgId switch
        {
            MsgRegisterRequest => "RegisterRequest",
            MsgRegisterResponse => "RegisterResponse",
            MsgHeartbeatRequest => "HeartbeatRequest",
            MsgHeartbeatResponse => "HeartbeatResponse",
            MsgInvokeRequest => "InvokeRequest",
            MsgInvokeResponse => "InvokeResponse",
            MsgStartTaskRequest => "StartTaskRequest",
            MsgStartTaskResponse => "StartTaskResponse",
            MsgStreamTaskRequest => "StreamTaskRequest",
            MsgTaskEvent => "TaskEvent",
            MsgCancelTaskRequest => "CancelTaskRequest",
            MsgCancelTaskResponse => "CancelTaskResponse",
            MsgProviderConnectRequest => "ProviderConnectRequest",
            MsgProviderConnectResponse => "ProviderConnectResponse",
            MsgProviderHeartbeatRequest => "ProviderHeartbeatRequest",
            MsgProviderHeartbeatResponse => "ProviderHeartbeatResponse",
            MsgProviderDrainRequest => "ProviderDrainRequest",
            MsgProviderDrainResponse => "ProviderDrainResponse",
            MsgGetTaskResultRequest => "GetTaskResultRequest",
            MsgGetTaskResultResponse => "GetTaskResultResponse",
            MsgListLocalRequest => "ListLocalRequest",
            MsgListLocalResponse => "ListLocalResponse",
            _ => $"Unknown(0x{msgId:X6})"
        };
    }
}
