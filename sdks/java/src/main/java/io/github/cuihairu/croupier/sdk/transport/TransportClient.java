package io.github.cuihairu.croupier.sdk.transport;

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
     */
    byte[] request(int msgType, byte[] data);

    /**
     * Indicates whether the transport is connected.
     *
     * @return true when connected
     */
    boolean isConnected();

    @Override
    void close();
}
