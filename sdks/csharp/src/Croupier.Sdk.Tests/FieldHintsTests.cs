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
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// F：x-ui 呈现 hints 便捷层测试。
/// </summary>
public class FieldHintsTests
{
    [Fact]
    public void EmptySchema_CreatesObjectSkeleton()
    {
        var descriptor = Validation.FieldHints.SetFieldWidget(
            new FunctionDescriptor { Id = "player.ban", Version = "1.0.0" }, "id", "Select");
        Assert.NotNull(descriptor.InputSchema);
        var schema = JsonDocument.Parse(descriptor.InputSchema!);
        Assert.Equal("object", schema.RootElement.GetProperty("type").GetString());
        Assert.Equal("Select", schema.RootElement
            .GetProperty("properties").GetProperty("id")
            .GetProperty("x-widget").GetString());
    }

    [Fact]
    public void PreservesExistingAttributes_AndOverrides()
    {
        var descriptor = new FunctionDescriptor
        {
            Id = "player.ban",
            InputSchema = "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\",\"title\":\"玩家 ID\",\"x-widget\":\"Input\"}}}",
        };
        var updated = Validation.FieldHints.SetFieldWidget(descriptor, "id", "TreeSelect");
        Assert.Contains("Input", descriptor.InputSchema); // 不可变：原描述符不变
        var prop = JsonDocument.Parse(updated.InputSchema!)
            .RootElement.GetProperty("properties").GetProperty("id");
        Assert.Equal("TreeSelect", prop.GetProperty("x-widget").GetString());
        Assert.Equal("玩家 ID", prop.GetProperty("title").GetString());
    }

    [Fact]
    public void OptionsSource_Object()
    {
        var descriptor = Validation.FieldHints.SetFieldHint(
            new FunctionDescriptor { Id = "player.ban" }, "id", "x-options-source",
            JsonSerializer.SerializeToElement(new Dictionary<string, string>
            {
                ["functionId"] = "player.list",
                ["labelPath"] = "/items/*/name",
                ["valuePath"] = "/items/*/id",
            }));
        Assert.Contains("player.list", descriptor.InputSchema);
        Assert.Contains("/items/*/name", descriptor.InputSchema);
    }

    [Fact]
    public void XUnderscore_NormalizedToXDash()
    {
        var descriptor = Validation.FieldHints.SetFieldHint(
            new FunctionDescriptor { Id = "f" }, "a", "x_widget",
            JsonSerializer.SerializeToElement("Input"));
        Assert.Contains("x-widget", descriptor.InputSchema);
        Assert.DoesNotContain("x_widget", descriptor.InputSchema);
    }

    [Fact]
    public void InvalidHint_Rejected()
    {
        Assert.Throws<ArgumentException>(() => Validation.FieldHints.SetFieldHint(
            new FunctionDescriptor { Id = "f" }, "a", "widget",
            JsonSerializer.SerializeToElement("Input")));
    }

    [Fact]
    public void EmptyField_Rejected()
    {
        Assert.Throws<ArgumentException>(() => Validation.FieldHints.SetFieldHint(
            new FunctionDescriptor { Id = "f" }, " ", "x-widget",
            JsonSerializer.SerializeToElement("Input")));
    }

    [Fact]
    public void EmptyWidget_Rejected()
    {
        Assert.Throws<ArgumentException>(() => Validation.FieldHints.SetFieldWidget(
            new FunctionDescriptor { Id = "f" }, "a", " "));
    }
}
