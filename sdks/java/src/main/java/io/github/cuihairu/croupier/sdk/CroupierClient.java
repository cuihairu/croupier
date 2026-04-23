package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.JobEventInfo;
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

    // ========== Job Management Methods ==========

    /**
     * Starts an asynchronous job and returns its ID.
     *
     * <p>This is a convenience method that delegates to the Invoker's startJob method.</p>
     *
     * @param functionId the ID of the function to execute
     * @param payload the job payload as a JSON string
     *return the job ID for tracking
     * @throws CroupierException if job start fails
     */
    String startJob(String functionId, String payload) throws CroupierException;

    /**
     * Starts an asynchronous job with metadata and returns its ID.
     *
     * @param functionId the ID of the function to execute
     * @param payload the job payload as a JSON string
     * @param metadata additional metadata for the job
     * @return the job ID for tracking
     * @throws CroupierException if job start fails
     */
    String startJob(String functionId, String payload, Map<String, String> metadata) throws CroupierException;

    /**
     * Streams events from a running job.
     *
     * <p>This is a convenience method that delegates to the Invoker's streamJob method.</p>
     *
     * @param jobId the job ID to stream events for
     * @return a Publisher that emits JobEventInfo objects
     */
    Publisher<JobEventInfo> streamJob(String jobId);

    /**
     * Cancels a running job.
     *
     * <p>This is a convenience method that delegates to the Invoker's cancelJob method.</p>
     *
     * @param jobId the job ID to cancel
     * @return true if cancellation was successful, false otherwise
     * @throws CroupierException if cancellation fails
     */
    boolean cancelJob(String jobId) throws CroupierException;
}
