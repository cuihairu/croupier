/**
 * 双车道派发（控制优先级）：业务洪峰下心跳依然可达。
 *
 * <p>回归背景：业务/控制共队列时，业务 handler 洪峰打满队列 → 心跳被
 * fail-fast 拒绝 → Agent 判定会话死亡 → 过载升级为连接雪崩。
 * 双车道后控制消息（心跳/注册/drain）走独立单 worker 车道，永不拒绝。</p>
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
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

@DisplayName("TCPTransport dual-lane control priority")
class TCPTransportDualLaneTest {

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

    private void acceptOne() throws IOException {
        server.setSoTimeout(5000);
        serverSide = server.accept();
        serverSide.setSoTimeout(1000);
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
        byte[] payload = in.readNBytes(len);
        return Protocol.parseMessage(payload);
    }

    @Test
    @DisplayName("业务队列打满时控制心跳仍被即时应答（会话存活）")
    void heartbeatSurvivesBusinessSaturation() throws Exception {
        int port = server.getLocalPort();
        transport = new TCPTransport("127.0.0.1", port, 5000);
        transport.setInboundWorkerCount(1); // 单业务 worker → 更易打满
        CountDownLatch businessStarted = new CountDownLatch(1);
        AtomicBoolean controlHandled = new AtomicBoolean(false);
        transport.setInboundListener((msgId, reqId, body) -> {
            if (Protocol.isControlRequest(msgId)) {
                controlHandled.set(true);
                return "pong".getBytes();
            }
            businessStarted.countDown();
            try {
                Thread.sleep(3000); // 占住唯一业务 worker
            } catch (InterruptedException ignored) {
            }
            return new byte[0];
        });
        transport.connect();
        acceptOne();

        // 打满业务车道：worker 1 个 + 队列上限 4 → 连发 6 个业务请求
        for (int reqId = 101; reqId <= 106; reqId++) {
            writeFrame(Protocol.MSG_INVOKE_REQUEST, reqId, new byte[0]);
        }
        assertTrue(businessStarted.await(2, TimeUnit.SECONDS),
            "business handler should have started");

        // 业务饱和后立刻发心跳：控制车道必须照常处理并应答。
        writeFrame(Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST, 2001, new byte[0]);

        boolean answered = false;
        long deadline = System.currentTimeMillis() + 5000;
        while (System.currentTimeMillis() < deadline) {
            Protocol.ParsedMessage resp;
            try {
                resp = readFrame();
            } catch (java.net.SocketTimeoutException e) {
                continue;
            }
            System.out.println("frame msg=" + Protocol.msgIdString(resp.msgId)
                + " req=" + resp.reqId + " body=" + new String(resp.body));
            if (resp.msgId == Protocol.MSG_PROVIDER_HEARTBEAT_RESPONSE
                && resp.reqId == 2001
                && "pong".equals(new String(resp.body))) {
                answered = true;
                break;
            }
        }
        assertTrue(controlHandled.get(), "control lane must handle heartbeat");
        assertTrue(answered, "heartbeat must be answered despite business saturation");
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
