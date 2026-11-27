// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

namespace Croupier.Sdk;

/// <summary>
/// AUD-069: the Server accepted a write for approval instead of executing it.
/// No task was dispatched and no business side effect occurred. The caller must
/// preserve the approval ID returned inside <see cref="Models.InvokeResult"/>
/// where a result surface exists; this exception is thrown by task start, which
/// returns only a task ID.
/// </summary>
public sealed class ApprovalRequiredException : InvalidOperationException
{
    public ApprovalRequiredException(string message) : base(message)
    {
    }
}
