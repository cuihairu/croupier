package io.github.cuihairu.croupier.sdk.invoker;

import java.util.Objects;

/**
 * Exception thrown by the Invoker when operations fail.
 *
 * <p>This exception contains error details including an error code and message
 * to help diagnose invocation failures.</p>
 */
public class InvokerException extends Exception {

    private final ErrorCode errorCode;

    /**
     * Error codes for invoker failures.
     */
    public enum ErrorCode {
        /** Connection failed or lost */
        CONNECTION_FAILED("CONNECTION_FAILED"),
        /** Connection timeout */
        CONNECTION_TIMEOUT("CONNECTION_TIMEOUT"),
        /** Function not found */
        NOT_FOUND("NOT_FOUND"),
        /** Invalid arguments or payload */
        INVALID_ARGUMENT("INVALID_ARGUMENT"),
        /** Authentication failed */
        UNAUTHENTICATED("UNAUTHENTICATED"),
        /** Permission denied */
        PERMISSION_DENIED("PERMISSION_DENIED"),
        /** Internal server error */
        INTERNAL("INTERNAL"),
        /** Service unavailable */
        UNAVAILABLE("UNAVAILABLE"),
        /** Unknown error */
        UNKNOWN("UNKNOWN"),
        /** Operation cancelled */
        CANCELLED("CANCELLED"),
        /** Operation timeout */
        TIMEOUT("TIMEOUT"),
        /** Resource already exists */
        ALREADY_EXISTS("ALREADY_EXISTS"),
        /** Resource exhausted */
        RESOURCE_EXHAUSTED("RESOURCE_EXHAUSTED"),
        /** Failed precondition */
        FAILED_PRECONDITION("FAILED_PRECONDITION"),
        /** Operation aborted */
        ABORTED("ABORTED"),
        /** Out of range */
        OUT_OF_RANGE("OUT_OF_RANGE"),
        /** Unimplemented operation */
        UNIMPLEMENTED("UNIMPLEMENTED"),
        /** Data loss */
        DATA_LOSS("DATA_LOSS");

        private final String code;

        ErrorCode(String code) {
            this.code = code;
        }

        public String getCode() {
            return code;
        }
    }

    /**
     * Creates a new InvokerException with an error code and message.
     *
     * @param errorCode the error code
     * @param message the error message
     */
    public InvokerException(ErrorCode errorCode, String message) {
        super(message);
        this.errorCode = errorCode;
    }

    /**
     * Creates a new InvokerException with an error code, message, and cause.
     *
     * @param errorCode the error code
     * @param message the error message
     * @param cause the underlying cause
     */
    public InvokerException(ErrorCode errorCode, String message, Throwable cause) {
        super(message, cause);
        this.errorCode = errorCode;
    }

    /**
     * Creates a new InvokerException from a status code and message.
     * Status codes follow standard error code conventions (compatible with gRPC codes).
     *
     * @param statusCode the status code as an integer
     * @param message the error message
     * @return a new InvokerException with appropriate error code
     */
    public static InvokerException fromStatusCode(int statusCode, String message) {
        ErrorCode errorCode = fromStatusCode(statusCode);
        return new InvokerException(errorCode, message);
    }

    /**
     * Creates a new InvokerException from a status code, message, and cause.
     * Status codes follow standard error code conventions (compatible with gRPC codes).
     *
     * @param statusCode the status code as an integer
     * @param message the error message
     * @param cause the underlying cause
     * @return a new InvokerException with appropriate error code
     */
    public static InvokerException fromStatusCode(int statusCode, String message, Throwable cause) {
        ErrorCode errorCode = fromStatusCode(statusCode);
        return new InvokerException(errorCode, message, cause);
    }

    /**
     * @deprecated Use {@link #fromStatusCode(int, String)} instead.
     */
    @Deprecated
    public static InvokerException fromGrpcStatus(int grpcStatusCode, String message) {
        return fromStatusCode(grpcStatusCode, message);
    }

    /**
     * @deprecated Use {@link #fromStatusCode(int, String, Throwable)} instead.
     */
    @Deprecated
    public static InvokerException fromGrpcStatus(int grpcStatusCode, String message, Throwable cause) {
        return fromStatusCode(grpcStatusCode, message, cause);
    }

    private static ErrorCode fromStatusCode(int statusCode) {
        // Map standard status codes to our error codes
        // Compatible with gRPC status codes for backward compatibility
        return switch (statusCode) {
            case 1 -> ErrorCode.CANCELLED;
            case 2 -> ErrorCode.UNKNOWN;
            case 3 -> ErrorCode.INVALID_ARGUMENT;
            case 4 -> ErrorCode.TIMEOUT;
            case 5 -> ErrorCode.NOT_FOUND;
            case 6 -> ErrorCode.ALREADY_EXISTS;
            case 7 -> ErrorCode.PERMISSION_DENIED;
            case 8 -> ErrorCode.RESOURCE_EXHAUSTED;
            case 9 -> ErrorCode.FAILED_PRECONDITION;
            case 10 -> ErrorCode.ABORTED;
            case 11 -> ErrorCode.OUT_OF_RANGE;
            case 12 -> ErrorCode.UNIMPLEMENTED;
            case 13 -> ErrorCode.INTERNAL;
            case 14 -> ErrorCode.UNAVAILABLE;
            case 15 -> ErrorCode.DATA_LOSS;
            case 16 -> ErrorCode.UNAUTHENTICATED;
            default -> ErrorCode.UNKNOWN;
        };
    }

    /**
     * Gets the error code associated with this exception.
     *
     * @return the error code
     */
    public ErrorCode getErrorCode() {
        return errorCode;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        InvokerException that = (InvokerException) o;
        return errorCode == that.errorCode && Objects.equals(getMessage(), that.getMessage());
    }

    @Override
    public int hashCode() {
        return Objects.hash(errorCode, getMessage());
    }

    @Override
    public String toString() {
        return "InvokerException{" +
               "errorCode=" + errorCode +
               ", message='" + getMessage() + '\'' +
               '}';
    }
}
