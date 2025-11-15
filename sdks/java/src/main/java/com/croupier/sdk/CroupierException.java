package com.croupier.sdk;

/**
 * Base exception for Croupier SDK operations
 */
public class CroupierException extends Exception {
    public CroupierException(String message) {
        super(message);
    }

    public CroupierException(String message, Throwable cause) {
        super(message, cause);
    }

    public CroupierException(Throwable cause) {
        super(cause);
    }
}