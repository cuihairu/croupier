/**
 * 入站派发路径测试（覆盖 dispatchInbound / inboundPool / writeResponseSilently /
 * readLoop 入站分支 / setInboundListener——与 C++/C# 同款语义验证）。
 */
package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.net.ServerSocket;
import java.net.Socket;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

@DisplayName("TCPTransport inbound dispatch")
class TCPTransportInboundTest {

    private ServerSocket server;
    private Socket serverSide;
    private DataInputStream in;
    private DataOutputStream out;
    private TCPTransport transport;

    @BeforeEach
    void setUp() throws IOException {
        server = new ServerSocket(0);
    }

    @AfterEach
    void tearDown() {
        if (transport != null) {
            try {
                transport.close();
            } catch (Exception ignored) {
            }
        }
        closeQuietly(serverSide);
        closeQuietly(server);
    }

    /** 接受一条连接，供测试端直接读写帧（无 mock 循环）。 */
    private void acceptOne() throws IOException {
        server.setSoTimeout(5000);
        serverSide = server.accept();
        in = new DataInputStream(serverSide.getInputStream());
        out = new DataOutputStream(serverSide.getOutputStream());
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

    @Test
    @DisplayName("agent 推送 invoke：listener 执行并回写 payload 响应")
    void invokeReachesListenerAndResponds() throws Exception {
        int port = server.getLocalPort();
        AtomicInteger calls = new AtomicInteger();
        transport = new TCPTransport("127.0.0.1", port, 3000);
        transport.setInboundListener((msgId, reqId, body) -> {
            calls.incrementAndGet();
            return body;
        });
        transport.connect();
        acceptOne();

        byte[] body = {1, 2, 3, 4};
        writeFrame(Protocol.MSG_INVOKE_REQUEST, 7001, body);

        Protocol.ParsedMessage resp = readFrame();
        assertEquals(7001, resp.reqId);
        assertArrayEquals(body, resp.body);
        // 轮询等待 listener 计数（worker 异步执行）。
        for (int i = 0; i < 40 && calls.get() < 1; i++) {
            Thread.sleep(50);
        }
        assertTrue(calls.get() >= 1);
    }

    @Test
    @DisplayName("listener 抛异常：回空响应且连接仍可用")
    void handlerExceptionYieldsEmptyResponseAndConnectionSurvives() throws Exception {
        int port = server.getLocalPort();
        transport = new TCPTransport("127.0.0.1", port, 3000);
        transport.setInboundListener((msgId, reqId, body) -> {
            if (body.length == 0) {
                throw new IllegalStateException("boom");
            }
            return body;
        });
        transport.connect();
        acceptOne();

        writeFrame(Protocol.MSG_INVOKE_REQUEST, 7002, new byte[0]);
        Protocol.ParsedMessage resp = readFrame();
        assertEquals(7002, resp.reqId);
        assertEquals(0, resp.body.length);

        // 连接仍可用：合法请求正常回。
        byte[] ok = {9};
        writeFrame(Protocol.MSG_INVOKE_REQUEST, 7003, ok);
        Protocol.ParsedMessage resp2 = readFrame();
        assertArrayEquals(ok, resp2.body);
    }

    @Test
    @DisplayName("未设置 listener：帧被丢弃、连接保持")
    void noListenerDropsFrame() throws Exception {
        int port = server.getLocalPort();
        transport = new TCPTransport("127.0.0.1", port, 3000);
        transport.connect();
        acceptOne();

        writeFrame(Protocol.MSG_INVOKE_REQUEST, 7004, new byte[] {5});
        // 短等待后 transport 仍应处于连接态（无响应、无崩溃）。
        Thread.sleep(200);
        assertTrue(transport.isConnected());
    }

    @Test
    @DisplayName("并发推送：全部请求收到响应（有界池 + 队列语义）")
    void concurrentInvocationsAllAnswered() throws Exception {
        int port = server.getLocalPort();
        transport = new TCPTransport("127.0.0.1", port, 5000);
        transport.setInboundListener((msgId, reqId, body) -> {
            try {
                Thread.sleep(30);
            } catch (InterruptedException ignored) {
            }
            return body;
        });
        transport.connect();
        acceptOne();

        final int total = 16;
        for (int i = 0; i < total; i++) {
            writeFrame(Protocol.MSG_INVOKE_REQUEST, 8000 + i, new byte[] {(byte) i});
        }
        int answered = 0;
        for (int i = 0; i < total; i++) {
            Protocol.ParsedMessage resp = readFrame();
            // 空体（饱和快速失败）也计为应答——语义允许。
            if (resp.body.length == 1 || resp.body.length == 0) {
                answered++;
            }
        }
        assertEquals(total, answered);
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
