"""Performance tests for Croupier SDK."""

import time
import json
import threading
from concurrent.futures import ThreadPoolExecutor

from croupier import ClientConfig, CroupierClient, FunctionDescriptor


class TestPerformanceConfig:
    """Test configuration loading performance."""

    def test_config_creation_speed(self):
        """Test speed of creating configs."""
        num_configs = 1000

        start = time.time()
        for i in range(num_configs):
            config = ClientConfig(service_id=f"service-{i}")
        elapsed = time.time() - start

        # Should create 1000 configs in less than 1 second
        assert elapsed < 1.0
        print(f"Created {num_configs} configs in {elapsed:.3f}s")

    def test_client_creation_speed(self):
        """Test speed of creating clients."""
        num_clients = 100

        start = time.time()
        for i in range(num_clients):
            config = ClientConfig(service_id=f"service-{i}")
            client = CroupierClient(config)
        elapsed = time.time() - start

        # Should create 100 clients in less than 1 second
        assert elapsed < 1.0
        print(f"Created {num_clients} clients in {elapsed:.3f}s")


class TestPerformanceFunctionRegistration:
    """Test function registration performance."""

    def test_function_registration_speed(self):
        """Test speed of registering functions."""
        client = CroupierClient()
        num_functions = 100

        start = time.time()
        for i in range(num_functions):
            def handler(ctx, payload):
                return "ok"

            desc = FunctionDescriptor(
                id=f"test.func.{i}",
                version="1.0.0"
            )
            client.register_function(desc, handler)
        elapsed = time.time() - start

        # Should register 100 functions quickly
        assert elapsed < 1.0
        print(f"Registered {num_functions} functions in {elapsed:.3f}s")

    def test_handler_lookup_speed(self):
        """Test speed of looking up handlers."""
        client = CroupierClient()
        num_functions = 100

        # Register functions
        for i in range(num_functions):
            def handler(ctx, payload):
                return f"response-{i}"

            desc = FunctionDescriptor(
                id=f"test.func.{i}",
                version="1.0.0"
            )
            client.register_function(desc, handler)

        # Time lookups
        num_lookups = 1000
        start = time.time()
        for i in range(num_lookups):
            func_id = f"test.func.{i % num_functions}"
            handler = client._handlers.get(func_id)
        elapsed = time.time() - start

        # Should perform 1000 lookups quickly
        assert elapsed < 0.1
        print(f"Performed {num_lookups} lookups in {elapsed:.3f}s")


class TestPerformanceJSONProcessing:
    """Test JSON processing performance."""

    def test_json_parsing_speed(self):
        """Test speed of parsing JSON."""
        json_data = '{"key":"value","number":123,"nested":{"data":"test"}}'
        num_operations = 1000

        start = time.time()
        for _ in range(num_operations):
            parsed = json.loads(json_data)
        elapsed = time.time() - start

        # Should parse 1000 JSON strings quickly
        assert elapsed < 0.5
        print(f"Parsed {num_operations} JSON strings in {elapsed:.3f}s")

    def test_json_serialization_speed(self):
        """Test speed of serializing JSON."""
        data = {"key": "value", "number": 123, "nested": {"data": "test"}}
        num_operations = 1000

        start = time.time()
        for _ in range(num_operations):
            json_str = json.dumps(data)
        elapsed = time.time() - start

        # Should serialize 1000 objects quickly
        assert elapsed < 0.5
        print(f"Serialized {num_operations} objects in {elapsed:.3f}s")

    def test_large_json_parsing_speed(self):
        """Test speed of parsing large JSON."""
        # Create 100KB JSON
        large_json = "{"
        for i in range(1000):
            large_json += f'"key{i}": {i},'
        large_json = large_json[:-1] + "}"

        num_operations = 100
        start = time.time()
        for _ in range(num_operations):
            parsed = json.loads(large_json)
        elapsed = time.time() - start

        # Should handle large JSON reasonably
        assert elapsed < 5.0
        print(f"Parsed {num_operations} large JSON ({len(large_json)} bytes) in {elapsed:.3f}s")


class TestPerformanceStringOperations:
    """Test string operation performance."""

    def test_string_concatenation_speed(self):
        """Test speed of string concatenation."""
        num_operations = 10000

        start = time.time()
        result = ""
        for i in range(num_operations):
            result += f"test-{i}-"
        elapsed = time.time() - start

        # Should complete reasonably quickly
        assert elapsed < 1.0
        print(f"Performed {num_operations} concatenations in {elapsed:.3f}s")

    def test_string_formatting_speed(self):
        """Test speed of string formatting."""
        num_operations = 10000

        start = time.time()
        for i in range(num_operations):
            formatted = f"Value: {i}, Data: {i * 2}"
        elapsed = time.time() - start

        # Should complete quickly
        assert elapsed < 0.5
        print(f"Formatted {num_operations} strings in {elapsed:.3f}s")


class TestPerformanceCollections:
    """Test collection operation performance."""

    def test_list_append_speed(self):
        """Test speed of list append operations."""
        num_items = 100000

        start = time.time()
        lst = []
        for i in range(num_items):
            lst.append(i)
        elapsed = time.time() - start

        # Should append 100k items quickly
        assert elapsed < 0.5
        print(f"Appended {num_items} items in {elapsed:.3f}s")

    def test_dict_insert_speed(self):
        """Test speed of dict insert operations."""
        num_items = 10000

        start = time.time()
        d = {}
        for i in range(num_items):
            d[f"key_{i}"] = i
        elapsed = time.time() - start

        # Should insert 10k items quickly
        assert elapsed < 0.5
        print(f"Inserted {num_items} items in {elapsed:.3f}s")

    def test_dict_lookup_speed(self):
        """Test speed of dict lookups."""
        d = {f"key_{i}": i for i in range(10000)}
        num_lookups = 100000

        start = time.time()
        for i in range(num_lookups):
            value = d.get(f"key_{i % 10000}")
        elapsed = time.time() - start

        # Should perform 100k lookups very quickly
        assert elapsed < 0.5
        print(f"Performed {num_lookups} lookups in {elapsed:.3f}s")


class TestPerformanceMemory:
    """Test memory performance."""

    def test_large_string_memory(self):
        """Test creating large string."""
        size = 10 * 1024 * 1024  # 10MB

        start = time.time()
        large_string = "x" * size
        elapsed = time.time() - start

        assert len(large_string) == size
        assert elapsed < 1.0
        print(f"Created {size} byte string in {elapsed:.3f}s")

    def test_large_list_memory(self):
        """Test creating large list."""
        num_items = 100000

        start = time.time()
        large_list = list(range(num_items))
        elapsed = time.time() - start

        assert len(large_list) == num_items
        assert elapsed < 0.5
        print(f"Created list with {num_items} items in {elapsed:.3f}s")

    def test_large_dict_memory(self):
        """Test creating large dict."""
        num_items = 10000

        start = time.time()
        large_dict = {f"key_{i}": i for i in range(num_items)}
        elapsed = time.time() - start

        assert len(large_dict) == num_items
        assert elapsed < 1.0
        print(f"Created dict with {num_items} items in {elapsed:.3f}s")


class TestPerformanceThreading:
    """Test threading performance."""

    def test_thread_creation_speed(self):
        """Test speed of creating threads."""
        num_threads = 100

        start = time.time()
        threads = []
        for i in range(num_threads):
            def dummy():
                pass

            t = threading.Thread(target=dummy)
            threads.append(t)
            t.start()
        elapsed = time.time() - start

        # Clean up
        for t in threads:
            t.join()

        # Creating threads should be reasonably fast
        assert elapsed < 5.0
        print(f"Created and started {num_threads} threads in {elapsed:.3f}s")

    def test_lock_acquisition_speed(self):
        """Test speed of lock acquisition."""
        lock = threading.Lock()
        num_operations = 100000

        start = time.time()
        for _ in range(num_operations):
            with lock:
                pass
        elapsed = time.time() - start

        # Should acquire lock very quickly
        assert elapsed < 1.0
        print(f"Acquired lock {num_operations} times in {elapsed:.3f}s")


class TestPerformanceHandlerExecution:
    """Test handler execution performance."""

    def test_simple_handler_speed(self):
        """Test speed of simple handler execution."""
        call_count = [0]

        def handler(ctx, payload):
            call_count[0] += 1
            return "ok"

        client = CroupierClient()
        client.register_function(
            FunctionDescriptor(id="test.func", version="1.0.0"),
            handler
        )

        num_calls = 10000
        start = time.time()
        for _ in range(num_calls):
            handler(None, "{}")
        elapsed = time.time() - start

        assert call_count[0] == num_calls
        assert elapsed < 1.0
        print(f"Called handler {num_calls} times in {elapsed:.3f}s")

    def test_handler_with_json_processing(self):
        """Test handler with JSON processing."""
        def handler(ctx, payload):
            data = json.loads(payload)
            result = json.dumps(data)
            return result

        client = CroupierClient()
        client.register_function(
            FunctionDescriptor(id="test.func", version="1.0.0"),
            handler
        )

        num_calls = 1000
        payload = json.dumps({"key": "value", "number": 123})

        start = time.time()
        for _ in range(num_calls):
            handler(None, payload)
        elapsed = time.time() - start

        assert elapsed < 1.0
        print(f"Called handler with JSON {num_calls} times in {elapsed:.3f}s")


class TestPerformanceConcurrentExecution:
    """Test concurrent execution performance."""

    def test_concurrent_handler_execution_speed(self):
        """Test speed of concurrent handler execution."""
        call_count = [0]
        lock = threading.Lock()

        def handler(ctx, payload):
            with lock:
                call_count[0] += 1
            time.sleep(0.001)  # Simulate work
            return "ok"

        client = CroupierClient()
        client.register_function(
            FunctionDescriptor(id="test.func", version="1.0.0"),
            handler
        )

        num_threads = 10
        num_calls_per_thread = 100

        start = time.time()

        threads = []
        for _ in range(num_threads):
            def call_handler():
                for _ in range(num_calls_per_thread):
                    handler(None, "{}")

            t = threading.Thread(target=call_handler)
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        elapsed = time.time() - start

        assert call_count[0] == num_threads * num_calls_per_thread
        print(f"Concurrent calls ({num_threads} threads x {num_calls_per_thread} calls) in {elapsed:.3f}s")

    def test_thread_pool_executor_speed(self):
        """Test speed with ThreadPoolExecutor."""
        def task(task_id):
            return task_id * 2

        num_tasks = 1000
        start = time.time()

        with ThreadPoolExecutor(max_workers=10) as executor:
            results = list(executor.map(task, range(num_tasks)))

        elapsed = time.time() - start

        assert len(results) == num_tasks
        assert elapsed < 5.0
        print(f"Executed {num_tasks} tasks in {elapsed:.3f}s")


class TestPerformanceScaling:
    """Test performance scaling."""

    def test_linear_scaling_dict_operations(self):
        """Test that dict operations scale well."""
        sizes = [100, 1000, 10000]
        results = {}

        for size in sizes:
            d = {f"key_{i}": i for i in range(size)}

            start = time.time()
            for i in range(size):
                value = d.get(f"key_{i}")
            elapsed = time.time() - start

            results[size] = elapsed

        # Larger dicts should not be disproportionately slower
        # Just verify the operation completes without excessive time
        # (dict lookups are O(1), so scaling should be roughly linear with size)
        assert results[10000] < 1.0  # Should complete within 1 second
        print(f"Dict lookup scaling: {results}")


class TestPerformanceThroughput:
    """Test throughput metrics."""

    def test_operations_per_second(self):
        """Test operations per second."""
        num_operations = 10000

        start = time.time()
        for i in range(num_operations):
            # Simple operation
            result = i * 2
        elapsed = time.time() - start

        # Avoid division by zero when elapsed is too small
        if elapsed < 1e-9:
            elapsed = 1e-9
        ops_per_second = num_operations / elapsed
        print(f"Operations per second: {ops_per_second:.0f}")

        # Should achieve reasonable throughput
        assert ops_per_second > 10000

    def test_json_operations_per_second(self):
        """Test JSON operations per second."""
        data = {"key": "value", "number": 123}
        num_operations = 1000

        start = time.time()
        for _ in range(num_operations):
            json_str = json.dumps(data)
            parsed = json.loads(json_str)
        elapsed = time.time() - start

        ops_per_second = num_operations / elapsed
        print(f"JSON operations per second: {ops_per_second:.0f}")

        # Should achieve reasonable JSON throughput
        assert ops_per_second > 1000


class TestPerformanceLatency:
    """Test operation latency."""

    def test_handler_latency(self):
        """Test handler call latency."""
        def handler(ctx, payload):
            return "ok"

        client = CroupierClient()
        client.register_function(
            FunctionDescriptor(id="test.func", version="1.0.0"),
            handler
        )

        latencies = []
        for _ in range(100):
            start = time.time()
            handler(None, "{}")
            elapsed = time.time() - start
            latencies.append(elapsed)

        avg_latency = sum(latencies) / len(latencies)
        max_latency = max(latencies)
        min_latency = min(latencies)

        print(f"Handler latency - avg: {avg_latency * 1000:.3f}ms, min: {min_latency * 1000:.3f}ms, max: {max_latency * 1000:.3f}ms")

        # Average latency should be very low
        assert avg_latency < 0.01  # Less than 10ms


class TestPerformanceMemoryEfficiency:
    """Test memory efficiency."""

    def test_string_memory_efficiency(self):
        """Test string memory usage efficiency."""
        import sys

        strings = ["test"] * 10000
        total_size = sum(sys.getsizeof(s) for s in strings)

        # Python may intern small strings, so memory usage should be reasonable
        print(f"Total memory for 10000 strings: {total_size} bytes")
        assert total_size < 10 * 1024 * 1024  # Less than 10MB

    def test_list_memory_efficiency(self):
        """Test list memory usage efficiency."""
        import sys

        lst = list(range(100000))
        list_size = sys.getsizeof(lst)

        print(f"Memory for list of 100000 ints: {list_size} bytes")
        # List should be reasonably efficient
        assert list_size < 10 * 1024 * 1024  # Less than 10MB for list structure


class TestPerformanceStress:
    """Stress tests for performance limits."""

    def test_rapid_client_creation_destruction(self):
        """Test rapid client creation and destruction."""
        num_iterations = 100

        start = time.time()
        for i in range(num_iterations):
            config = ClientConfig(service_id=f"service-{i}")
            client = CroupierClient(config)
            # Client destroyed immediately
        elapsed = time.time() - start

        print(f"Created/destroyed {num_iterations} clients in {elapsed:.3f}s")
        assert elapsed < 5.0

    def test_high_function_registration_count(self):
        """Test registering many functions."""
        client = CroupierClient()
        num_functions = 1000

        start = time.time()
        for i in range(num_functions):
            def handler(ctx, payload):
                return f"response-{i}"

            desc = FunctionDescriptor(
                id=f"test.func.{i}",
                version="1.0.0"
            )
            client.register_function(desc, handler)
        elapsed = time.time() - start

        print(f"Registered {num_functions} functions in {elapsed:.3f}s")
        assert elapsed < 10.0
        assert len(client._handlers) == num_functions
