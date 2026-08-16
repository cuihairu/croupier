package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for TransportAddresses utility class.
 */
class TransportAddressesTest {

    @Test
    void normalizeAddress_withNull_returnsDefault() {
        assertEquals("tcp://127.0.0.1:19091", TransportAddresses.normalizeAddress(null));
    }

    @Test
    void normalizeAddress_withEmpty_returnsDefault() {
        assertEquals("tcp://127.0.0.1:19091", TransportAddresses.normalizeAddress(""));
    }

    @Test
    void normalizeAddress_withWhitespace_returnsDefault() {
        assertEquals("tcp://127.0.0.1:19091", TransportAddresses.normalizeAddress("   "));
    }

    @Test
    void normalizeAddress_withSimpleAddress_addsTcpScheme() {
        assertEquals("tcp://192.168.1.1:8080", TransportAddresses.normalizeAddress("192.168.1.1:8080"));
    }

    @Test
    void normalizeAddress_withSimpleAddress_trimsWhitespace() {
        assertEquals("tcp://127.0.0.1:19090", TransportAddresses.normalizeAddress("  127.0.0.1:19090  "));
    }

    @Test
    void normalizeAddress_withTcpScheme_leavesUnchanged() {
        assertEquals("tcp://127.0.0.1:19090", TransportAddresses.normalizeAddress("tcp://127.0.0.1:19090"));
    }

    @Test
    void normalizeAddress_withTlsTcpScheme_leavesUnchanged() {
        assertEquals("tls+tcp://127.0.0.1:19090", TransportAddresses.normalizeAddress("tls+tcp://127.0.0.1:19090"));
    }

    @Test
    void normalizeAddress_withIpcScheme_leavesUnchanged() {
        assertEquals("ipc:///tmp/socket", TransportAddresses.normalizeAddress("ipc:///tmp/socket"));
    }

    @Test
    void normalizeAddress_withInprocScheme_leavesUnchanged() {
        assertEquals("inproc://name", TransportAddresses.normalizeAddress("inproc://name"));
    }

    @Test
    void normalizeAddress_withWsScheme_leavesUnchanged() {
        assertEquals("ws://localhost:8080", TransportAddresses.normalizeAddress("ws://localhost:8080"));
    }

    @Test
    void normalizeAddress_withWssScheme_leavesUnchanged() {
        assertEquals("wss://localhost:8080", TransportAddresses.normalizeAddress("wss://localhost:8080"));
    }

    @Test
    void normalizeAddress_withLocalhost() {
        assertEquals("tcp://localhost:19090", TransportAddresses.normalizeAddress("localhost:19090"));
    }

    @Test
    void normalizeAddress_withIPv6() {
        assertEquals("tcp://[::1]:19090", TransportAddresses.normalizeAddress("[::1]:19090"));
    }

    @Test
    void normalizeAddress_withPortOnly() {
        assertEquals("tcp://:19090", TransportAddresses.normalizeAddress(":19090"));
    }
}
