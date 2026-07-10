package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for TaskEventInfo.
 */
class TaskEventInfoTest {

    @Test
    @DisplayName("Builder should create TaskEventInfo with all fields")
    void testBuilderAllFields() {
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-123")
                .type("completed")
                .message("Task completed successfully")
                .progress(100)
                .payload("result data")
                .error(null)
                .done(true)
                .build();

        assertEquals("task-123", event.getTaskId());
        assertEquals("completed", event.getType());
        assertEquals("Task completed successfully", event.getMessage());
        assertEquals(100, event.getProgress());
        assertEquals("result data", event.getPayload());
        assertNull(event.getError());
        assertTrue(event.isDone());
    }

    @Test
    @DisplayName("Builder with partial values should work")
    void testBuilderPartialValues() {
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-456")
                .type("started")
                .build();

        assertEquals("task-456", event.getTaskId());
        assertEquals("started", event.getType());
    }

    @Test
    @DisplayName("TaskEventInfo with same values should be equal")
    void testEquals() {
        TaskEventInfo event1 = TaskEventInfo.builder()
                .taskId("task-1")
                .type("completed")
                .progress(100)
                .build();

        TaskEventInfo event2 = TaskEventInfo.builder()
                .taskId("task-1")
                .type("completed")
                .progress(100)
                .build();

        assertEquals(event1, event2);
    }

    @Test
    @DisplayName("toString should contain field values")
    void testToString() {
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-test")
                .type("progress")
                .progress(50)
                .build();

        String str = event.toString();
        assertTrue(str.contains("task-test"));
        assertTrue(str.contains("progress"));
    }

    @Test
    @DisplayName("Started event should have type 'started'")
    void testStartedEvent() {
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-123")
                .type("started")
                .message("Task started")
                .progress(0)
                .build();

        assertEquals("started", event.getType());
        assertEquals(0, event.getProgress());
    }

    @Test
    @DisplayName("Completed event should have type 'completed'")
    void testCompletedEvent() {
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-123")
                .type("completed")
                .message("Task completed")
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
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-123")
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
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-123")
                .type("cancelled")
                .message("Task cancelled by user")
                .progress(50)
                .done(true)
                .build();

        assertEquals("cancelled", event.getType());
        assertEquals("Task cancelled by user", event.getMessage());
        assertTrue(event.isDone());
    }

    @Test
    @DisplayName("Progress event should have progress value")
    void testProgressEvent() {
        TaskEventInfo event = TaskEventInfo.builder()
                .taskId("task-123")
                .type("progress")
                .message("Processing...")
                .progress(50)
                .build();

        assertEquals("progress", event.getType());
        assertEquals(50, event.getProgress());
        assertFalse(event.isDone());
    }
}
