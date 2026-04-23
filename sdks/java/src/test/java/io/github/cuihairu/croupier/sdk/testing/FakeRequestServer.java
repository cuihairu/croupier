package io.github.cuihairu.croupier.sdk.testing;

import io.github.cuihairu.croupier.sdk.transport.RequestHandler;
import io.github.cuihairu.croupier.sdk.transport.RequestServer;

/**
 * In-memory request server double for unit tests.
 */
public final class FakeRequestServer implements RequestServer {
    private RequestHandler handler;
    private boolean listening;

    @Override
    public void setHandler(RequestHandler handler) {
        this.handler = handler;
    }

    @Override
    public void listen() {
        listening = true;
    }

    @Override
    public boolean isListening() {
        return listening;
    }

    @Override
    public void close() {
        listening = false;
    }

    public byte[] dispatch(int msgType, int requestId, byte[] body) throws Exception {
        if (handler == null) {
            throw new IllegalStateException("Handler not set");
        }
        return handler.handle(msgType, requestId, body);
    }
}
