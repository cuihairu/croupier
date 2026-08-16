using Xunit;

namespace Croupier.Sdk.Tests.Integration;

[AttributeUsage(AttributeTargets.Method)]
public sealed class IntegrationFactAttribute : FactAttribute
{
    private const string DisabledReason = "Integration test disabled; set CROUPIER_RUN_INTEGRATION_TESTS=1 to run.";

    public IntegrationFactAttribute()
    {
        if (!string.Equals(Environment.GetEnvironmentVariable("CROUPIER_RUN_INTEGRATION_TESTS"), "1", StringComparison.Ordinal))
        {
            Skip = DisabledReason;
        }
    }
}
