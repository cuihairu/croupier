package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** AUD-069: pending-approval responses are preserved instead of being mis-decoded. */
class ServerHttpInvokerPendingApprovalTest {

    @Test
    void invokePendingApprovalSurfacesApprovalCredentials() throws Exception {
        try (ApprovalServer server = new ApprovalServer("{\"taskId\":\"\",\"approvalId\":\"apr-1\","
            + "\"approvalRequired\":true,\"approvalWorkflow\":\"two_person\",\"traceId\":\"tr-1\"}")) {
            ServerHttpInvoker invoker = invoker(server);
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.APPROVAL_REQUIRED, error.getErrorCode());
            assertEquals("apr-1", error.getApprovalId());
            assertEquals("two_person", error.getApprovalWorkflow());
            assertTrue(error.getMessage().contains("apr-1"));
        }
    }

    @Test
    void startTaskPendingApprovalSurfacesApprovalCredentials() throws Exception {
        try (ApprovalServer server = new ApprovalServer("{\"taskId\":\"\",\"status\":\"approval_required\","
            + "\"approvalId\":\"apr-2\",\"approvalWorkflow\":\"single_admin\"}")) {
            ServerHttpInvoker invoker = invoker(server);
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.startTask("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.APPROVAL_REQUIRED, error.getErrorCode());
            assertEquals("apr-2", error.getApprovalId());
            assertEquals("single_admin", error.getApprovalWorkflow());
            assertEquals("approval_required", error.getApprovalStatus());
        }
    }

    @Test
    void missingApprovalIdStaysMissingOutcome() throws Exception {
        try (ApprovalServer server = new ApprovalServer("{\"taskId\":\"\",\"status\":\"approval_required\"}")) {
            ServerHttpInvoker invoker = invoker(server);
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.startTask("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.INTERNAL, error.getErrorCode());
            assertTrue(error.getMessage().contains("taskId"));
        }
    }

    private static ServerHttpInvoker invoker(ApprovalServer server) {
        return new ServerHttpInvoker(InvokerConfig.builder()
            .address(server.baseUrl())
            .retry(RetryConfig.builder().enabled(false).build())
            .build());
    }

    private static final class ApprovalServer implements AutoCloseable {
        private final HttpServer server;
        private final String body;
        private final List<String> paths = new ArrayList<>();

        ApprovalServer(String body) throws IOException {
            this.body = body;
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", this::respond);
            server.start();
        }

        private void respond(HttpExchange exchange) throws IOException {
            paths.add(exchange.getRequestURI().getPath());
            byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().add("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, bytes.length);
            exchange.getResponseBody().write(bytes);
            exchange.close();
        }

        String baseUrl() {
            return "http://127.0.0.1:" + server.getAddress().getPort();
        }

        @Override
        public void close() {
            server.stop(0);
        }
    }
}
