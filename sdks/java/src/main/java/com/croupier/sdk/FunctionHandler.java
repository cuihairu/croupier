package com.croupier.sdk;

/**
 * Function handler interface for game function implementations
 */
@FunctionalInterface
public interface FunctionHandler {
    /**
     * Handle a function call
     *
     * @param context Function execution context (JSON string)
     * @param payload Function payload (JSON string)
     * @return Function result (JSON string)
     * @throws Exception if function execution fails
     */
    String handle(String context, String payload) throws Exception;
}