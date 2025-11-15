package com.croupier.sdk;

import java.util.concurrent.CompletableFuture;

/**
 * Croupier client interface for function registration and execution
 */
public interface CroupierClient {
    /**
     * Register a function with the agent
     *
     * @param descriptor Function descriptor
     * @param handler Function handler implementation
     * @throws CroupierException if registration fails
     */
    void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) throws CroupierException;

    /**
     * Connect to the agent
     *
     * @return CompletableFuture that completes when connection is established
     */
    CompletableFuture<Void> connect();

    /**
     * Start serving function calls
     * This method blocks until the service is stopped
     *
     * @throws CroupierException if serving fails
     */
    void serve() throws CroupierException;

    /**
     * Start serving function calls asynchronously
     *
     * @return CompletableFuture that completes when the service starts
     */
    CompletableFuture<Void> serveAsync();

    /**
     * Stop the client service gracefully
     */
    void stop();

    /**
     * Close the client and clean up resources
     */
    void close();

    /**
     * Get the local server address
     *
     * @return Local server address, or null if not started
     */
    String getLocalAddress();

    /**
     * Check if the client is connected to the agent
     *
     * @return true if connected
     */
    boolean isConnected();

    /**
     * Check if the client is serving
     *
     * @return true if serving
     */
    boolean isServing();
}