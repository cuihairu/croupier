"""Tests for MainThreadDispatcher."""

import threading
import time

import pytest

from croupier.dispatcher import MainThreadDispatcher, get_dispatcher, enqueue, process


@pytest.fixture(autouse=True)
def reset_dispatcher():
    """Reset dispatcher singleton before each test."""
    MainThreadDispatcher.reset_instance()
    yield
    MainThreadDispatcher.reset_instance()


class TestMainThreadDispatcher:
    """Test MainThreadDispatcher class."""

    def test_singleton(self):
        """Test that MainThreadDispatcher is a singleton."""
        d1 = MainThreadDispatcher()
        d2 = MainThreadDispatcher()
        assert d1 is d2

    def test_get_instance(self):
        """Test get_instance returns singleton."""
        d1 = MainThreadDispatcher.get_instance()
        d2 = MainThreadDispatcher.get_instance()
        assert d1 is d2

    def test_initialize(self):
        """Test initialize sets main thread ID."""
        dispatcher = MainThreadDispatcher()
        assert dispatcher.is_initialized() is False

        dispatcher.initialize()
        assert dispatcher.is_initialized() is True

    def test_is_main_thread(self):
        """Test is_main_thread detection."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        # Should be on main thread in test
        assert dispatcher.is_main_thread() is True

    def test_is_main_thread_from_other_thread(self):
        """Test is_main_thread returns False from other thread."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        result = [None]

        def check_thread():
            result[0] = dispatcher.is_main_thread()

        thread = threading.Thread(target=check_thread)
        thread.start()
        thread.join()

        assert result[0] is False

    def test_enqueue_from_main_thread(self):
        """Test enqueue from main thread executes immediately."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def callback():
            executed.append(True)

        dispatcher.enqueue(callback)
        assert len(executed) == 1

    def test_enqueue_from_other_thread(self):
        """Test enqueue from other thread queues callback."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def callback():
            executed.append(True)

        def enqueue_from_thread():
            dispatcher.enqueue(callback)

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        # Callback should be queued, not executed yet
        assert len(executed) == 0
        assert dispatcher.get_pending_count() == 1

        # Process queue
        dispatcher.process_queue()
        assert len(executed) == 1

    def test_enqueue_none(self):
        """Test enqueue None does nothing."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        dispatcher.enqueue(None)
        assert dispatcher.get_pending_count() == 0

    def test_enqueue_with_data(self):
        """Test enqueue_with_data passes data to callback."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        result = []

        def callback(data):
            result.append(data)

        dispatcher.enqueue_with_data(callback, "test_data")
        assert result == ["test_data"]

    def test_process_queue(self):
        """Test process_queue processes callbacks."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def enqueue_from_thread():
            for i in range(5):
                dispatcher.enqueue(lambda i=i: executed.append(i))

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        processed = dispatcher.process_queue()
        assert processed == 5
        assert len(executed) == 5

    def test_process_queue_max_count(self):
        """Test process_queue respects max_count."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def enqueue_from_thread():
            for i in range(10):
                dispatcher.enqueue(lambda i=i: executed.append(i))

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        processed = dispatcher.process_queue(max_count=3)
        assert processed == 3
        assert len(executed) == 3
        assert dispatcher.get_pending_count() == 7

    def test_process_queue_empty(self):
        """Test process_queue on empty queue returns 0."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        processed = dispatcher.process_queue()
        assert processed == 0

    def test_process_queue_callback_error(self):
        """Test process_queue continues on callback error."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def enqueue_from_thread():
            dispatcher.enqueue(lambda: 1 / 0)  # Will raise ZeroDivisionError
            dispatcher.enqueue(lambda: executed.append(True))

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        processed = dispatcher.process_queue()
        assert processed == 2
        assert len(executed) == 1

    def test_get_pending_count(self):
        """Test get_pending_count returns queue size."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        assert dispatcher.get_pending_count() == 0

        def enqueue_from_thread():
            for i in range(5):
                dispatcher.enqueue(lambda: None)

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        assert dispatcher.get_pending_count() == 5

    def test_set_max_process_per_frame(self):
        """Test set_max_process_per_frame."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        dispatcher.set_max_process_per_frame(100)
        assert dispatcher._max_process_per_frame == 100

    def test_set_max_process_per_frame_zero(self):
        """Test set_max_process_per_frame with zero defaults to 1000."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        dispatcher.set_max_process_per_frame(0)
        assert dispatcher._max_process_per_frame == 1000

    def test_clear(self):
        """Test clear removes all pending callbacks."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        def enqueue_from_thread():
            for i in range(5):
                dispatcher.enqueue(lambda: None)

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        assert dispatcher.get_pending_count() == 5

        dispatcher.clear()
        assert dispatcher.get_pending_count() == 0

    def test_reset_instance(self):
        """Test reset_instance resets singleton."""
        d1 = MainThreadDispatcher.get_instance()
        d1.initialize()

        MainThreadDispatcher.reset_instance()

        d2 = MainThreadDispatcher.get_instance()
        assert d1 is not d2
        assert d2.is_initialized() is False


class TestConvenienceFunctions:
    """Test convenience functions."""

    def test_get_dispatcher(self):
        """Test get_dispatcher returns singleton."""
        d = get_dispatcher()
        assert isinstance(d, MainThreadDispatcher)

    def test_enqueue_function(self):
        """Test enqueue convenience function."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def enqueue_from_thread():
            enqueue(lambda: executed.append(True))

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        assert dispatcher.get_pending_count() == 1

    def test_process_function(self):
        """Test process convenience function."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        def enqueue_from_thread():
            for i in range(3):
                enqueue(lambda: None)

        thread = threading.Thread(target=enqueue_from_thread)
        thread.start()
        thread.join()

        processed = process()
        assert processed == 3


class TestMainThreadDispatcherConcurrency:
    """Test MainThreadDispatcher thread safety."""

    def test_concurrent_enqueue(self):
        """Test concurrent enqueue from multiple threads."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []

        def enqueue_from_thread(thread_id):
            for i in range(10):
                dispatcher.enqueue(lambda i=i, tid=thread_id: executed.append((tid, i)))

        threads = []
        for i in range(5):
            thread = threading.Thread(target=enqueue_from_thread, args=(i,))
            threads.append(thread)
            thread.start()

        for thread in threads:
            thread.join()

        assert dispatcher.get_pending_count() == 50

        processed = dispatcher.process_queue()
        assert processed == 50
        assert len(executed) == 50

    def test_concurrent_enqueue_and_process(self):
        """Test concurrent enqueue and process."""
        dispatcher = MainThreadDispatcher()
        dispatcher.initialize()

        executed = []
        stop = False

        def enqueue_thread():
            for i in range(20):
                dispatcher.enqueue(lambda i=i: executed.append(i))
                time.sleep(0.001)

        def process_thread():
            total = 0
            while not stop or dispatcher.get_pending_count() > 0:
                total += dispatcher.process_queue()
                time.sleep(0.001)
            return total

        enqueue_t = threading.Thread(target=enqueue_thread)
        process_t = threading.Thread(target=process_thread)

        enqueue_t.start()
        process_t.start()

        enqueue_t.join()
        stop = True
        process_t.join()

        assert len(executed) == 20
