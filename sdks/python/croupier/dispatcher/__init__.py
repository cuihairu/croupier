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

"""
Main thread dispatcher - ensures callbacks execute on the main thread.

Usage:
    1. Call MainThreadDispatcher.get_instance().initialize() once on the main thread
    2. Call MainThreadDispatcher.get_instance().process_queue() in your main loop
    3. Use MainThreadDispatcher.get_instance().enqueue() from any thread

Example:
    >>> from croupier.dispatcher import MainThreadDispatcher
    >>>
    >>> # In main thread initialization
    >>> MainThreadDispatcher.get_instance().initialize()
    >>>
    >>> # From any thread
    >>> MainThreadDispatcher.get_instance().enqueue(lambda: print("On main thread!"))
    >>>
    >>> # In main loop
    >>> MainThreadDispatcher.get_instance().process_queue()
"""

from __future__ import annotations

import logging
import queue
import threading
from typing import Callable, Optional, TypeVar

T = TypeVar("T")

LOG = logging.getLogger(__name__)


class MainThreadDispatcher:
    """
    Main thread dispatcher singleton.

    Ensures callbacks are executed on the main thread by queuing them
    and processing them when process_queue() is called.
    """

    _instance: Optional["MainThreadDispatcher"] = None
    _lock: threading.Lock = threading.Lock()
    _initialized: bool = False
    _main_thread_id: Optional[int] = None
    _max_process_per_frame: int = 1000
    _queue: queue.Queue[Callable[[], None]]

    def __new__(cls) -> "MainThreadDispatcher":
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    inst = super().__new__(cls)
                    inst._queue = queue.Queue()
                    cls._instance = inst
        return cls._instance

    @classmethod
    def get_instance(cls) -> "MainThreadDispatcher":
        """Get the singleton instance."""
        return cls()

    @classmethod
    def reset_instance(cls) -> None:
        """Reset the singleton instance. Primarily for testing."""
        with cls._lock:
            if cls._instance is not None:
                cls._instance.clear()
                cls._instance._initialized = False
                cls._instance._main_thread_id = None
            cls._instance = None

    def initialize(self) -> None:
        """Initialize the dispatcher. Must be called once on the main thread."""
        self._main_thread_id = threading.current_thread().ident
        self._initialized = True
        LOG.info("MainThreadDispatcher initialized on thread %s", self._main_thread_id)

    def is_initialized(self) -> bool:
        """Check if the dispatcher has been initialized."""
        return self._initialized

    def enqueue(self, callback: Callable[[], None]) -> None:
        """
        Enqueue a callback to be executed on the main thread.

        If called from the main thread and initialized, executes immediately.

        Args:
            callback: The callback to execute
        """
        if callback is None:
            return

        # If already on main thread and initialized, execute immediately
        if self._initialized and self.is_main_thread():
            try:
                callback()
            except Exception as e:
                LOG.error("Callback error (immediate): %s", e, exc_info=True)
            return

        self._queue.put(callback)

    def enqueue_with_data(self, callback: Callable[[T], None], data: T) -> None:
        """
        Enqueue a callback with data to be executed on the main thread.

        Args:
            callback: The callback to execute
            data: The data to pass to the callback
        """
        if callback is None:
            return
        self.enqueue(lambda: callback(data))

    def process_queue(self, max_count: Optional[int] = None) -> int:
        """
        Process queued callbacks on the main thread.

        Call this from your main loop.

        Args:
            max_count: Maximum number of callbacks to process.
                      Uses default if None.

        Returns:
            The number of callbacks processed
        """
        if max_count is None:
            max_count = self._max_process_per_frame

        processed = 0

        while processed < max_count:
            try:
                callback = self._queue.get_nowait()
            except queue.Empty:
                break

            try:
                callback()
            except Exception as e:
                # Log but don't interrupt processing
                LOG.error("Callback error: %s", e, exc_info=True)

            processed += 1

        return processed

    def get_pending_count(self) -> int:
        """Get the number of pending callbacks in the queue."""
        return int(self._queue.qsize())  # type: ignore[arg-type]

    def is_main_thread(self) -> bool:
        """Check if the current thread is the main thread."""
        if self._main_thread_id is None:
            return False
        return threading.current_thread().ident == self._main_thread_id

    def set_max_process_per_frame(self, max_count: int) -> None:
        """
        Set the maximum number of callbacks to process per frame.

        Args:
            max_count: Maximum callbacks per frame. Use a large number for unlimited.
        """
        self._max_process_per_frame = max_count if max_count > 0 else 1000

    def clear(self) -> None:
        """Clear all pending callbacks from the queue."""
        while not self._queue.empty():
            try:
                self._queue.get_nowait()
            except queue.Empty:
                break


# Convenience functions
def get_dispatcher() -> MainThreadDispatcher:
    """Get the dispatcher instance."""
    return MainThreadDispatcher.get_instance()


def enqueue(callback: Callable[[], None]) -> None:
    """Enqueue a callback to be executed on the main thread."""
    get_dispatcher().enqueue(callback)


def process() -> int:
    """Process the queue and return the number of callbacks processed."""
    return get_dispatcher().process_queue()


__all__ = [
    "MainThreadDispatcher",
    "get_dispatcher",
    "enqueue",
    "process",
]
