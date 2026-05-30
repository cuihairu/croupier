package io.github.cuihairu.croupier.sdk.transport;

/**
 * Minimal abstraction for a local request server.
 */
public interface RequestServer extends AutoCloseable {
    /**
     * Registers the request handler invoked for each incoming request.
     *
     * @param handler request handler
     */
    void setHandler(RequestHandler handler);

    /**
     * Starts listening for incoming requests.
     */
    void listen();

    /**
     * Indicates whether the server is listening.
     *
     * @return true when listening
     */
    boolean isListening();

    @Override
    void close();
}
