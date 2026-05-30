package io.github.cuihairu.croupier.sdk.transport;

/**
 * Handles a protocol request received by a local request server.
 */
@FunctionalInterface
public interface RequestHandler {
    /**
     * Handles a single protocol request.
     *
     * @param msgType protocol message type
     * @param requestId request identifier from the wire header
     * @param body protobuf request body
     * @return protobuf response body
     * @throws Exception when request handling fails
     */
    byte[] handle(int msgType, int requestId, byte[] body) throws Exception;
}
