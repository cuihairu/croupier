package com.croupier.sdk.examples;

import io.github.cuihairu.croupier.sdk.CroupierSDK;
import io.github.cuihairu.croupier.sdk.invoker.InvokeOptions;
import io.github.cuihairu.croupier.sdk.invoker.Invoker;
import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;
import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import io.github.cuihairu.croupier.sdk.invoker.JobEventInfo;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * Comprehensive examples demonstrating the Croupier Java SDK Invoker functionality.
 *
 * <p>This class shows how to use the Invoker to call functions registered with
 * the Croupier platform, including synchronous calls, asynchronous jobs, and
 * event streaming.</p>
 *
 * <p>Before running these examples, ensure you have a Croupier server running
 * at the configured address.</p>
 */
public class InvokerExample {

    /**
     * Main entry point for running examples.
     */
    public static void main(String[] args) {
        System.out.println("🎮 Croupier Java SDK Invoker 示例");
        System.out.println("====================================");
        System.out.println();

        try {
            // Run all examples
            syncInvokeExample();
            asyncJobExample();
            jobStreamExample();
            jobCancelExample();
            schemaValidationExample();

            System.out.println("\n✅ 所有示例完成");

        } catch (Exception e) {
            System.err.println("❌ 示例执行失败: " + e.getMessage());
            e.printStackTrace();
        }
    }

    /**
     * Example 1: Synchronous function invocation.
     */
    static void syncInvokeExample() throws InvokerException {
        printHeader("同步调用示例 (Synchronous Invocation)");

        // Create invoker with custom configuration
        InvokerConfig config = InvokerConfig.builder()
            .address("127.0.0.1:8080")
            .timeout(30000)
            .insecure(true)
            .build();

        Invoker invoker = CroupierSDK.createInvoker(config);

        try {
            // Connect to server
            invoker.connect();
            System.out.println("✅ 已连接到服务器\n");

            // Prepare invocation payload
            String functionId = "player.ban";
            String payload = String.format("{\"player_id\":\"%s\",\"reason\":\"%s\",\"duration\":%d}",
                "12345", "作弊行为", 86400);

            // Set invocation options with idempotency key
            InvokeOptions options = InvokeOptions.builder()
                .idempotencyKey("sync-" + Instant.now().toEpochMilli())
                .header("X-Game-ID", "my-game")
                .header("X-Env", "development")
                .build();

            // Invoke function synchronously
            String result = invoker.invoke(functionId, payload, options);
            System.out.println("📨 调用结果: " + result);

        } catch (InvokerException e) {
            System.out.println("❌ 调用失败: " + e.getMessage());
        } finally {
            invoker.close();
        }
    }

    /**
     * Example 2: Asynchronous job execution.
     */
    static void asyncJobExample() throws InvokerException {
        printHeader("异步任务示例 (Asynchronous Job)");

        Invoker invoker = CroupierSDK.createInvoker();

        try {
            invoker.connect();
            System.out.println("✅ 已连接到服务器\n");

            // Start an asynchronous job
            String functionId = "player.ban";
            String payload = String.format("{\"player_id\":\"%s\",\"reason\":\"%s\",\"duration\":%d}",
                "67890", "严重违规", 604800);

            String jobId = invoker.startJob(functionId, payload);
            System.out.println("🚀 任务已启动，Job ID: " + jobId);

        } catch (InvokerException e) {
            System.out.println("❌ 任务失败: " + e.getMessage());
        } finally {
            invoker.close();
        }
    }

    /**
     * Example 3: Stream job events using Reactive Streams.
     */
    static void jobStreamExample() throws InvokerException {
        printHeader("流式任务事件示例 (Job Event Streaming)");

        Invoker invoker = CroupierSDK.createInvoker();

        try {
            invoker.connect();
            System.out.println("✅ 已连接到服务器\n");

            // Start a job
            String functionId = "player.ban";
            String payload = String.format("{\"player_id\":\"%s\",\"reason\":\"%s\",\"duration\":%d}",
                "11111", "测试流式", 3600);

            String jobId = invoker.startJob(functionId, payload);
            System.out.println("🚀 任务已启动，Job ID: " + jobId);
            System.out.println("📡 接收任务事件...\n");

            // Subscribe to job events
            invoker.streamJob(jobId).subscribe(new Subscriber<JobEventInfo>() {
                private Subscription subscription;
                private int eventCount = 0;

                @Override
                public void onSubscribe(Subscription s) {
                    this.subscription = s;
                    s.request(1); // Request first event
                }

                @Override
                public void onNext(JobEventInfo event) {
                    eventCount++;
                    System.out.printf("📬 事件 [%s]: %s%n", event.getType(), event.getMessage());

                    if (event.getPayload() != null && !event.getPayload().isEmpty()) {
                        System.out.println("   载荷: " + event.getPayload());
                    }

                    if (event.getProgress() != null) {
                        System.out.println("   进度: " + event.getProgress() + "%");
                    }

                    if (event.getError() != null) {
                        System.out.println("   错误: " + event.getError());
                    }

                    if (event.isDone()) {
                        System.out.println("✅ 任务完成 (共 " + eventCount + " 个事件)");
                    } else {
                        subscription.request(1); // Request next event
                    }
                }

                @Override
                public void onError(Throwable t) {
                    System.out.println("❌ 流式事件错误: " + t.getMessage());
                }

                @Override
                public void onComplete() {
                    System.out.println("✅ 事件流结束");
                }
            });

            // Wait for events to be processed
            Thread.sleep(2000);

        } catch (InvokerException | InterruptedException e) {
            System.out.println("❌ 操作失败: " + e.getMessage());
        } finally {
            invoker.close();
        }
    }

    /**
     * Example 4: Job cancellation.
     */
    static void jobCancelExample() throws InvokerException {
        printHeader("取消任务示例 (Job Cancellation)");

        Invoker invoker = CroupierSDK.createInvoker();

        try {
            invoker.connect();
            System.out.println("✅ 已连接到服务器\n");

            // Start a long-running task
            String functionId = "player.ban";
            String payload = String.format("{\"player_id\":\"%s\",\"reason\":\"%s\",\"duration\":%d}",
                "22222", "测试取消", 9999999);

            String jobId = invoker.startJob(functionId, payload);
            System.out.println("🚀 任务已启动，Job ID: " + jobId + "\n");

            // Wait a bit then cancel
            Thread.sleep(1000);

            // Cancel the job
            invoker.cancelJob(jobId);
            System.out.println("🛑 任务已取消: " + jobId + "\n");

        } catch (InvokerException | InterruptedException e) {
            System.out.println("❌ 操作失败: " + e.getMessage());
        } finally {
            invoker.close();
        }
    }

    /**
     * Example 5: Schema validation.
     */
    static void schemaValidationExample() throws InvokerException {
        printHeader("Schema 验证示例 (Schema Validation)");

        Invoker invoker = CroupierSDK.createInvoker();

        try {
            invoker.connect();
            System.out.println("✅ 已连接到服务器\n");

            // Set function schema
            Map<String, Object> schema = new HashMap<>();
            schema.put("type", "object");

            Map<String, Object> properties = new HashMap<>();

            Map<String, Object> playerIdProp = new HashMap<>();
            playerIdProp.put("type", "string");
            properties.put("player_id", playerIdProp);

            Map<String, Object> reasonProp = new HashMap<>();
            reasonProp.put("type", "string");
            properties.put("reason", reasonProp);

            Map<String, Object> durationProp = new HashMap<>();
            durationProp.put("type", "number");
            durationProp.put("minimum", 0);
            properties.put("duration", durationProp);

            schema.put("properties", properties);
            schema.put("required", java.util.List.of("player_id", "reason"));

            invoker.setSchema("player.ban", schema);
            System.out.println("✅ Schema 已设置\n");

            // Test valid payload
            String validPayload = String.format("{\"player_id\":\"%s\",\"reason\":\"%s\",\"duration\":%d}",
                "33333", "测试验证", 3600);

            System.out.println("测试有效载荷...");
            try {
                String result = invoker.invoke("player.ban", validPayload);
                System.out.println("✅ 有效载荷验证通过: " + result + "\n");
            } catch (InvokerException e) {
                System.out.println("❌ 有效载荷验证失败: " + e.getMessage() + "\n");
            }

            // Test invalid payload (missing required field)
            String invalidPayload = "{\"player_id\":\"33333\"}"; // Missing 'reason'

            System.out.println("测试无效载荷（缺少必需字段）...");
            try {
                invoker.invoke("player.ban", invalidPayload);
                System.out.println("❌ 无效载荷应该被拒绝\n");
            } catch (InvokerException e) {
                System.out.println("✅ 无效载荷被正确拒绝: " + e.getMessage() + "\n");
            }

        } catch (InvokerException e) {
            System.out.println("❌ 操作失败: " + e.getMessage());
        } finally {
            invoker.close();
        }
    }

    /**
     * Helper method to print section headers.
     */
    private static void printHeader(String title) {
        System.out.println("\n" + "=".repeat(60));
        System.out.println(title);
        System.out.println("=".repeat(60));
    }
}
