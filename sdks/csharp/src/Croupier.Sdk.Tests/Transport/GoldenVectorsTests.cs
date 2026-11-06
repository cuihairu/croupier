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

using System;
using System.IO;
using System.Text.Json;
using Croupier.Sdk.Transport;
using Xunit;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// CR-FIX-05 cross-SDK behavioural contract golden vectors (.NET surface).
/// Reads proto/contracts/golden_vectors.json and asserts Protocol.NewMessage
/// produces the exact frame bytes and ParseMessage round-trips them.
/// </summary>
public class GoldenVectorsTests
{
    private static string GoldenPath()
    {
        foreach (var start in new[] { Directory.GetCurrentDirectory(), AppContext.BaseDirectory })
        {
            for (var dir = new DirectoryInfo(start); dir != null; dir = dir.Parent)
            {
                var candidate = Path.Combine(dir.FullName, "proto", "contracts", "golden_vectors.json");
                if (File.Exists(candidate))
                {
                    return candidate;
                }
            }
        }

        throw new FileNotFoundException("proto/contracts/golden_vectors.json not found");
    }

    [Fact]
    public void WireFramesEncodeAndDecode()
    {
        using var doc = JsonDocument.Parse(File.ReadAllText(GoldenPath()));
        var vectors = doc.RootElement.GetProperty("vectors");

        foreach (var vector in vectors.EnumerateObject())
        {
            var value = vector.Value;
            var msgId = Convert.ToInt32(value.GetProperty("msgId").GetString(), 16);
            var reqId = Convert.ToInt32(value.GetProperty("reqId").GetString(), 16);
            var body = Convert.FromHexString(value.GetProperty("body").GetString() ?? string.Empty);
            var want = Convert.FromHexString(value.GetProperty("frame").GetString() ?? string.Empty);

            var got = Protocol.NewMessage(msgId, reqId, body);
            Assert.Equal(want, got);

            var parsed = Protocol.ParseMessage(want);
            Assert.Equal(Protocol.Version1, parsed.Version);
            Assert.Equal(msgId, parsed.MsgId);
            Assert.Equal(reqId, parsed.ReqId);
            Assert.Equal(body, parsed.Body);
        }
    }

    [Fact]
    public void UsesKnownMsgIdsFromContractSurface()
    {
        using var doc = JsonDocument.Parse(File.ReadAllText(GoldenPath()));
        var vectors = doc.RootElement.GetProperty("vectors");

        Assert.Equal(Protocol.MsgRegisterRequest, ParseHex(vectors, "register_request"));
        Assert.Equal(Protocol.MsgHeartbeatRequest, ParseHex(vectors, "heartbeat_request"));
        Assert.Equal(Protocol.MsgTaskEvent, ParseHex(vectors, "task_event"));
    }

    private static int ParseHex(JsonElement vectors, string name)
    {
        return Convert.ToInt32(vectors.GetProperty(name).GetProperty("msgId").GetString(), 16);
    }
}
