package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;

import static org.junit.jupiter.api.Assertions.*;

/**
 * SdkWireMessages 边缘补测：
 * RegisterCapabilitiesRequest 顶层未知 varint 字段 skipField、
 * encode 阶段 IOException 转 IllegalStateException、
 * 以及首字节 0x00（tag=0）的行为基线。
 *
 * 注意：protobuf-java 4.29.5 的 CodedInputStream.readTag() 对 field number 0
 * 直接抛 InvalidProtocolBufferException（"invalid tag (zero)"），不会返回 0；
 * 因此各解码器中 `if (tag == 0) break;` 防御分支经任何输入均不可达（死代码），
 * 此处固化真实行为：截断/零 tag 输入统一转 IllegalArgumentException。
 */
@DisplayName("SdkWireMessages zero-tag behavior and encode failure edge paths")
class SdkWireMessagesZeroTagEdgeTest {

    /** 首字节 0x00 → readTag 抛 invalid tag → 解码器转 IllegalArgumentException。 */
    @Test
    @DisplayName("zero leading tag fails every decoder with IllegalArgumentException")
    void zeroLeadingTagFailsEveryDecoder() {
        byte[] zeroTag = {0x00};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeResponse(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeStartTaskResponse(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeTaskStreamRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeTaskEvent(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeCancelTaskRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectResponse(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeHeartbeatRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeRegisterCapabilitiesRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeFilePushRequest(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeFilePushResponse(zeroTag));
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderDrainRequest(zeroTag));
    }

    @Test
    @DisplayName("嵌套消息（map entry / FunctionDescriptor / ProviderMeta）内 tag=0 同样失败")
    void zeroTagInsideNestedMessagesFails() {
        // InvokeRequest metadata entry（field 4）内容首字节 0x00
        byte[] entryZero = {0x22, 0x01, 0x00};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeRequest(entryZero));

        // ProviderConnectRequest 内嵌 FunctionDescriptor（field 3）内容首字节 0x00
        byte[] functionZero = {0x1A, 0x01, 0x00};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectRequest(functionZero));

        // RegisterCapabilitiesRequest 嵌套 ProviderMeta（field 1）内容首字节 0x00
        byte[] metaZero = {0x0A, 0x01, 0x00};
        assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeRegisterCapabilitiesRequest(metaZero));
    }

    @Test
    @DisplayName("RegisterCapabilitiesRequest 顶层未知 varint 字段被 skipField")
    void registerCapabilitiesUnknownTopLevelFieldIsSkipped() {
        // field 5, wire type 0（varint）：0x28 = (5<<3)|0
        byte[] unknownVarint = {0x28, 0x01};
        SdkWireMessages.RegisterCapabilitiesRequest capabilities =
            SdkWireMessages.decodeRegisterCapabilitiesRequest(unknownVarint);
        assertEquals("", capabilities.provider.id);
        assertEquals(0, capabilities.manifestJsonGz.length);
    }

    @Test
    @DisplayName("encode 阶段 IOException 转为 IllegalStateException")
    void encodeFailureBecomesIllegalState() throws Exception {
        Class<?> encoderClass =
            Class.forName("io.github.cuihairu.croupier.sdk.wire.SdkWireMessages$Encoder");
        Object failing = Proxy.newProxyInstance(
            encoderClass.getClassLoader(),
            new Class<?>[]{encoderClass},
            (proxy, method, args) -> {
                throw new java.io.IOException("encode boom");
            });
        Method encode = SdkWireMessages.class.getDeclaredMethod("encode", encoderClass);
        encode.setAccessible(true);
        InvocationTargetException thrown = assertThrows(InvocationTargetException.class,
            () -> encode.invoke(null, failing));
        assertInstanceOf(IllegalStateException.class, thrown.getCause());
        assertEquals("Failed to encode protobuf message", thrown.getCause().getMessage());
    }
}
