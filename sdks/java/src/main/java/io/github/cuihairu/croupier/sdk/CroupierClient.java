package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.TaskEventInfo;
import org.reactivestreams.Publisher;

import java.util.Map;
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
     * Check if the client is connected to the agent
     *
     * @return true if connected
     */
    boolean isConnected();

    /**
     * Get the current session ID from the agent
     *
     * @return session ID, or empty string if not connected
     */
    String getSessionId();

    /**
     * Check if the client is serving
     *
     * @return true if serving
     */
    boolean isServing();

    // ========== Task Management Methods ==========

    /**
     * Starts an asynchronous task and returns its ID.
     *
     * <p>This is a convenience method that delegates to the Invoker's startTask method.</p>
     *
     * @param functionId the ID of the function to execute
     * @param payload the task payload as a JSON string
     *return the task ID for tracking
     * @throws CroupierException if task start fails
     */
    String startTask(String functionId, String payload) throws CroupierException;

    /**
     * Starts an asynchronous task with metadata and returns its ID.
     *
     * @param functionId the ID of the function to execute
     * @param payload the task payload as a JSON string
     * @param metadata additional metadata for the task
     * @return the task ID for tracking
     * @throws CroupierException if task start fails
     */
    String startTask(String functionId, String payload, Map<String, String> metadata) throws CroupierException;

    /**
     * Streams events from a running task.
     *
     * <p>This is a convenience method that delegates to the Invoker's streamTask method.</p>
     *
     * @param taskId the task ID to stream events for
     * @return a Publisher that emits TaskEventInfo objects
     */
    Publisher<TaskEventInfo> streamTask(String taskId);

    /**
     * Cancels a running task.
     *
     * <p>This is a convenience method that delegates to the Invoker's cancelTask method.</p>
     *
     * @param taskId the task ID to cancel
     * @return true if cancellation was successful, false otherwise
     * @throws CroupierException if cancellation fails
     */
    boolean cancelTask(String taskId) throws CroupierException;
}
