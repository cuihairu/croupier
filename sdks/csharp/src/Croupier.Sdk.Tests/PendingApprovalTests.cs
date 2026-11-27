// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// AUD-069: pending-approval responses carry an approval credential, not a
/// business result or a task ID, and must not be classified as write failures.
/// </summary>
public sealed class PendingApprovalTests
{
    private sealed class JsonHandler(string body) : HttpMessageHandler
    {
        public int Calls { get; private set; }

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            Calls++;
            return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(body, Encoding.UTF8, "application/json"),
            });
        }
    }

    private static CroupierInvoker Invoker(HttpMessageHandler handler) =>
        new(new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" }, new HttpClient(handler), ownsHttpClient: true);

    private static RetryConfig FastRetry => new()
    {
        Enabled = true, MaxAttempts = 3, InitialDelayMs = 1, JitterFactor = 0, RetryWrites = true,
    };

    private const string InvokeApproval =
        """
        {"taskId":"","approvalId":"apr-1","approvalRequired":true,"approvalWorkflow":"two_person","traceId":"tr-1"}
        """;

    private const string TaskApproval =
        """
        {"taskId":"","status":"approval_required","approvalId":"apr-2","approvalWorkflow":"single_admin"}
        """;

    [Fact]
    public async Task InvokeReturnsPendingApprovalResult()
    {
        var handler = new JsonHandler(InvokeApproval);
        var result = await Invoker(handler).InvokeAsync("player.ban", "{}", new InvokeOptions { Retry = FastRetry });

        result.Success.Should().BeFalse();
        result.ApprovalRequired.Should().BeTrue();
        result.ApprovalId.Should().Be("apr-1");
        result.ApprovalWorkflow.Should().Be("two_person");
        result.ErrorCode.Should().Be("APPROVAL_REQUIRED");
        result.Error.Should().Contain("apr-1");
        handler.Calls.Should().Be(1);
    }

    [Fact]
    public async Task StartTaskThrowsApprovalRequiredException()
    {
        var handler = new JsonHandler(TaskApproval);
        var action = async () => await Invoker(handler).StartTaskAsync("player.ban", "{}", new InvokeOptions { Retry = FastRetry });

        var error = await action.Should().ThrowAsync<ApprovalRequiredException>();
        error.Which.Message.Should().Contain("apr-2");
        handler.Calls.Should().Be(1);
    }

    [Fact]
    public async Task MissingApprovalIdStaysMissingOutcome()
    {
        var handler = new JsonHandler("""{"taskId":"","status":"approval_required"}""");
        var action = async () => await Invoker(handler).StartTaskAsync("player.ban", "{}");

        await action.Should().ThrowAsync<InvalidOperationException>()
            .WithMessage("*taskId*");
    }
}
