// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// Comprehensive tests for Protocol message handling
/// </summary>
public class ProtocolComprehensiveTests
{
    #region Constants Tests

    [Fact]
    public void Version1_IsCorrect()
    {
        Protocol.Version1.Should().Be(0x01);
    }

    [Fact]
    public void HeaderSize_IsCorrect()
    {
        Protocol.HeaderSize.Should().Be(8);
    }

    #endregion

    #region Message Type Constants Tests

    [Fact]
    public void ControlService_MessageTypes_AreOdd()
    {
        Protocol.MsgRegisterRequest.Should().Be(0x010101);
        Protocol.MsgRegisterResponse.Should().Be(0x010102);
        Protocol.MsgHeartbeatRequest.Should().Be(0x010103);
        Protocol.MsgHeartbeatResponse.Should().Be(0x010104);
        Protocol.MsgRegisterCapabilitiesReq.Should().Be(0x010105);
        Protocol.MsgRegisterCapabilitiesResp.Should().Be(0x010106);
    }

    [Fact]
    public void ClientService_MessageTypes_AreCorrect()
    {
        Protocol.MsgRegisterClientRequest.Should().Be(0x020101);
        Protocol.MsgRegisterClientResponse.Should().Be(0x020102);
        Protocol.MsgClientHeartbeatRequest.Should().Be(0x020103);
        Protocol.MsgClientHeartbeatResponse.Should().Be(0x020104);
    }

    [Fact]
    public void InvokerService_MessageTypes_AreCorrect()
    {
        Protocol.MsgInvokeRequest.Should().Be(0x030101);
        Protocol.MsgInvokeResponse.Should().Be(0x030102);
        Protocol.MsgStartTaskRequest.Should().Be(0x030103);
        Protocol.MsgStartTaskResponse.Should().Be(0x030104);
        Protocol.MsgStreamTaskRequest.Should().Be(0x030105);
        Protocol.MsgTaskEvent.Should().Be(0x030106);
        Protocol.MsgCancelTaskRequest.Should().Be(0x030107);
        Protocol.MsgCancelTaskResponse.Should().Be(0x030108);
    }

    [Fact]
    public void ProviderService_MessageTypes_AreCorrect()
    {
        Protocol.MsgProviderConnectRequest.Should().Be(0x050101);
        Protocol.MsgProviderConnectResponse.Should().Be(0x050102);
        Protocol.MsgProviderHeartbeatRequest.Should().Be(0x050103);
        Protocol.MsgProviderHeartbeatResponse.Should().Be(0x050104);
        Protocol.MsgProviderDrainRequest.Should().Be(0x050105);
        Protocol.MsgProviderDrainResponse.Should().Be(0x050106);
        Protocol.MsgGetTaskResultRequest.Should().Be(0x050107);
        Protocol.MsgGetTaskResultResponse.Should().Be(0x050108);
    }

    #endregion

    #region PutMsgId Tests

    [Fact]
    public void PutMsgId_EncodesCorrectly()
    {
        var buffer = new byte[3];
        Protocol.PutMsgId(buffer, 0, 0x123456);

        buffer[0].Should().Be(0x12);
        buffer[1].Should().Be(0x34);
        buffer[2].Should().Be(0x56);
    }

    [Fact]
    public void PutMsgId_WithOffset_EncodesCorrectly()
    {
        var buffer = new byte[5];
        Protocol.PutMsgId(buffer, 2, 0xABCDEF);

        buffer[2].Should().Be(0xAB);
        buffer[3].Should().Be(0xCD);
        buffer[4].Should().Be(0xEF);
    }

    [Fact]
    public void PutMsgId_MinValue_EncodesCorrectly()
    {
        var buffer = new byte[3];
        Protocol.PutMsgId(buffer, 0, 0x000000);

        buffer[0].Should().Be(0x00);
        buffer[1].Should().Be(0x00);
        buffer[2].Should().Be(0x00);
    }

    [Fact]
    public void PutMsgId_MaxValue_EncodesCorrectly()
    {
        var buffer = new byte[3];
        Protocol.PutMsgId(buffer, 0, 0xFFFFFF);

        buffer[0].Should().Be(0xFF);
        buffer[1].Should().Be(0xFF);
        buffer[2].Should().Be(0xFF);
    }

    [Fact]
    public void PutMsgId_TruncatesTo24Bits()
    {
        var buffer = new byte[3];
        Protocol.PutMsgId(buffer, 0, 0x12345678); // 32-bit value

        buffer[0].Should().Be(0x34);
        buffer[1].Should().Be(0x56);
        buffer[2].Should().Be(0x78);
    }

    #endregion

    #region GetMsgId Tests

    [Fact]
    public void GetMsgId_DecodesCorrectly()
    {
        var buffer = new byte[] { 0x12, 0x34, 0x56 };
        var msgId = Protocol.GetMsgId(buffer, 0);

        msgId.Should().Be(0x123456);
    }

    [Fact]
    public void GetMsgId_WithOffset_DecodesCorrectly()
    {
        var buffer = new byte[] { 0x00, 0x00, 0xAB, 0xCD, 0xEF };
        var msgId = Protocol.GetMsgId(buffer, 2);

        msgId.Should().Be(0xABCDEF);
    }

    [Fact]
    public void GetMsgId_MinValue_DecodesCorrectly()
    {
        var buffer = new byte[] { 0x00, 0x00, 0x00 };
        var msgId = Protocol.GetMsgId(buffer, 0);

        msgId.Should().Be(0x000000);
    }

    [Fact]
    public void GetMsgId_MaxValue_DecodesCorrectly()
    {
        var buffer = new byte[] { 0xFF, 0xFF, 0xFF };
        var msgId = Protocol.GetMsgId(buffer, 0);

        msgId.Should().Be(0xFFFFFF);
    }

    [Fact]
    public void GetMsgId_RoundTrip_PreservesValue()
    {
        var originalMsgId = 0xABCDEF;
        var buffer = new byte[3];

        Protocol.PutMsgId(buffer, 0, originalMsgId);
        var decodedMsgId = Protocol.GetMsgId(buffer, 0);

        decodedMsgId.Should().Be(originalMsgId);
    }

    #endregion

    #region NewMessage Tests

    [Fact]
    public void NewMessage_WithNullBody_CreatesHeaderOnly()
    {
        var message = Protocol.NewMessage(0x010101, 12345, null);

        message.Should().HaveCount(Protocol.HeaderSize);
        message[0].Should().Be(Protocol.Version1);
    }

    [Fact]
    public void NewMessage_WithEmptyBody_CreatesHeaderOnly()
    {
        var message = Protocol.NewMessage(0x010101, 12345, Array.Empty<byte>());

        message.Should().HaveCount(Protocol.HeaderSize);
        message[0].Should().Be(Protocol.Version1);
    }

    [Fact]
    public void NewMessage_WithBody_CreatesHeaderAndBody()
    {
        var body = new byte[] { 0x01, 0x02, 0x03, 0x04 };
        var message = Protocol.NewMessage(0x010101, 12345, body);

        message.Should().HaveCount(Protocol.HeaderSize + body.Length);
        message[0].Should().Be(Protocol.Version1);
    }

    [Fact]
    public void NewMessage_EncodesMsgIdCorrectly()
    {
        var message = Protocol.NewMessage(0x123456, 0, null);
        var msgId = Protocol.GetMsgId(message, 1);

        msgId.Should().Be(0x123456);
    }

    [Fact]
    public void NewMessage_EncodesRequestIdBigEndian()
    {
        var message = Protocol.NewMessage(0, 0x12345678, null);

        message[4].Should().Be(0x12);
        message[5].Should().Be(0x34);
        message[6].Should().Be(0x56);
        message[7].Should().Be(0x78);
    }

    [Fact]
    public void NewMessage_WithBody_CopiesBodyCorrectly()
    {
        var body = new byte[] { 0xAA, 0xBB, 0xCC };
        var message = Protocol.NewMessage(0, 0, body);

        message[Protocol.HeaderSize].Should().Be(0xAA);
        message[Protocol.HeaderSize + 1].Should().Be(0xBB);
        message[Protocol.HeaderSize + 2].Should().Be(0xCC);
    }

    [Fact]
    public void NewMessage_ZeroRequestId_Works()
    {
        var message = Protocol.NewMessage(0x010101, 0, null);

        var reqId = (message[4] << 24) | (message[5] << 16) | (message[6] << 8) | message[7];
        reqId.Should().Be(0);
    }

    [Fact]
    public void NewMessage_MaxRequestId_Works()
    {
        var message = Protocol.NewMessage(0x010101, -1, null);

        var reqId = (message[4] << 24) | (message[5] << 16) | (message[6] << 8) | message[7];
        reqId.Should().Be(unchecked((int)0xFFFFFFFF));
    }

    #endregion

    #region ParseMessage Tests

    [Fact]
    public void ParseMessage_WithValidMessage_ParsesCorrectly()
    {
        var originalMsg = Protocol.NewMessage(0x010101, 12345, new byte[] { 0x01, 0x02 });
        var parsed = Protocol.ParseMessage(originalMsg);

        parsed.Version.Should().Be(Protocol.Version1);
        parsed.MsgId.Should().Be(0x010101);
        parsed.ReqId.Should().Be(12345);
        parsed.Body.Should().BeEquivalentTo(new byte[] { 0x01, 0x02 });
    }

    [Fact]
    public void ParseMessage_WithHeaderOnly_ParsesCorrectly()
    {
        var message = new byte[Protocol.HeaderSize];
        message[0] = Protocol.Version1;

        var parsed = Protocol.ParseMessage(message);

        parsed.Version.Should().Be(Protocol.Version1);
        parsed.MsgId.Should().Be(0);
        parsed.ReqId.Should().Be(0);
        parsed.Body.Should().BeEmpty();
    }

    [Fact]
    public void ParseMessage_WithTooShortMessage_ThrowsArgumentException()
    {
        var shortMessage = new byte[Protocol.HeaderSize - 1];

        var action = () => Protocol.ParseMessage(shortMessage);

        action.Should().Throw<ArgumentException>()
            .WithMessage("*too short*");
    }

    [Fact]
    public void ParseMessage_WithEmptyArray_ThrowsArgumentException()
    {
        var action = () => Protocol.ParseMessage(Array.Empty<byte>());

        action.Should().Throw<ArgumentException>()
            .WithMessage("*too short*");
    }

    [Fact]
    public void ParseMessage_RoundTrip_PreservesData()
    {
        var originalBody = new byte[] { 0xDE, 0xAD, 0xBE, 0xEF };
        var originalMsg = Protocol.NewMessage(0x030101, 0x12345678, originalBody);
        var parsed = Protocol.ParseMessage(originalMsg);

        parsed.MsgId.Should().Be(0x030101);
        parsed.ReqId.Should().Be(0x12345678);
        parsed.Body.Should().BeEquivalentTo(originalBody);
    }

    [Fact]
    public void ParseMessage_WithLargeBody_ParsesCorrectly()
    {
        var largeBody = new byte[10000];
        for (int i = 0; i < largeBody.Length; i++)
            largeBody[i] = (byte)(i % 256);

        var message = Protocol.NewMessage(0x010101, 0, largeBody);
        var parsed = Protocol.ParseMessage(message);

        parsed.Body.Should().HaveCount(10000);
        parsed.Body.Should().BeEquivalentTo(largeBody);
    }

    #endregion

    #region IsRequest Tests

    [Fact]
    public void IsRequest_WithOddMsgId_ReturnsTrue()
    {
        Protocol.IsRequest(0x010101).Should().BeTrue();
        Protocol.IsRequest(0x030101).Should().BeTrue();
        Protocol.IsRequest(0x050101).Should().BeTrue();
        Protocol.IsRequest(1).Should().BeTrue();
        Protocol.IsRequest(99999).Should().BeTrue(); // Odd number
    }

    [Fact]
    public void IsRequest_WithEvenMsgId_ReturnsFalse()
    {
        Protocol.IsRequest(0x010102).Should().BeFalse();
        Protocol.IsRequest(0x030102).Should().BeFalse();
        Protocol.IsRequest(2).Should().BeFalse();
        Protocol.IsRequest(10000).Should().BeFalse(); // Even number
    }

    [Fact]
    public void IsRequest_WithTaskEvent_ReturnsFalse()
    {
        Protocol.IsRequest(Protocol.MsgTaskEvent).Should().BeFalse();
    }

    #endregion

    #region IsResponse Tests

    [Fact]
    public void IsResponse_WithEvenMsgId_ReturnsTrue()
    {
        Protocol.IsResponse(0x010102).Should().BeTrue();
        Protocol.IsResponse(0x030102).Should().BeTrue();
        Protocol.IsResponse(2).Should().BeTrue();
        Protocol.IsResponse(10000).Should().BeTrue(); // Even number
    }

    [Fact]
    public void IsResponse_WithOddMsgId_ReturnsFalse()
    {
        Protocol.IsResponse(0x010101).Should().BeFalse();
        Protocol.IsResponse(0x030101).Should().BeFalse();
        Protocol.IsResponse(1).Should().BeFalse();
        Protocol.IsResponse(99999).Should().BeFalse(); // Odd number
    }

    [Fact]
    public void IsResponse_WithTaskEvent_ReturnsFalse()
    {
        Protocol.IsResponse(Protocol.MsgTaskEvent).Should().BeFalse();
    }

    #endregion

    #region GetResponseMsgId Tests

    [Fact]
    public void GetResponseMsgId_WithRequest_ReturnsNextOdd()
    {
        Protocol.GetResponseMsgId(0x010101).Should().Be(0x010102);
        Protocol.GetResponseMsgId(0x030101).Should().Be(0x030102);
        Protocol.GetResponseMsgId(0x050101).Should().Be(0x050102);
    }

    [Fact]
    public void GetResponseMsgId_AlwaysReturnsEven()
    {
        for (int i = 1; i < 100; i += 2)
        {
            var responseId = Protocol.GetResponseMsgId(i);
            (responseId % 2).Should().Be(0);
        }
    }

    [Fact]
    public void GetResponseMsgId_IsInverseOfIsRequest()
    {
        var requestId = 0x030101;
        var responseId = Protocol.GetResponseMsgId(requestId);

        Protocol.IsRequest(requestId).Should().BeTrue();
        Protocol.IsResponse(responseId).Should().BeTrue();
    }

    #endregion

    #region MsgIdString Tests

    [Fact]
    public void MsgIdString_WithKnownMessages_ReturnsCorrectNames()
    {
        Protocol.MsgIdString(Protocol.MsgRegisterRequest).Should().Be("RegisterRequest");
        Protocol.MsgIdString(Protocol.MsgRegisterResponse).Should().Be("RegisterResponse");
        Protocol.MsgIdString(Protocol.MsgHeartbeatRequest).Should().Be("HeartbeatRequest");
        Protocol.MsgIdString(Protocol.MsgHeartbeatResponse).Should().Be("HeartbeatResponse");
        Protocol.MsgIdString(Protocol.MsgInvokeRequest).Should().Be("InvokeRequest");
        Protocol.MsgIdString(Protocol.MsgInvokeResponse).Should().Be("InvokeResponse");
        Protocol.MsgIdString(Protocol.MsgStartTaskRequest).Should().Be("StartTaskRequest");
        Protocol.MsgIdString(Protocol.MsgStartTaskResponse).Should().Be("StartTaskResponse");
        Protocol.MsgIdString(Protocol.MsgStreamTaskRequest).Should().Be("StreamTaskRequest");
        Protocol.MsgIdString(Protocol.MsgTaskEvent).Should().Be("TaskEvent");
        Protocol.MsgIdString(Protocol.MsgCancelTaskRequest).Should().Be("CancelTaskRequest");
        Protocol.MsgIdString(Protocol.MsgCancelTaskResponse).Should().Be("CancelTaskResponse");
    }

    [Fact]
    public void MsgIdString_WithProviderMessages_ReturnsCorrectNames()
    {
        Protocol.MsgIdString(Protocol.MsgProviderConnectRequest).Should().Be("ProviderConnectRequest");
        Protocol.MsgIdString(Protocol.MsgProviderConnectResponse).Should().Be("ProviderConnectResponse");
        Protocol.MsgIdString(Protocol.MsgProviderHeartbeatRequest).Should().Be("ProviderHeartbeatRequest");
        Protocol.MsgIdString(Protocol.MsgProviderHeartbeatResponse).Should().Be("ProviderHeartbeatResponse");
        Protocol.MsgIdString(Protocol.MsgProviderDrainRequest).Should().Be("ProviderDrainRequest");
        Protocol.MsgIdString(Protocol.MsgProviderDrainResponse).Should().Be("ProviderDrainResponse");
    }

    [Fact]
    public void MsgIdString_WithUnknownMessage_ReturnsHexFormat()
    {
        var result = Protocol.MsgIdString(unchecked((int)0xDEADBEEF));

        result.Should().Be("Unknown(0xDEADBEEF)");
    }

    [Fact]
    public void MsgIdString_WithZero_ReturnsHexFormat()
    {
        var result = Protocol.MsgIdString(0);

        result.Should().Be("Unknown(0x000000)");
    }

    #endregion

    #region ParsedMessage Struct Tests

    [Fact]
    public void ParsedMessage_CanBeCreated()
    {
        var body = new byte[] { 0x01, 0x02 };
        var parsed = new Protocol.ParsedMessage(1, 0x010101, 123, body);

        parsed.Version.Should().Be(1);
        parsed.MsgId.Should().Be(0x010101);
        parsed.ReqId.Should().Be(123);
        parsed.Body.Should().BeEquivalentTo(body);
    }

    [Fact]
    public void ParsedMessage_WithEmptyBody_CanBeCreated()
    {
        var parsed = new Protocol.ParsedMessage(1, 0x010101, 123, Array.Empty<byte>());

        parsed.Body.Should().BeEmpty();
    }

    [Fact]
    public void ParsedMessage_WithNullBody_HandledCorrectly()
    {
        var parsed = new Protocol.ParsedMessage(1, 0x010101, 123, null!);

        parsed.Body.Should().NotBeNull();
    }

    #endregion
}
