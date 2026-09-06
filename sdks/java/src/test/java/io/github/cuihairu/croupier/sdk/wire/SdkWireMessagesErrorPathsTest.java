package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * SdkWireMessages 解码器边界补测：
 * 未知字段 skipField、截断输入抛 IllegalArgumentException、
 * 嵌套消息（map entry / ProviderMeta / FunctionDescriptor）的错误传播，
 * 以及各消息构造器的 null 归一化。
 */
@DisplayName("SdkWireMessages decode error paths and null normalization")
class SdkWireMessagesErrorPathsTest {

    /** decodeProviderFunctionDescriptor 是私有方法，经 ProviderConnectRequest 或反射驱动。 */
    private static SdkWireMessages.ProviderFunctionDescriptor decodeFunctionDescriptor(byte[] data) throws Exception {
        java.lang.reflect.Method method = SdkWireMessages.class
            .getDeclaredMethod("decodeProviderFunctionDescriptor", byte[].class);
        method.setAccessible(true);
        try {
            return (SdkWireMessages.ProviderFunctionDescriptor) method.invoke(null, (Object) data);
        } catch (java.lang.reflect.InvocationTargetException e) {
            Throwable cause = e.getCause();
            if (cause instanceof Exception ex) {
                throw ex;
            }
            throw e;
        }
    }

    /** 未知字段（field 100, wire type 2）应被 skipField 而不是报错。 */
    @Test
    @DisplayName("unknown fields are skipped by every decoder")
    void unknownFieldsAreSkipped() throws Exception {
        byte[] unknown = { (byte) 0xA2, 0x06, 0x03, 'a', 'b', 'c' };
        assertEquals("", SdkWireMessages.decodeInvokeRequest(unknown).functionId);
        assertTrue(SdkWireMessages.decodeInvokeRequest(unknown).metadata.isEmpty());
        assertEquals(0, SdkWireMessages.decodeInvokeResponse(unknown).payload.length);
        assertEquals("", SdkWireMessages.decodeStartTaskResponse(unknown).taskId);
        assertEquals("", SdkWireMessages.decodeTaskStreamRequest(unknown).taskId);
        assertEquals("", SdkWireMessages.decodeTaskEvent(unknown).type);
        assertEquals("", SdkWireMessages.decodeCancelTaskRequest(unknown).taskId);
        assertEquals("", SdkWireMessages.decodeProviderConnectRequest(unknown).serviceId);
        assertEquals("", SdkWireMessages.decodeProviderConnectResponse(unknown).sessionId);
        assertEquals("", SdkWireMessages.decodeHeartbeatRequest(unknown).serviceId);
        assertEquals("", SdkWireMessages.decodeFilePushRequest(unknown).transferId);
        assertEquals("", SdkWireMessages.decodeFilePushResponse(unknown).transferId);
        assertEquals("", SdkWireMessages.decodeProviderDrainRequest(unknown).sessionId);
        SdkWireMessages.ProviderFunctionDescriptor descriptor = decodeFunctionDescriptor(unknown);
        assertEquals("", descriptor.id);
        // 未知字段的 map entry：key 为空不入 metadata
        byte[] invokeWithOddEntry = {0x22, 0x06, (byte) 0xA2, 0x06, 0x03, 'a', 'b', 'c'};
        assertTrue(SdkWireMessages.decodeInvokeRequest(invokeWithOddEntry).metadata.isEmpty());
        // RegisterCapabilitiesRequest 嵌套 ProviderMeta 的未知字段
        byte[] nestedUnknown = {0x0A, 0x06, (byte) 0xA2, 0x06, 0x03, 'a', 'b', 'c'};
        assertEquals("", SdkWireMessages.decodeRegisterCapabilitiesRequest(nestedUnknown).provider.id);
        // ProviderConnectRequest 内嵌 FunctionDescriptor 的未知字段
        byte[] connectUnknownFn = {0x1A, 0x06, (byte) 0xA2, 0x06, 0x03, 'a', 'b', 'c'};
        assertEquals(1, SdkWireMessages.decodeProviderConnectRequest(connectUnknownFn).functions.size());
    }

    /** 声明长度超过实际数据的 string/bytes 字段：EOF 解码失败转 IllegalArgumentException。 */
    @Test
    @DisplayName("truncated wire bytes fail with IllegalArgumentException")
    void truncatedInputFails() throws Exception {
        byte[] truncatedString = {0x0A, 0x05, 'a'};
        byte[] truncatedBytes = {0x0A, 0x05, 'a'};

        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeResponse(truncatedBytes));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeStartTaskResponse(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeTaskStreamRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeTaskEvent(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeCancelTaskRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectResponse(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeHeartbeatRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> decodeFunctionDescriptor(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeRegisterCapabilitiesRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeFilePushRequest(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeFilePushResponse(truncatedString));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderDrainRequest(truncatedString));

        // InvokeRequest metadata entry（field 4）内截断 string
        byte[] badEntry = {0x22, 0x03, 0x0A, 0x05, 'a'};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeRequest(badEntry));
        // ProviderConnectRequest 内嵌 FunctionDescriptor 截断
        byte[] badFunction = {0x1A, 0x03, 0x0A, 0x05, 'a'};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectRequest(badFunction));
        // RegisterCapabilitiesRequest 嵌套 ProviderMeta 截断 + manifest 截断
        byte[] badMeta = {0x0A, 0x03, 0x0A, 0x05, 'a'};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeRegisterCapabilitiesRequest(badMeta));
        byte[] badManifest = {0x12, 0x05, 'x'};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeRegisterCapabilitiesRequest(badManifest));
    }

    @Test
    @DisplayName("constructors normalize null parts to empty values")
    void constructorsNormalizeNulls() {
        SdkWireMessages.ProviderMeta meta = new SdkWireMessages.ProviderMeta(null, null, null, null);
        assertEquals("", meta.id);
        assertEquals("", meta.version);
        assertEquals("", meta.lang);
        assertEquals("", meta.sdk);

        SdkWireMessages.RegisterCapabilitiesRequest capabilities =
            new SdkWireMessages.RegisterCapabilitiesRequest(null, null);
        assertNull(capabilities.provider);
        assertEquals(0, capabilities.manifestJsonGz.length);

        SdkWireMessages.FilePushRequest request =
            new SdkWireMessages.FilePushRequest(null, null, null, null);
        assertEquals("", request.transferId);
        assertEquals("", request.fileName);
        assertEquals("", request.contentSha256);
        assertEquals(0, request.data.length);

        SdkWireMessages.FilePushResponse response =
            new SdkWireMessages.FilePushResponse(null, false, null, null);
        assertEquals("", response.transferId);
        assertFalse(response.ok);
        assertEquals("", response.storedPath);
        assertEquals("", response.error);

        SdkWireMessages.ProviderDrainRequest drain =
            new SdkWireMessages.ProviderDrainRequest(null, null, 0);
        assertEquals("", drain.sessionId);
        assertEquals("", drain.reason);
        assertEquals(0, drain.retryAfterMs);
    }
}
