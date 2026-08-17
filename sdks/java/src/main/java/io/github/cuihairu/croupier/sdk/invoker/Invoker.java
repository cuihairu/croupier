package io.github.cuihairu.croupier.sdk.invoker;

import java.util.Map;
import org.reactivestreams.Publisher;

/**
 * Interface for invoking functions registered with the Croupier platform.
 *
 * <p>The public L3 Invoker calls the Croupier Server HTTP API. It never reuses
 * the Provider TCP session, so Server authorization, scope checks, audit and
 * task persistence remain authoritative.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * Invoker invoker = CroupierSDK.createInvoker();
 * invoker.connect().get();
 *
 * // Synchronous invocation
 * String result = invoker.invoke("player.ban", "{\"player_id\":\"123\"}").get();
 *
 * // Asynchronous task
 * String taskId = invoker.startTask("player.ban", "{\"player_id\":\"456\"}").get();
 *
 * // Stream task events
 * invoker.streamTask(taskId)
 *     .doOnNext(event -> System.out.println("Event: " + event.getType()))
 *     .subscribe();
 *
 * invoker.close().get();
 * }</pre>
 */
public interface Invoker {

    /**
     * Marks the request-based Server invoker ready for use.
     *
     * <p>HTTP does not open a Provider-like persistent session. Requests may
     * still be issued without calling this method.</p>
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
     * Starts an asynchronous task and returns its ID.
     *
     * <p>The task runs in the background and can be monitored using streamTask.</p>
     *
     * @param functionId the ID of the function to invoke
     * @param payload the function payload as a JSON string
     * @return the task ID for tracking
     * @throws InvokerException if task start fails
     */
    String startTask(String functionId, String payload) throws InvokerException;

    /**
     * Starts an asynchronous task with options and returns its ID.
     *
     * @param functionId the ID of the function to invoke
     * @param payload the function payload as a JSON string
     * @param options invocation options
     * @return the task ID for tracking
     * @throws InvokerException if task start fails
     */
    String startTask(String functionId, String payload, InvokeOptions options) throws InvokerException;

    /**
     * Streams events from a running task.
     *
     * <p>Returns a reactive stream of task events that can be subscribed to.
     * The stream completes when the task finishes (completed, error, or cancelled).</p>
     *
     * @param taskId the task ID to stream events for
     * @return a Publisher that emits TaskEventInfo objects
     */
    Publisher<TaskEventInfo> streamTask(String taskId);

    /**
     * Cancels a running task.
     *
     * @param taskId the task ID to cancel
     * @throws InvokerException if cancellation fails
     */
    void cancelTask(String taskId) throws InvokerException;

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
