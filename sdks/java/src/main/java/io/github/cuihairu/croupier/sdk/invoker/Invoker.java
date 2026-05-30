package io.github.cuihairu.croupier.sdk.invoker;

import java.util.Map;
import org.reactivestreams.Publisher;

/**
 * Interface for invoking functions registered with the Croupier platform.
 *
 * <p>The Invoker provides client functionality for calling remote functions,
 * supporting both synchronous calls and asynchronous jobs with event streaming.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * Invoker invoker = CroupierSDK.createInvoker();
 * invoker.connect().get();
 *
 * // Synchronous invocation
 * String result = invoker.invoke("player.ban", "{\"player_id\":\"123\"}").get();
 *
 * // Asynchronous job
 * String jobId = invoker.startJob("player.ban", "{\"player_id\":\"456\"}").get();
 *
 * // Stream job events
 * invoker.streamJob(jobId)
 *     .doOnNext(event -> System.out.println("Event: " + event.getType()))
 *     .subscribe();
 *
 * invoker.close().get();
 * }</pre>
 */
public interface Invoker {

    /**
     * Connects to the server.
     *
     * <p>This method establishes a connection to the configured server address.
     * It is automatically called by invoke/startJob if not connected.</p>
     *
     * @throws InvokerException if connection fails
     */
    void connect() throws InvokerException;

    /**
     * Synchronously invokes a function and returns the result.
     *
     * <p>This method blocks until the function completes and returns the result.</p>
     *
     * @param functionId the ID of the function to invoke
     * @param payload the function payload as a JSON string
     * @return the function result as a string
     * @throws InvokerException if invocation fails
     */
    String invoke(String functionId, String payload) throws InvokerException;

    /**
     * Synchronously invokes a function with options and returns the result.
     *
     * @param functionId the ID of the function to invoke
     * @param payload the function payload as a JSON string
     * @param options invocation options (idempotency key, timeout, headers)
     * @return the function result as a string
     * @throws InvokerException if invocation fails
     */
    String invoke(String functionId, String payload, InvokeOptions options) throws InvokerException;

    /**
     * Starts an asynchronous job and returns its ID.
     *
     * <p>The job runs in the background and can be monitored using streamJob.</p>
     *
     * @param functionId the ID of the function to invoke
     * @param payload the function payload as a JSON string
     * @return the job ID for tracking
     * @throws InvokerException if job start fails
     */
    String startJob(String functionId, String payload) throws InvokerException;

    /**
     * Starts an asynchronous job with options and returns its ID.
     *
     * @param functionId the ID of the function to invoke
     * @param payload the function payload as a JSON string
     * @param options invocation options
     * @return the job ID for tracking
     * @throws InvokerException if job start fails
     */
    String startJob(String functionId, String payload, InvokeOptions options) throws InvokerException;

    /**
     * Streams events from a running job.
     *
     * <p>Returns a reactive stream of job events that can be subscribed to.
     * The stream completes when the job finishes (completed, error, or cancelled).</p>
     *
     * @param jobId the job ID to stream events for
     * @return a Publisher that emits JobEventInfo objects
     */
    Publisher<JobEventInfo> streamJob(String jobId);

    /**
     * Cancels a running job.
     *
     * @param jobId the job ID to cancel
     * @throws InvokerException if cancellation fails
     */
    void cancelJob(String jobId) throws InvokerException;

    /**
     * Sets a validation schema for a function.
     *
     * <p>The schema is used to validate payloads before invocation.
     * Note: Full JSON Schema validation is not yet implemented.</p>
     *
     * @param functionId the function ID
     * @param schema the schema as a Map (will be converted to JSON internally)
     */
    void setSchema(String functionId, Map<String, Object> schema);

    /**
     * Closes the invoker and releases resources.
     *
     * @throws InvokerException if close fails
     */
    void close() throws InvokerException;

    /**
     * Checks if the invoker is connected to the server.
     *
     * @return true if connected, false otherwise
     */
    boolean isConnected();
}
