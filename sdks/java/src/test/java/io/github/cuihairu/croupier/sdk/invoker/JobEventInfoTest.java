package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for JobEventInfo.
 */
class JobEventInfoTest {

    @Test
    @DisplayName("Builder should create JobEventInfo with all fields")
    void testBuilderAllFields() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-123")
                .type("completed")
                .message("Job completed successfully")
                .progress(100)
                .payload("result data")
                .error(null)
                .done(true)
                .build();

        assertEquals("job-123", event.getJobId());
        assertEquals("completed", event.getType());
        assertEquals("Job completed successfully", event.getMessage());
        assertEquals(100, event.getProgress());
        assertEquals("result data", event.getPayload());
        assertNull(event.getError());
        assertTrue(event.isDone());
    }

    @Test
    @DisplayName("Builder with partial values should work")
    void testBuilderPartialValues() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-456")
                .type("started")
                .build();

        assertEquals("job-456", event.getJobId());
        assertEquals("started", event.getType());
    }

    @Test
    @DisplayName("JobEventInfo with same values should be equal")
    void testEquals() {
        JobEventInfo event1 = JobEventInfo.builder()
                .jobId("job-1")
                .type("completed")
                .progress(100)
                .build();

        JobEventInfo event2 = JobEventInfo.builder()
                .jobId("job-1")
                .type("completed")
                .progress(100)
                .build();

        assertEquals(event1, event2);
    }

    @Test
    @DisplayName("toString should contain field values")
    void testToString() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-test")
                .type("progress")
                .progress(50)
                .build();

        String str = event.toString();
        assertTrue(str.contains("job-test"));
        assertTrue(str.contains("progress"));
    }

    @Test
    @DisplayName("Started event should have type 'started'")
    void testStartedEvent() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-123")
                .type("started")
                .message("Job started")
                .progress(0)
                .build();

        assertEquals("started", event.getType());
        assertEquals(0, event.getProgress());
    }

    @Test
    @DisplayName("Completed event should have type 'completed'")
    void testCompletedEvent() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-123")
                .type("completed")
                .message("Job completed")
                .progress(100)
                .payload("done")
                .done(true)
                .build();

        assertEquals("completed", event.getType());
        assertEquals(100, event.getProgress());
        assertTrue(event.isDone());
    }

    @Test
    @DisplayName("Error event should have type 'error'")
    void testErrorEvent() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-123")
                .type("error")
                .message("Something went wrong")
                .error("Something went wrong")
                .progress(0)
                .done(true)
                .build();

        assertEquals("error", event.getType());
        assertEquals("Something went wrong", event.getError());
        assertTrue(event.isDone());
    }

    @Test
    @DisplayName("Cancelled event should have type 'cancelled'")
    void testCancelledEvent() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-123")
                .type("cancelled")
                .message("Job cancelled by user")
                .progress(50)
                .done(true)
                .build();

        assertEquals("cancelled", event.getType());
        assertEquals("Job cancelled by user", event.getMessage());
        assertTrue(event.isDone());
    }

    @Test
    @DisplayName("Progress event should have progress value")
    void testProgressEvent() {
        JobEventInfo event = JobEventInfo.builder()
                .jobId("job-123")
                .type("progress")
                .message("Processing...")
                .progress(50)
                .build();

        assertEquals("progress", event.getType());
        assertEquals(50, event.getProgress());
        assertFalse(event.isDone());
    }
}
