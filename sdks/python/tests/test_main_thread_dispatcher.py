# Copyright 2025 Croupier Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Tests for MainThreadDispatcher."""

import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import List

import pytest

from croupier.dispatcher import MainThreadDispatcher


@pytest.fixture(autouse=True)
def reset_dispatcher():
    """Reset the dispatcher before and after each test."""
    MainThreadDispatcher.reset_instance()
    MainThreadDispatcher.get_instance().initialize()
    yield
    MainThreadDispatcher.reset_instance()


def test_initialize_sets_main_thread():
    """Test that initialization sets the main thread correctly."""
    assert MainThreadDispatcher.get_instance().is_initialized()
    assert MainThreadDispatcher.get_instance().is_main_thread()


def test_enqueue_from_main_thread_executes_immediately():
    """Test that enqueue from main thread executes immediately."""
    executed = [False]

    MainThreadDispatcher.get_instance().enqueue(lambda: executed.__setitem__(0, True))

    assert executed[0]


def test_enqueue_from_background_thread_queues_for_later():
    """Test that enqueue from background thread queues the callback."""
    executed = [False]
    event = threading.Event()

    def background_task():
        MainThreadDispatcher.get_instance().enqueue(lambda: executed.__setitem__(0, True))
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    # Should not be executed yet
    assert not executed[0]
    assert MainThreadDispatcher.get_instance().get_pending_count() == 1

    # Process the queue
    processed = MainThreadDispatcher.get_instance().process_queue()

    assert processed == 1
    assert executed[0]
    assert MainThreadDispatcher.get_instance().get_pending_count() == 0


def test_enqueue_with_data_passes_data_correctly():
    """Test that enqueue_with_data passes data correctly."""
    received_data = [None]
    event = threading.Event()

    def background_task():
        MainThreadDispatcher.get_instance().enqueue_with_data(
            lambda data: received_data.__setitem__(0, data), "test-data"
        )
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    MainThreadDispatcher.get_instance().process_queue()

    assert received_data[0] == "test-data"


def test_process_queue_respects_max_count():
    """Test that process_queue respects the max count parameter."""
    count = [0]
    events_done = threading.Event()

    def background_task():
        for _ in range(10):
            MainThreadDispatcher.get_instance().enqueue(lambda: count.__setitem__(0, count[0] + 1))
        events_done.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    events_done.wait(timeout=5)
    thread.join()

    assert MainThreadDispatcher.get_instance().get_pending_count() == 10

    # Process only 5
    processed = MainThreadDispatcher.get_instance().process_queue(5)

    assert processed == 5
    assert count[0] == 5
    assert MainThreadDispatcher.get_instance().get_pending_count() == 5

    # Process remaining
    processed = MainThreadDispatcher.get_instance().process_queue()
    assert processed == 5
    assert count[0] == 10


def test_process_queue_handles_exceptions():
    """Test that process_queue handles exceptions gracefully."""
    results: List[int] = []
    event = threading.Event()

    def background_task():
        MainThreadDispatcher.get_instance().enqueue(lambda: results.append(1))
        MainThreadDispatcher.get_instance().enqueue(
            lambda: (_ for _ in ()).throw(RuntimeError("Test"))
        )
        MainThreadDispatcher.get_instance().enqueue(lambda: results.append(3))
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    # Should process all callbacks even with exception
    processed = MainThreadDispatcher.get_instance().process_queue()

    assert processed == 3
    assert len(results) == 2
    assert 1 in results
    assert 3 in results


def test_clear_removes_all_pending_callbacks():
    """Test that clear removes all pending callbacks."""
    event = threading.Event()

    def background_task():
        for _ in range(5):
            MainThreadDispatcher.get_instance().enqueue(lambda: None)
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    assert MainThreadDispatcher.get_instance().get_pending_count() == 5

    MainThreadDispatcher.get_instance().clear()

    assert MainThreadDispatcher.get_instance().get_pending_count() == 0


def test_set_max_process_per_frame_limits_processing():
    """Test that set_max_process_per_frame limits processing."""
    MainThreadDispatcher.get_instance().set_max_process_per_frame(3)
    event = threading.Event()

    def background_task():
        for _ in range(10):
            MainThreadDispatcher.get_instance().enqueue(lambda: None)
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    processed = MainThreadDispatcher.get_instance().process_queue()
    assert processed == 3

    # Reset to default
    MainThreadDispatcher.get_instance().set_max_process_per_frame(0)


def test_enqueue_null_callback_is_ignored():
    """Test that enqueue with None callback is ignored."""
    initial_count = MainThreadDispatcher.get_instance().get_pending_count()
    event = threading.Event()

    def background_task():
        MainThreadDispatcher.get_instance().enqueue(None)  # type: ignore
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    assert MainThreadDispatcher.get_instance().get_pending_count() == initial_count


def test_concurrent_enqueue_is_thread_safe():
    """Test that concurrent enqueue operations are thread safe."""
    counter = [0]
    lock = threading.Lock()
    thread_count = 10
    enqueues_per_thread = 100

    def increment():
        with lock:
            counter[0] += 1

    def worker():
        for _ in range(enqueues_per_thread):
            MainThreadDispatcher.get_instance().enqueue(increment)

    with ThreadPoolExecutor(max_workers=thread_count) as executor:
        futures = [executor.submit(worker) for _ in range(thread_count)]
        for future in as_completed(futures):
            future.result()  # Ensure no exceptions

    # Process all
    total_processed = 0
    while True:
        processed = MainThreadDispatcher.get_instance().process_queue(100)
        if processed == 0:
            break
        total_processed += processed

    assert counter[0] == thread_count * enqueues_per_thread
    assert total_processed == thread_count * enqueues_per_thread


def test_is_main_thread_returns_false_on_background_thread():
    """Test that is_main_thread returns False on background thread."""
    is_main = [True]
    event = threading.Event()

    def background_task():
        is_main[0] = MainThreadDispatcher.get_instance().is_main_thread()
        event.set()

    thread = threading.Thread(target=background_task)
    thread.start()
    event.wait(timeout=5)
    thread.join()

    assert not is_main[0]
