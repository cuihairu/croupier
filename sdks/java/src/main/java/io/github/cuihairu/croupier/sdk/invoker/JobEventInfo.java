package io.github.cuihairu.croupier.sdk.invoker;

import java.util.Objects;

/**
 * Information about a job event received during streaming.
 *
 * <p>Job events are emitted during asynchronous job execution and provide
 * real-time updates about job progress, completion, or errors.</p>
 *
 * <p>Event types:</p>
 * <ul>
 *   <li>{@code started} - Job has started execution</li>
 *   <li>{@code progress} - Job progress update with percentage</li>
 *   <li>{@code completed} - Job completed successfully</li>
 *   <li>{@code error} - Job failed with an error</li>
 *   <li>{@code cancelled} - Job was cancelled</li>
 * </ul>
 */
public class JobEventInfo {

    private final String type;
    private final String jobId;
    private final String payload;
    private final String message;
    private final Integer progress;
    private final String error;
    private final boolean done;

    private JobEventInfo(Builder builder) {
        this.type = builder.type;
        this.jobId = builder.jobId;
        this.payload = builder.payload;
        this.message = builder.message;
        this.progress = builder.progress;
        this.error = builder.error;
        this.done = builder.done;
    }

    /**
     * Creates a new builder for constructing JobEventInfo instances.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Gets the event type.
     *
     * @return the event type (started, progress, completed, error, cancelled)
     */
    public String getType() {
        return type;
    }

    /**
     * Gets the job ID this event belongs to.
     *
     * @return the job ID
     */
    public String getJobId() {
        return jobId;
    }

    /**
     * Gets the event payload data (if any).
     *
     * @return the payload as a string, or null if not set
     */
    public String getPayload() {
        return payload;
    }

    /**
     * Gets the event message.
     *
     * @return the message, or null if not set
     */
    public String getMessage() {
        return message;
    }

    /**
     * Gets the progress percentage (if applicable).
     *
     * @return progress from 0-100, or null if not applicable
     */
    public Integer getProgress() {
        return progress;
    }

    /**
     * Gets the error message (if the event type is error).
     *
     * @return the error message, or null if not an error event
     */
    public String getError() {
        return error;
    }

    /**
     * Checks if this event indicates the job is finished.
     *
     * @return true if the job is done (completed, error, or cancelled)
     */
    public boolean isDone() {
        return done;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        JobEventInfo that = (JobEventInfo) o;
        return done == that.done &&
               Objects.equals(type, that.type) &&
               Objects.equals(jobId, that.jobId) &&
               Objects.equals(payload, that.payload) &&
               Objects.equals(message, that.message) &&
               Objects.equals(progress, that.progress) &&
               Objects.equals(error, that.error);
    }

    @Override
    public int hashCode() {
        return Objects.hash(type, jobId, payload, message, progress, error, done);
    }

    @Override
    public String toString() {
        return "JobEventInfo{" +
               "type='" + type + '\'' +
               ", jobId='" + jobId + '\'' +
               ", payload='" + payload + '\'' +
               ", message='" + message + '\'' +
               ", progress=" + progress +
               ", error='" + error + '\'' +
               ", done=" + done +
               '}';
    }

    /**
     * Builder for creating JobEventInfo instances.
     */
    public static class Builder {
        private String type;
        private String jobId;
        private String payload;
        private String message;
        private Integer progress;
        private String error;
        private boolean done;

        /**
         * Sets the event type.
         *
         * @param type the event type
         * @return this builder
         */
        public Builder type(String type) {
            this.type = type;
            return this;
        }

        /**
         * Sets the job ID.
         *
         * @param jobId the job ID
         * @return this builder
         */
        public Builder jobId(String jobId) {
            this.jobId = jobId;
            return this;
        }

        /**
         * Sets the event payload.
         *
         * @param payload the payload data
         * @return this builder
         */
        public Builder payload(String payload) {
            this.payload = payload;
            return this;
        }

        /**
         * Sets the event message.
         *
         * @param message the message
         * @return this builder
         */
        public Builder message(String message) {
            this.message = message;
            return this;
        }

        /**
         * Sets the progress percentage.
         *
         * @param progress progress from 0-100
         * @return this builder
         */
        public Builder progress(int progress) {
            this.progress = progress;
            return this;
        }

        /**
         * Sets the error message.
         *
         * @param error the error message
         * @return this builder
         */
        public Builder error(String error) {
            this.error = error;
            return this;
        }

        /**
         * Marks this event as done (completed, error, or cancelled).
         *
         * @param done true if the job is finished
         * @return this builder
         */
        public Builder done(boolean done) {
            this.done = done;
            return this;
        }

        /**
         * Builds the JobEventInfo instance.
         *
         * @return a new JobEventInfo
         */
        public JobEventInfo build() {
            return new JobEventInfo(this);
        }
    }
}
