"""Concurrency tests for Croupier SDK."""

import threading
import time
import concurrent.futures

from croupier import ClientConfig, CroupierClient, FunctionDescriptor


class TestConcurrentClientCreation:
    """Test concurrent client creation."""

    def test_multiple_threads_create_clients(self):
        """Test creating clients from multiple threads."""
        num_threads = 10
        results = []
        exceptions = []

        def create_client(thread_id):
            try:
                config = ClientConfig(service_id=f"service-{thread_id}")
                client = CroupierClient(config)
                results.append(thread_id)
            except Exception as e:
                exceptions.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=create_client, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(results) == num_threads
        assert len(exceptions) == 0

    def test_concurrent_config_loading(self):
        """Test concurrent config loading."""
        num_threads = 20
        results = []
        exceptions = []

        def load_config(thread_id):
            try:
                config = ClientConfig(service_id=f"test-{thread_id}")
                assert config.service_id == f"test-{thread_id}"
                results.append(thread_id)
            except Exception as e:
                exceptions.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=load_config, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(results) == num_threads
        assert len(exceptions) == 0


class TestConcurrentFunctionRegistration:
    """Test concurrent function registration."""

    def test_concurrent_register_different_functions(self):
        """Test registering different functions concurrently."""
        client = CroupierClient()
        num_threads = 10
        results = []
        exceptions = []

        def register_function(thread_id):
            try:
                def handler(ctx, payload):
                    return f"response-{thread_id}"

                desc = FunctionDescriptor(
                    id=f"test.func.{thread_id}",
                    version="1.0.0"
                )
                client.register_function(desc, handler)
                results.append(thread_id)
            except Exception as e:
                exceptions.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=register_function, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(results) == num_threads
        assert len(exceptions) == 0
        assert len(client._handlers) == num_threads

    def test_concurrent_register_same_function(self):
        """Test registering same function concurrently (should fail)."""
        client = CroupierClient()
        num_threads = 5
        exceptions = []

        def register_function(thread_id):
            try:
                def handler(ctx, payload):
                    return "ok"

                desc = FunctionDescriptor(id="test.func", version="1.0.0")
                client.register_function(desc, handler)
            except Exception as e:
                exceptions.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=register_function, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        # All should succeed (SDK allows overwriting functions)
        assert len(exceptions) == 0
        # Verify function was registered
        assert "test.func" in client._handlers


class TestConcurrentDataAccess:
    """Test concurrent data access."""

    def test_concurrent_metadata_access(self):
        """Test concurrent access to metadata."""
        num_threads = 10
        num_operations = 100
        errors = []

        metadata = {}

        def access_metadata(thread_id):
            try:
                for i in range(num_operations):
                    # Read
                    value = metadata.get(f"key_{i}")
                    # Write
                    metadata[f"key_{thread_id}_{i}"] = f"value_{i}"
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=access_metadata, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0

    def test_concurrent_list_operations(self):
        """Test concurrent list operations."""
        num_threads = 10
        num_operations = 100
        shared_list = []
        errors = []

        def append_items(thread_id):
            try:
                for i in range(num_operations):
                    shared_list.append(f"item_{thread_id}_{i}")
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=append_items, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0
        assert len(shared_list) == num_threads * num_operations

    def test_concurrent_dict_operations(self):
        """Test concurrent dict operations."""
        num_threads = 10
        num_operations = 100
        shared_dict = {}
        lock = threading.Lock()
        errors = []

        def update_dict(thread_id):
            try:
                for i in range(num_operations):
                    with lock:
                        shared_dict[f"key_{thread_id}_{i}"] = i
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=update_dict, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0
        assert len(shared_dict) == num_threads * num_operations


class TestConcurrentCounter:
    """Test concurrent counter operations."""

    def test_concurrent_counter_without_lock(self):
        """Test concurrent counter without lock (race condition)."""
        counter = [0]  # Use list to make it mutable in closure
        num_threads = 10
        num_operations = 100

        def increment():
            for _ in range(num_operations):
                counter[0] += 1

        threads = []
        for _ in range(num_threads):
            t = threading.Thread(target=increment)
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        # May not equal expected due to race condition
        assert counter[0] > 0

    def test_concurrent_counter_with_lock(self):
        """Test concurrent counter with lock."""
        counter = [0]
        lock = threading.Lock()
        num_threads = 10
        num_operations = 100

        def increment():
            for _ in range(num_operations):
                with lock:
                    counter[0] += 1

        threads = []
        for _ in range(num_threads):
            t = threading.Thread(target=increment)
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert counter[0] == num_threads * num_operations


class TestThreadPoolExecutor:
    """Test with ThreadPoolExecutor."""

    def test_client_creation_with_executor(self):
        """Test creating clients using ThreadPoolExecutor."""
        num_clients = 20
        results = []

        def create_client(task_id):
            config = ClientConfig(service_id=f"service-{task_id}")
            client = CroupierClient(config)
            return task_id

        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(create_client, i) for i in range(num_clients)]
            results = [f.result() for f in concurrent.futures.as_completed(futures)]

        assert len(results) == num_clients

    def test_function_registration_with_executor(self):
        """Test function registration using ThreadPoolExecutor."""
        client = CroupierClient()
        num_functions = 10

        def register_function(func_id):
            def handler(ctx, payload):
                return f"response-{func_id}"

            desc = FunctionDescriptor(id=f"test.func.{func_id}", version="1.0.0")
            client.register_function(desc, handler)
            return func_id

        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(register_function, i) for i in range(num_functions)]
            results = [f.result() for f in concurrent.futures.as_completed(futures)]

        assert len(results) == num_functions
        assert len(client._handlers) == num_functions


class TestConcurrentHandlerExecution:
    """Test concurrent handler execution."""

    def test_concurrent_handler_calls(self):
        """Test calling handler concurrently."""
        call_count = [0]
        lock = threading.Lock()

        def handler(ctx, payload):
            with lock:
                call_count[0] += 1
            time.sleep(0.01)  # Simulate work
            return "ok"

        client = CroupierClient()
        client.register_function(
            FunctionDescriptor(id="test.func", version="1.0.0"),
            handler
        )

        num_threads = 10
        num_calls = 10
        threads = []

        def call_handler():
            for _ in range(num_calls):
                handler(None, "{}")

        for _ in range(num_threads):
            t = threading.Thread(target=call_handler)
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert call_count[0] == num_threads * num_calls


class TestConcurrentJSONProcessing:
    """Test concurrent JSON processing."""

    def test_concurrent_json_parsing(self):
        """Test parsing JSON concurrently."""
        import json

        num_threads = 10
        num_operations = 100
        errors = []

        def parse_json(thread_id):
            try:
                for i in range(num_operations):
                    data = json.loads(f'{{"key": "{thread_id}", "value": {i}}}')
                    assert data["key"] == str(thread_id)
                    assert data["value"] == i
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=parse_json, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0

    def test_concurrent_json_serialization(self):
        """Test serializing JSON concurrently."""
        import json

        num_threads = 10
        num_operations = 100
        errors = []

        def serialize_json(thread_id):
            try:
                for i in range(num_operations):
                    data = {"key": str(thread_id), "value": i}
                    json_str = json.dumps(data)
                    assert "key" in json_str
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=serialize_json, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0


class TestConcurrentResourceCleanup:
    """Test concurrent resource cleanup."""

    def test_concurrent_client_lifecycle(self):
        """Test creating and destroying clients concurrently."""
        num_operations = 50
        errors = []

        def create_destroy_client(op_id):
            try:
                config = ClientConfig(service_id=f"service-{op_id}")
                client = CroupierClient(config)
                # Client will be destroyed when function exits
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_operations):
            t = threading.Thread(target=create_destroy_client, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0

    def test_rapid_create_destroy_cycles(self):
        """Test rapid create-destroy cycles."""
        num_cycles = 100
        errors = []

        for i in range(num_cycles):
            try:
                config = ClientConfig(service_id=f"service-{i}")
                client = CroupierClient(config)
                del client  # Explicitly destroy
            except Exception as e:
                errors.append(e)

        assert len(errors) == 0


class TestConcurrentErrorHandling:
    """Test concurrent error handling."""

    def test_concurrent_exception_handling(self):
        """Test handling exceptions from multiple threads."""
        num_threads = 10
        exceptions_caught = []

        def raise_exception(thread_id):
            try:
                raise ValueError(f"Error from thread {thread_id}")
            except Exception as e:
                exceptions_caught.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=raise_exception, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(exceptions_caught) == num_threads

    def test_concurrent_error_recovery(self):
        """Test error recovery in concurrent scenario."""
        num_threads = 10
        success_count = [0]
        lock = threading.Lock()

        def attempt_operation(thread_id):
            try:
                # Simulate operation that might fail
                if thread_id % 3 == 0:
                    raise RuntimeError("Simulated failure")
                with lock:
                    success_count[0] += 1
            except Exception:
                pass  # Handle error gracefully

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=attempt_operation, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        # Some should succeed
        assert success_count[0] > 0


class TestConcurrentSharedState:
    """Test concurrent shared state management."""

    def test_shared_dict_with_lock(self):
        """Test shared dictionary with proper locking."""
        shared_dict = {}
        lock = threading.Lock()
        num_threads = 10
        num_operations = 100
        errors = []

        def update_dict(thread_id):
            try:
                for i in range(num_operations):
                    with lock:
                        key = f"thread_{thread_id}_op_{i}"
                        shared_dict[key] = i
            except Exception as e:
                errors.append(e)

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=update_dict, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(errors) == 0
        assert len(shared_dict) == num_threads * num_operations

    def test_shared_counter_with_atomic(self):
        """Test shared counter using list as atomic container."""
        counter = [0]
        num_threads = 10
        num_operations = 100

        def increment():
            for _ in range(num_operations):
                counter[0] += 1  # List item assignment is atomic in CPython

        threads = []
        for _ in range(num_threads):
            t = threading.Thread(target=increment)
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        # In CPython, this should be correct due to GIL
        assert counter[0] == num_threads * num_operations


class TestConcurrentSynchronization:
    """Test concurrent synchronization primitives."""

    def test_barrier_synchronization(self):
        """Test using barrier for synchronization."""
        num_threads = 5
        barrier = threading.Barrier(num_threads)
        results = []

        def worker(thread_id):
            # Phase 1
            results.append(f"Thread {thread_id} phase 1")
            barrier.wait()

            # Phase 2 (all threads have completed phase 1)
            results.append(f"Thread {thread_id} phase 2")
            barrier.wait()

            # Phase 3 (all threads have completed phase 2)
            results.append(f"Thread {thread_id} phase 3")

        threads = []
        for i in range(num_threads):
            t = threading.Thread(target=worker, args=(i,))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        assert len(results) == num_threads * 3

    def test_condition_variable(self):
        """Test using condition variable."""
        condition = threading.Condition()
        ready = False
        processed = [0]

        def consumer():
            nonlocal ready
            with condition:
                while not ready:
                    condition.wait()
                processed[0] += 1

        def producer():
            nonlocal ready
            with condition:
                ready = True
                condition.notify_all()

        # Start consumers
        consumers = []
        for _ in range(5):
            t = threading.Thread(target=consumer)
            consumers.append(t)
            t.start()

        time.sleep(0.1)  # Let consumers wait

        # Start producer
        producer_thread = threading.Thread(target=producer)
        producer_thread.start()

        # Wait for all
        for t in consumers:
            t.join()
        producer_thread.join()

        assert processed[0] == 5


class TestConcurrentPerformance:
    """Test concurrent performance."""

    def test_parallel_processing_speedup(self):
        """Test that parallel processing provides speedup."""
        import time

        def work(duration):
            time.sleep(duration)
            return duration

        num_tasks = 10
        task_duration = 0.01

        # Sequential
        start = time.time()
        for _ in range(num_tasks):
            work(task_duration)
        sequential_time = time.time() - start

        # Parallel
        start = time.time()
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(work, task_duration) for _ in range(num_tasks)]
            results = [f.result() for f in concurrent.futures.as_completed(futures)]
        parallel_time = time.time() - start

        # Parallel should be faster (or at least not much slower)
        assert parallel_time < sequential_time * 1.5

    def test_high_throughput_operations(self):
        """Test high throughput concurrent operations."""
        num_operations = 1000
        num_threads = 10
        operations_per_thread = num_operations // num_threads
        counter = [0]
        lock = threading.Lock()

        def perform_operations():
            for _ in range(operations_per_thread):
                with lock:
                    counter[0] += 1

        start = time.time()

        threads = []
        for _ in range(num_threads):
            t = threading.Thread(target=perform_operations)
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        elapsed = time.time() - start

        assert counter[0] == num_operations
        # Should complete reasonably quickly
        assert elapsed < 5.0
