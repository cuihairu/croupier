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

using Croupier.Sdk.Transport;
using Xunit;

namespace Croupier.Sdk.Tests.Transport;

public class ProtocolTests
{
    [Fact]
    public void TestNewMessage()
    {
        int msgType = Protocol.MsgInvokeRequest;
        int reqId = 12345;
        byte[] body = System.Text.Encoding.UTF8.GetBytes("test payload");

        byte[] message = Protocol.NewMessage(msgType, reqId, body);

        Assert.Equal(Protocol.HeaderSize + body.Length, message.Length);
        Assert.Equal(Protocol.Version1, message[0]);
    }

    [Fact]
    public void TestNewMessageWithNullBody()
    {
        int msgType = Protocol.MsgHeartbeatRequest;
        int reqId = 999;

        byte[] message = Protocol.NewMessage(msgType, reqId, null);

        Assert.Equal(Protocol.HeaderSize, message.Length);
        Assert.Equal(Protocol.Version1, message[0]);
    }

    [Fact]
    public void TestParseMessage()
    {
        int msgType = Protocol.MsgProviderConnectRequest;
        int reqId = 999;
        byte[] body = System.Text.Encoding.UTF8.GetBytes("hello world");

        byte[] message = Protocol.NewMessage(msgType, reqId, body);
        var parsed = Protocol.ParseMessage(message);

        Assert.Equal(Protocol.Version1, parsed.Version);
        Assert.Equal(msgType, parsed.MsgId);
        Assert.Equal(reqId, parsed.ReqId);
        Assert.Equal(body, parsed.Body);
    }

    [Fact]
    public void TestParseMessageEmptyBody()
    {
        int msgType = Protocol.MsgHeartbeatRequest;
        int reqId = 12345;

        byte[] message = Protocol.NewMessage(msgType, reqId, Array.Empty<byte>());
        var parsed = Protocol.ParseMessage(message);

        Assert.Equal(Protocol.Version1, parsed.Version);
        Assert.Equal(msgType, parsed.MsgId);
        Assert.Equal(reqId, parsed.ReqId);
        Assert.Empty(parsed.Body);
    }

    [Fact]
    public void TestParseMessageTooShort()
    {
        byte[] shortMessage = new byte[] { 0x01, 0x02, 0x03 };

        Assert.Throws<ArgumentException>(() => Protocol.ParseMessage(shortMessage));
    }

    [Fact]
    public void TestPutAndGetMsgId()
    {
        int msgId = 0x030101;
        byte[] buf = new byte[3];
        Protocol.PutMsgId(buf, 0, msgId);
        int decoded = Protocol.GetMsgId(buf, 0);

        Assert.Equal(msgId, decoded);
    }

    [Fact]
    public void TestPutAndGetMsgIdMaxValue()
    {
        int msgId = 0xFFFFFF;
        byte[] buf = new byte[3];
        Protocol.PutMsgId(buf, 0, msgId);
        int decoded = Protocol.GetMsgId(buf, 0);

        Assert.Equal(msgId, decoded);
    }

    [Fact]
    public void TestGetResponseMsgId()
    {
        Assert.Equal(Protocol.MsgInvokeResponse, Protocol.GetResponseMsgId(Protocol.MsgInvokeRequest));
        Assert.Equal(Protocol.MsgProviderConnectResponse, Protocol.GetResponseMsgId(Protocol.MsgProviderConnectRequest));
        Assert.Equal(Protocol.MsgHeartbeatResponse, Protocol.GetResponseMsgId(Protocol.MsgHeartbeatRequest));
    }

    [Fact]
    public void TestIsRequest()
    {
        Assert.True(Protocol.IsRequest(Protocol.MsgInvokeRequest));
        Assert.True(Protocol.IsRequest(Protocol.MsgProviderConnectRequest));
        Assert.True(Protocol.IsRequest(Protocol.MsgHeartbeatRequest));
        Assert.False(Protocol.IsRequest(Protocol.MsgInvokeResponse));
        Assert.False(Protocol.IsRequest(Protocol.MsgProviderConnectResponse));
        Assert.False(Protocol.IsRequest(Protocol.MsgTaskEvent)); // TaskEvent is neither request nor response
    }

    [Fact]
    public void TestIsResponse()
    {
        Assert.True(Protocol.IsResponse(Protocol.MsgInvokeResponse));
        Assert.True(Protocol.IsResponse(Protocol.MsgProviderConnectResponse));
        Assert.True(Protocol.IsResponse(Protocol.MsgHeartbeatResponse));
        Assert.False(Protocol.IsResponse(Protocol.MsgInvokeRequest));
        Assert.False(Protocol.IsResponse(Protocol.MsgProviderConnectRequest));
        Assert.False(Protocol.IsResponse(Protocol.MsgTaskEvent)); // TaskEvent is neither request nor response
    }

    [Fact]
    public void TestMsgIdString()
    {
        Assert.Equal("InvokeRequest", Protocol.MsgIdString(Protocol.MsgInvokeRequest));
        Assert.Equal("InvokeResponse", Protocol.MsgIdString(Protocol.MsgInvokeResponse));
        Assert.Equal("ProviderConnectRequest", Protocol.MsgIdString(Protocol.MsgProviderConnectRequest));
        Assert.Equal("ProviderConnectResponse", Protocol.MsgIdString(Protocol.MsgProviderConnectResponse));
        Assert.Equal("StartTaskRequest", Protocol.MsgIdString(Protocol.MsgStartTaskRequest));
        Assert.Equal("TaskEvent", Protocol.MsgIdString(Protocol.MsgTaskEvent));

        // Unknown message type
        var unknown = Protocol.MsgIdString(0xFFFFFF);
        Assert.StartsWith("Unknown", unknown);
    }

    [Theory]
    [InlineData(Protocol.MsgRegisterRequest, Protocol.MsgRegisterResponse)]
    [InlineData(Protocol.MsgHeartbeatRequest, Protocol.MsgHeartbeatResponse)]
    [InlineData(Protocol.MsgInvokeRequest, Protocol.MsgInvokeResponse)]
    [InlineData(Protocol.MsgStartTaskRequest, Protocol.MsgStartTaskResponse)]
    [InlineData(Protocol.MsgProviderConnectRequest, Protocol.MsgProviderConnectResponse)]
    public void TestRequestResponsePairs(int requestMsgId, int expectedResponseMsgId)
    {
        Assert.Equal(expectedResponseMsgId, Protocol.GetResponseMsgId(requestMsgId));
        Assert.True(Protocol.IsRequest(requestMsgId));
        Assert.True(Protocol.IsResponse(expectedResponseMsgId));
    }
}
