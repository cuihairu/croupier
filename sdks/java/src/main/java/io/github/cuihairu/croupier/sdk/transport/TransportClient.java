package io.github.cuihairu.croupier.sdk.transport;

import io.github.cuihairu.croupier.sdk.invoker.InvokerException;

/**
 * Minimal transport abstraction for SDK request/response flows.
 */
public interface TransportClient extends AutoCloseable {
    /**
     * Connects the transport to its remote peer.
     */
    void connect();

    /**
     * Sends a request and returns the protobuf response body.
     *
     * @param msgType protocol message type
     * @param data protobuf request body
     * @return protobuf response body
     * @throws InvokerException if the request fails at the transport level
     */
    byte[] request(int msgType, byte[] data) throws InvokerException;

    /**
     * Indicates whether the transport is connected.
     *
     * @return true when connected
     */
    boolean isConnected();

    @Override
    void close();
}
