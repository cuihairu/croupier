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

namespace Croupier.Sdk.Models;

/// <summary>
/// 函数描述符
/// </summary>
public class FunctionDescriptor
{
    /// <summary>
    /// 函数唯一标识符 (格式: category.entity.operation)
    /// </summary>
    public string Id { get; set; } = string.Empty;

    /// <summary>
    /// 函数版本
    /// </summary>
    public string Version { get; set; } = "1.0.0";

    /// <summary>
    /// 函数分类 (player, wallet, moderation, etc.)
    /// </summary>
    public string Category { get; set; } = string.Empty;

    /// <summary>
    /// 风险级别 (low, medium, high, critical)
    /// </summary>
    public string Risk { get; set; } = "medium";

    /// <summary>
    /// 操作实体类型
    /// </summary>
    public string? Entity { get; set; }

    /// <summary>
    /// 操作类型
    /// </summary>
    public string? Operation { get; set; }

    /// <summary>
    /// 是否启用
    /// </summary>
    public bool Enabled { get; set; } = true;

    /// <summary>
    /// 显示名称
    /// </summary>
    public string? DisplayName { get; set; }

    /// <summary>
    /// 函数描述
    /// </summary>
    public string? Description { get; set; }

    /// <summary>
    /// 输入参数 JSON Schema
    /// </summary>
    public string? InputSchema { get; set; }

    /// <summary>
    /// 输出结果 JSON Schema
    /// </summary>
    public string? OutputSchema { get; set; }

    /// <summary>
    /// 标签
    /// </summary>
    public Dictionary<string, string>? Tags { get; set; }

    /// <summary>
    /// 验证描述符是否有效
    /// </summary>
    public bool IsValid()
    {
        if (string.IsNullOrWhiteSpace(Id))
            return false;

        if (string.IsNullOrWhiteSpace(Version))
            return false;

        if (string.IsNullOrWhiteSpace(Category))
            return false;

        if (string.IsNullOrWhiteSpace(Risk))
            return false;

        return true;
    }

    /// <summary>
    /// 获取完整函数标识符
    /// </summary>
    public string GetFullName() => $"{Category}.{Id}";
}
