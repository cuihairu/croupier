/**
 * 真实 TCP 链路的 Agent→Provider 入站调用测试（覆盖 attachInboundListener /
 * handleLocalRequest / handleInvokeRequest——此前全部测试用 FakeTransportClient，
 * instanceof TCPTransport 分支与入站 lambda 从未执行）。
 */
package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.net.ServerSocket;
import java.net.Socket;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

@DisplayName("Agent→Provider inbound over real TCP")
class CroupierClientInboundTcpTest {

    private ServerSocket server;
    private Socket serverSide;
    private DataInputStream in;
    private DataOutputStream out;
    private CroupierClientImpl client;

    @BeforeEach
    void setUp() throws IOException {
        server = new ServerSocket(0);
    }

    @AfterEach
    void tearDown() {
        if (client != null) {
            client.stop();
        }
        closeQuietly(serverSide);
        closeQuietly(server);
    }

    private volatile String handshakeLog = "";

    private void acceptAndHandshake() throws IOException {
        server.setSoTimeout(10000);
        serverSide = server.accept();
        serverSide.setSoTimeout(8000); // 读超时：断言失败时线程可退出
        in = new DataInputStream(serverSide.getInputStream());
        out = new DataOutputStream(serverSide.getOutputStream());
        // 应答 ProviderConnectRequest。
        Protocol.ParsedMessage first = readFrame();
        handshakeLog = "first-frame=0x" + Integer.toHexString(first.msgId) + " reqId=" + first.reqId;
        System.err.println("[agent] " + handshakeLog);
        writeFrame(Protocol.MSG_PROVIDER_CONNECT_RESPONSE, first.reqId,
            SdkWireMessages.encodeProviderConnectResponse(
                new SdkWireMessages.ProviderConnectResponse("inbound-session")));
    }

    private void writeFrame(int msgId, int reqId, byte[] body) throws IOException {
        byte[] frame = Protocol.newMessage(msgId, reqId, body);
        out.writeInt(frame.length);
        out.write(frame);
        out.flush();
    }

    private Protocol.ParsedMessage readFrame() throws IOException {
        int len = in.readInt();
        byte[] payload = new byte[len];
        in.readFully(payload);
        return Protocol.parseMessage(payload);
    }

    private ClientConfig config(int port) {
        ClientConfig config = new ClientConfig();
        config.setAgentAddr("127.0.0.1:" + port);
        config.setGameId("game-test");
        config.setEnv("development");
        config.setServiceId("java-inbound-tests");
        config.setHeartbeatInterval(60); // 拉长心跳，避免与断言交错
        config.setTimeoutSeconds(5);
        return config;
    }

    @Test
    @Timeout(20)
    @DisplayName("agent 推送 invoke：经 TCPTransport 派发到本地 handler 并回写响应")
    void agentInvokeDispatchesToLocalHandler() throws Exception {
        int port = server.getLocalPort();
        client = new CroupierClientImpl(config(port));
        AtomicInteger calls = new AtomicInteger();
        client.registerFunction(new FunctionDescriptor("test.echo", "1.0.0"),
            (ctx, payload) -> {
                calls.incrementAndGet();
                return "echo:" + payload;
            });
        // connect() 会同步等 ProviderConnectResponse——agent 线程先行。
        Thread agent = new Thread(() -> {
            try {
                acceptAndHandshake();
            } catch (Exception e) {
                // 由主线程断言失败兜底
            }
        });
        agent.setDaemon(true);
        agent.start();
        client.connect().get(5, TimeUnit.SECONDS);
        agent.join(5000);

        byte[] body = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("test.echo", "", new byte[] {'h', 'i'}, Map.of()));
        writeFrame(Protocol.MSG_INVOKE_REQUEST, 6001, body);

        Protocol.ParsedMessage resp = readFrame();
        assertEquals(6001, resp.reqId);
        SdkWireMessages.InvokeResponse parsed = SdkWireMessages.decodeInvokeResponse(resp.body);
        assertEquals("echo:hi", parsed.payloadUtf8());
        assertTrue(calls.get() >= 1, "handler should have been invoked");
    }

    @Test
    @Timeout(20)
    @DisplayName("未知函数：空响应（handleInvokeRequest 的 not found 分支）")
    void unknownFunctionReturnsEmpty() throws Exception {
        int port = server.getLocalPort();
        client = new CroupierClientImpl(config(port));
        client.registerFunction(new FunctionDescriptor("known.fn", "1.0.0"),
            (ctx, payload) -> "{}");
        Thread agent = new Thread(() -> {
            try {
                acceptAndHandshake();
            } catch (Exception ignored) {
            }
        });
        agent.setDaemon(true);
        agent.start();
        client.connect().get(5, TimeUnit.SECONDS);
        agent.join(5000);

        byte[] body = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("missing.fn", "", new byte[0], Map.of()));
        writeFrame(Protocol.MSG_INVOKE_REQUEST, 6002, body);

        Protocol.ParsedMessage resp = readFrame();
        assertEquals(6002, resp.reqId);
        // 未注册函数回空体或错误 JSON——帧必须到达（连接存活）。
        assertTrue(resp.body.length == 0 || new String(resp.body).contains("error"));
    }

    private static void closeQuietly(java.io.Closeable c) {
        if (c != null) {
            try {
                c.close();
            } catch (IOException ignored) {
            }
        }
    }
}
