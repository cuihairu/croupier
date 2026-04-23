package io.github.cuihairu.croupier.sdk.transport;

import java.io.IOException;
import java.net.ServerSocket;

/**
 * Address helpers shared by transport-backed client implementations.
 */
public final class TransportAddresses {
    private TransportAddresses() {
    }

    /**
     * Normalizes an address to TCP notation.
     *
     * @param address raw address such as {@code 127.0.0.1:19090}
     * @return normalized address such as {@code tcp://127.0.0.1:19090}
     */
    public static String normalizeAddress(String address) {
        String value = address == null || address.trim().isEmpty()
            ? "127.0.0.1:19090"
            : address.trim();
        return value.contains("://") ? value : "tcp://" + value;
    }

    /**
     * Resolves a listen address to a concrete TCP endpoint.
     *
     * @param address requested listen address
     * @return resolved address
     */
    public static String resolveLocalListenAddress(String address) {
        String normalized = normalizeAddress(
            address == null || address.trim().isEmpty() ? "127.0.0.1:0" : address.trim()
        );

        if (normalized.regionMatches(true, 0, "tcp://0.0.0.0:", 0, "tcp://0.0.0.0:".length())) {
            normalized = "tcp://127.0.0.1:" + normalized.substring("tcp://0.0.0.0:".length());
        }

        if (!normalized.startsWith("tcp://") || !normalized.endsWith(":0")) {
            return normalized;
        }

        try (ServerSocket socket = new ServerSocket(0)) {
            return normalized.substring(0, normalized.length() - 1) + socket.getLocalPort();
        } catch (IOException e) {
            throw new IllegalStateException("Failed to allocate local listen port", e);
        }
    }
}
