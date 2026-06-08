package io.github.cuihairu.croupier.sdk.testing;

import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;

import java.util.Arrays;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * In-memory transport double for unit tests.
 */
public final class FakeTransportClient implements TransportClient {
    @FunctionalInterface
    public interface Handler {
        byte[] handle(int msgType, byte[] data) throws Exception;
    }

    public record Call(int msgType, byte[] data) {
    }

    private final Handler handler;
    private final CopyOnWriteArrayList<Call> calls = new CopyOnWriteArrayList<>();
    private volatile boolean connected;

    public FakeTransportClient(Handler handler) {
        this.handler = handler;
    }

    @Override
    public void connect() {
        connected = true;
    }

    @Override
    public byte[] request(int msgType, byte[] data) throws InvokerException {
        if (!connected) {
            throw new RuntimeException("Not connected");
        }
        byte[] request = data == null ? new byte[0] : Arrays.copyOf(data, data.length);
        calls.add(new Call(msgType, request));
        try {
            return handler != null ? handler.handle(msgType, request) : new byte[0];
        } catch (InvokerException e) {
            throw e;
        } catch (RuntimeException e) {
            throw e;
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public boolean isConnected() {
        return connected;
    }

    @Override
    public void close() {
        connected = false;
    }

    public List<Call> getCalls() {
        return calls;
    }
}
