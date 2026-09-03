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

using System.Text.Json;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using Google.Protobuf;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// F：Provider 侧入站校验的派发接线（HandleInboundRequestAsync 路径）。
/// </summary>
public class InboundValidationDispatchTests
{
    private sealed class CountingHandler : IFunctionHandler
    {
        public int Calls;
        public Task<string> HandleAsync(FunctionContext context, string payload)
        {
            Calls++;
            return Task.FromResult("ok");
        }
    }

    private static CroupierClient NewClient(bool validate, out CountingHandler handler)
    {
        handler = new CountingHandler();
        var config = new ClientConfig
        {
            ValidateInputPayloads = validate,
        };
        var client = new CroupierClient(config);
        client.RegisterFunction(new FunctionDescriptor
        {
            Id = "player.ban",
            Version = "1.0.0",
            InputSchema = "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}},\"required\":[\"id\"]}",
        }, handler);
        return client;
    }

    private static byte[] InvokeBody(string functionId, string payload)
    {
        var request = new InvokeRequest
        {
            FunctionId = functionId,
            Payload = ByteString.CopyFromUtf8(payload),
        };
        return Google.Protobuf.MessageExtensions.ToByteArray(request);
    }

    private static async Task<byte[]> DispatchAsync(CroupierClient client, byte[] body)
    {
        var method = typeof(CroupierClient).GetMethod("HandleInboundRequestAsync",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
        Assert.NotNull(method);
        var task = (Task<byte[]>?)method!.Invoke(client, new object[] { Protocol.MsgInvokeRequest, 1, body });
        Assert.NotNull(task);
        return await task;
    }

    private static async Task<JsonElement?> PayloadAsJsonAsync(byte[] response)
    {
        var invokeResponse = InvokeResponse.Parser.ParseFrom(response);
        var text = invokeResponse.Payload.ToStringUtf8();
        if (text.Length == 0 || text[0] != '{') return null;
        return JsonDocument.Parse(text).RootElement;
    }

    [Fact]
    public async Task Config_Defaults_To_Off()
    {
        Assert.False(new ClientConfig().ValidateInputPayloads);
    }

    [Fact]
    public async Task InvalidPayload_ReturnsErrorAndSkipsHandler()
    {
        var client = NewClient(validate: true, out var handler);
        var response = await DispatchAsync(client, InvokeBody("player.ban", "{}"));
        var payload = await PayloadAsJsonAsync(response);
        Assert.NotNull(payload);
        Assert.True((payload!.Value.TryGetProperty("error", out var error)
                     && error.GetString()!.Contains("payload validation failed")),
            "expected validation error, got: " + payload);
        Assert.Equal(0, handler.Calls);
    }

    [Fact]
    public async Task ValidPayload_InvokesHandler()
    {
        var client = NewClient(validate: true, out var handler);
        var response = await DispatchAsync(client, InvokeBody("player.ban", "{\"id\":\"p1\"}"));
        var payload = await PayloadAsJsonAsync(response);
        Assert.Null(payload);
        Assert.Equal(1, handler.Calls);
    }

    [Fact]
    public async Task DisabledFlag_KeepsLegacyBehavior()
    {
        var client = NewClient(validate: false, out var handler);
        var response = await DispatchAsync(client, InvokeBody("player.ban", "{}"));
        var payload = await PayloadAsJsonAsync(response);
        Assert.Null(payload);
        Assert.Equal(1, handler.Calls);
    }
}
