package com.croupier.sdk.examples;

import io.github.cuihairu.croupier.sdk.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Instant;
import java.util.concurrent.CountDownLatch;

/**
 * Basic example demonstrating Croupier Java SDK usage
 */
public class BasicExample {
    private static final Logger logger = LoggerFactory.getLogger(BasicExample.class);

    public static void main(String[] args) {
        logger.info("Starting Croupier SDK example...");

        try {
            // Create client configuration
            ClientConfig config = new ClientConfig("example-game", "example-service");
            config.setAgentAddr("localhost:19090");
            config.setEnv("development");
            config.setServiceVersion("1.0.0");
            config.setInsecure(true); // Use insecure gRPC for development

            // Create client
            CroupierClient client = CroupierSDK.createClient(config);

            // Register functions
            registerFunctions(client);

            // Setup graceful shutdown
            CountDownLatch shutdownLatch = new CountDownLatch(1);
            Runtime.getRuntime().addShutdownHook(new Thread(() -> {
                logger.info("Received shutdown signal");
                client.stop();
                shutdownLatch.countDown();
            }));

            // Start serving
            client.serveAsync().thenRun(() -> {
                logger.info("Service started successfully");
            }).exceptionally(throwable -> {
                logger.error("Failed to start service", throwable);
                shutdownLatch.countDown();
                return null;
            });

            // Wait for shutdown
            shutdownLatch.await();

            // Cleanup
            client.close();
            logger.info("Example completed");

        } catch (Exception e) {
            logger.error("Example failed", e);
            System.exit(1);
        }
    }

    private static void registerFunctions(CroupierClient client) throws CroupierException {
        // Register player ban function
        FunctionDescriptor playerBanDesc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                .resource("player")
                .risk("danger")
                .operation("ban")
                .summary("Ban player")
                .description("Ban a player account by player ID.")
                .enabled(true)
                .build();

        FunctionHandler playerBanHandler = (context, payload) -> {
            logger.info("🔨 Banning player with payload: {}", payload);

            // Simulate processing time
            Thread.sleep(100);

            String result = String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"action\": \"ban\",\n" +
                "  \"timestamp\": \"%s\",\n" +
                "  \"message\": \"Player banned successfully\"\n" +
                "}",
                Instant.now().toString()
            );

            return result;
        };

        client.registerFunction(playerBanDesc, playerBanHandler);

        // Register inventory grant function
        FunctionDescriptor itemCreateDesc = CroupierSDK.functionDescriptor("inventory.grant", "1.0.0")
                .resource("inventory")
                .risk("warning")
                .operation("grant")
                .summary("Grant inventory item")
                .description("Grant an item to a player's inventory.")
                .enabled(true)
                .build();

        FunctionHandler itemCreateHandler = (context, payload) -> {
            logger.info("📦 Creating item with payload: {}", payload);

            String result = String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"action\": \"create\",\n" +
                "  \"item_id\": \"item_%d\",\n" +
                "  \"timestamp\": \"%s\"\n" +
                "}",
                System.currentTimeMillis(),
                Instant.now().toString()
            );

            return result;
        };

        client.registerFunction(itemCreateDesc, itemCreateHandler);

        logger.info("✅ All functions registered successfully");
    }
}
