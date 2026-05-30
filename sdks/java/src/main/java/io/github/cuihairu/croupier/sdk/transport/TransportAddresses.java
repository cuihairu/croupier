package io.github.cuihairu.croupier.sdk.transport;

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
}
