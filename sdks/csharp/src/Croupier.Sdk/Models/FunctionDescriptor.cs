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
    /// 函数唯一标识符
    /// </summary>
    public string Id { get; set; } = string.Empty;

    /// <summary>
    /// 函数版本
    /// </summary>
    public string Version { get; set; } = "1.0.0";

    /// <summary>
    /// 业务资源或能力域标识
    /// </summary>
    public string? Resource { get; set; }

    /// <summary>
    /// 业务动作标识
    /// </summary>
    public string? Operation { get; set; }

    /// <summary>
    /// 风险级别 (safe, warning, high, danger)
    /// </summary>
    public string? Risk { get; set; } = "warning";

    /// <summary>
    /// 权限标识
    /// </summary>
    public string? Permission { get; set; }

    /// <summary>
    /// 是否启用
    /// </summary>
    public bool Enabled { get; set; } = true;

    /// <summary>
    /// 简短摘要，用于函数目录和搜索
    /// </summary>
    public string? Summary { get; set; }

    /// <summary>
    /// 函数描述
    /// </summary>
    public string? Description { get; set; }

    /// <summary>
    /// 稳定操作 ID，默认可使用函数 ID
    /// </summary>
    public string? OperationId { get; set; }

    /// <summary>
    /// 是否已废弃
    /// </summary>
    public bool Deprecated { get; set; }

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

        return true;
    }
}
