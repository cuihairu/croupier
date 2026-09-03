package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;
import java.lang.reflect.Method;
import org.junit.jupiter.api.io.TempDir;

import java.nio.file.Files;
import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * F：文件下发接收（hotpatch P1 传输层）——Java 实现测试。
 */
public class FilePushTest {

    private CroupierClientImpl newClient(boolean enable, String staging) {
        ClientConfig config = new ClientConfig();
        config.setEnableFileTransfer(enable);
        config.setMaxFileSize(1024);
        config.setFileStagingDir(staging);
        return new CroupierClientImpl(config);
    }

    private byte[] push(CroupierClientImpl client, SdkWireMessages.FilePushRequest request)
            throws Exception {
        Method m = CroupierClientImpl.class.getDeclaredMethod("handleFilePushRequest", byte[].class);
        m.setAccessible(true);
        return (byte[]) m.invoke(client, (Object) SdkWireMessages.encodeFilePushRequest(request));
    }

    private SdkWireMessages.FilePushResponse decode(byte[] raw) {
        return SdkWireMessages.decodeFilePushResponse(raw);
    }

    private String sha256(byte[] data) throws Exception {
        byte[] digest = java.security.MessageDigest.getInstance("SHA-256").digest(data);
        StringBuilder sb = new StringBuilder(digest.length * 2);
        for (byte b : digest) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    @Test
    public void validPayloadStagedAndOk(@TempDir Path tempDir) throws Exception {
        Path staging = tempDir.resolve("staging");
        CroupierClientImpl client = newClient(true, staging.toString());
        byte[] data = "print('hotfix')".getBytes();
        byte[] raw = push(client, new SdkWireMessages.FilePushRequest(
                "t-1", "hotfix.lua", sha256(data), data));
        SdkWireMessages.FilePushResponse response = decode(raw);
        assertTrue(response.ok);
        assertTrue(response.storedPath.endsWith("hotfix.lua"));
        assertEquals("print('hotfix')", Files.readString(Path.of(response.storedPath)));
    }

    @Test
    public void disabledFlagRejects(@TempDir Path tempDir) throws Exception {
        CroupierClientImpl client = newClient(false, tempDir.toString());
        byte[] data = "x".getBytes();
        SdkWireMessages.FilePushResponse response = decode(push(client,
                new SdkWireMessages.FilePushRequest("t-2", "a.lua", sha256(data), data)));
        assertFalse(response.ok);
        assertTrue(response.error.contains("file transfer is disabled"));
    }

    @Test
    public void checksumMismatchRejects(@TempDir Path tempDir) throws Exception {
        CroupierClientImpl client = newClient(true, tempDir.toString());
        byte[] raw = push(client, new SdkWireMessages.FilePushRequest(
                "t-3", "a.lua", "de".repeat(32), "x".getBytes()));
        SdkWireMessages.FilePushResponse response = decode(raw);
        assertFalse(response.ok);
        assertTrue(response.error.contains("checksum mismatch"));
    }

    @Test
    public void pathTraversalRejects(@TempDir Path tempDir) throws Exception {
        CroupierClientImpl client = newClient(true, tempDir.toString());
        byte[] data = "x".getBytes();
        for (String evil : new String[]{"../evil.lua", "sub/dir/evil.lua", "/etc/evil.lua"}) {
            byte[] raw = push(client, new SdkWireMessages.FilePushRequest(
                    "t-4", evil, sha256(data), data));
            SdkWireMessages.FilePushResponse response = decode(raw);
            assertFalse(response.ok);
            assertTrue(response.error.contains("bare basename"), "evil=" + evil);
        }
        assertFalse(Files.exists(tempDir.resolve("evil.lua")));
    }

    @Test
    public void oversizeRejects(@TempDir Path tempDir) throws Exception {
        CroupierClientImpl client = newClient(true, tempDir.toString());
        byte[] data = new byte[2048];
        byte[] raw = push(client, new SdkWireMessages.FilePushRequest(
                "t-5", "big.lua", sha256(data), data));
        SdkWireMessages.FilePushResponse response = decode(raw);
        assertFalse(response.ok);
        assertTrue(response.error.contains("exceeds max"));
    }

    @Test
    public void emptyTransferIdRejects(@TempDir Path tempDir) throws Exception {
        CroupierClientImpl client = newClient(true, tempDir.toString());
        byte[] data = "x".getBytes();
        byte[] raw = push(client, new SdkWireMessages.FilePushRequest(
                "", "a.lua", sha256(data), data));
        SdkWireMessages.FilePushResponse response = decode(raw);
        assertFalse(response.ok);
        assertTrue(response.error.contains("transferId is required"));
    }
}
