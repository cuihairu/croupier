using Croupier.Sdk.Logging;

namespace Croupier.Sdk.Transport;

internal interface IClientTransport : IDisposable
{
    bool IsConnected { get; }
    void Connect();
    byte[] Call(int msgType, byte[]? data);
    Task<byte[]> CallAsync(int msgType, byte[]? data, CancellationToken cancellationToken = default);
}
