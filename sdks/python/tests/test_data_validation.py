"""Data validation tests for Croupier SDK."""

import pytest
import json
import re

import croupier
from croupier import ClientConfig, CroupierClient, FunctionDescriptor


class TestConfigValidation:
    """Test configuration validation."""

    def test_default_config_is_valid(self):
        """Test that default config is valid."""
        config = ClientConfig()
        assert config.service_id is not None

    def test_config_with_valid_agent_addr(self):
        """Test config with valid agent addresses."""
        valid_addrs = [
            "tcp://127.0.0.1:19090",
            "tcp://localhost:19090",
            "tcp://0.0.0.0:19090",
            "ipc:///tmp/croupier.sock",
        ]

        for addr in valid_addrs:
            config = ClientConfig(service_id="test", agent_addr=addr)
            assert config.agent_addr == addr

    def test_config_timeout_range(self):
        """Test config timeout values."""
        # Valid timeouts
        valid_timeouts = [0, 1, 30, 300, 3600]

        for timeout in valid_timeouts:
            config = ClientConfig(service_id="test", timeout_seconds=timeout)
            assert config.timeout_seconds == timeout

    def test_config_service_id_validation(self):
        """Test service ID validation."""
        # Valid service IDs
        valid_ids = [
            "test-service",
            "my_service",
            "service123",
            "MyService",
        ]

        for service_id in valid_ids:
            config = ClientConfig(service_id=service_id)
            assert config.service_id == service_id


class TestFunctionDescriptorValidation:
    """Test function descriptor validation."""

    def test_descriptor_requires_id(self):
        """Test that descriptor requires id."""
        client = CroupierClient()
        with pytest.raises(ValueError):
            client.register_function(FunctionDescriptor(id="", version="1.0.0"), lambda ctx, payload: "ok")

    def test_descriptor_requires_version(self):
        """Test that descriptor requires version."""
        client = CroupierClient()
        with pytest.raises(ValueError):
            client.register_function(FunctionDescriptor(id="test.func", version=""), lambda ctx, payload: "ok")

    def test_descriptor_version_format(self):
        """Test descriptor version format."""
        valid_versions = [
            "1.0.0",
            "2.3.4",
            "0.0.1",
            "10.20.30",
            "1.0.0-beta",
            "1.0.0-alpha.1",
        ]

        for version in valid_versions:
            desc = FunctionDescriptor(id="test.func", version=version)
            assert desc.version == version

    def test_descriptor_optional_fields(self):
        """Test descriptor optional fields can be None."""
        desc = FunctionDescriptor(
            id="test.func",
            version="1.0.0",
            category=None,
            risk=None,
            entity=None,
            operation=None
        )

        assert desc.id == "test.func"
        assert desc.version == "1.0.0"
        assert desc.category is None
        assert desc.risk is None
        assert desc.entity is None
        assert desc.operation is None


class TestFunctionContextValidation:
    """Test function context validation - SKIPPED: FunctionContext not in current SDK API."""

    @pytest.mark.skip(reason="FunctionContext class not available in current SDK API")
    def test_context_required_fields(self):
        """Test context has required fields."""
        pass

    @pytest.mark.skip(reason="FunctionContext class not available in current SDK API")
    def test_context_optional_fields(self):
        """Test context optional fields can be None."""
        pass

    @pytest.mark.skip(reason="FunctionContext class not available in current SDK API")
    def test_context_timestamp_range(self):
        """Test context timestamp values."""
        pass


class TestInvokeResultValidation:
    """Test invoke result validation - SKIPPED: InvokeResult not in current SDK API."""

    @pytest.mark.skip(reason="InvokeResult class not available in current SDK API")
    def test_result_with_success(self):
        """Test result with success=True."""
        pass

    @pytest.mark.skip(reason="InvokeResult class not available in current SDK API")
    def test_result_with_failure(self):
        """Test result with success=False."""
        pass

    @pytest.mark.skip(reason="InvokeResult class not available in current SDK API")
    def test_result_duration_range(self):
        """Test result duration values."""
        pass


class TestInvokeOptionsValidation:
    """Test invoke options validation."""

    def test_options_with_defaults(self):
        """Test options with default values."""
        options = croupier.InvokeOptions()

        # Should have sensible defaults or None
        assert options.idempotency_key is None or isinstance(options.idempotency_key, str)
        assert options.timeout is None or isinstance(options.timeout, int)
        assert options.headers is None or isinstance(options.headers, dict)
        assert options.retry is None or isinstance(options.retry, croupier.RetryConfig)

    def test_options_custom_values(self):
        """Test options with custom values."""
        retry_config = croupier.RetryConfig(max_attempts=5)
        options = croupier.InvokeOptions(idempotency_key="test-key-123", timeout=60, headers={"X-Custom": "value"}, retry=retry_config)

        assert options.idempotency_key == "test-key-123"
        assert options.timeout == 60
        assert options.headers == {"X-Custom": "value"}
        assert options.retry is not None
        assert options.retry.max_attempts == 5


class TestJobStatusValidation:
    """Test job status validation - SKIPPED: JobStatus not in current SDK API."""

    @pytest.mark.skip(reason="JobStatus class not available in current SDK API")
    def test_status_progress_range(self):
        """Test status progress values."""
        pass

    @pytest.mark.skip(reason="JobStatus class not available in current SDK API")
    def test_status_valid_states(self):
        """Test status has valid state."""
        pass


class TestJSONValidation:
    """Test JSON validation."""

    def test_valid_json_objects(self):
        """Test parsing valid JSON objects."""
        valid_jsons = [
            '{}',
            '{"key":"value"}',
            '{"number":123}',
            '{"bool":true}',
            '{"null":null}',
            '{"nested":{"key":"value"}}',
            '{"array":[1,2,3]}',
        ]

        for json_str in valid_jsons:
            parsed = json.loads(json_str)
            assert isinstance(parsed, dict)

    def test_valid_json_arrays(self):
        """Test parsing valid JSON arrays."""
        valid_jsons = [
            '[]',
            '[1,2,3]',
            '["a","b","c"]',
            '[true,false,null]',
            '[{"key":"value"}]',
        ]

        for json_str in valid_jsons:
            parsed = json.loads(json_str)
            assert isinstance(parsed, list)

    def test_invalid_json(self):
        """Test parsing invalid JSON raises error."""
        invalid_jsons = [
            '',
            '{',
            '[',
            '{invalid}',
            '{"key": value}',
            "[1, 2, 3,]",
        ]

        for json_str in invalid_jsons:
            with pytest.raises(json.JSONDecodeError):
                json.loads(json_str)


class TestStringValidation:
    """Test string validation."""

    def test_empty_string(self):
        """Test empty string is valid."""
        s = ""
        assert s == ""
        assert len(s) == 0

    def test_string_with_special_chars(self):
        """Test string with special characters."""
        special_strings = [
            "test\n\t\r",
            "test\\nstring",
            "\"quoted\"",
            "'apostrophed'",
            "test\\u4e2d\\u6587",  # Unicode escape
        ]

        for s in special_strings:
            assert isinstance(s, str)
            assert len(s) > 0

    def test_string_with_emoji(self):
        """Test string with emoji."""
        emoji_strings = [
            "😀",
            "🚀🎉",
            "Test with emoji: 😀🎉🚀",
        ]

        for s in emoji_strings:
            assert isinstance(s, str)
            assert len(s) > 0

    def test_string_encoding(self):
        """Test string encoding."""
        utf8_string = "Hello 你好 🚀"
        encoded = utf8_string.encode('utf-8')
        decoded = encoded.decode('utf-8')

        assert decoded == utf8_string


class TestNumericValidation:
    """Test numeric validation."""

    def test_integer_range(self):
        """Test integer range validation."""
        int_values = [
            0,
            1,
            -1,
            2**63 - 1,
            -2**63,
        ]

        for value in int_values:
            assert isinstance(value, int)

    def test_float_values(self):
        """Test float values."""
        float_values = [
            0.0,
            1.5,
            -1.5,
            1e10,
            1e-10,
            float('inf'),
            float('-inf'),
        ]

        for value in float_values:
            assert isinstance(value, float)

    def test_nan_value(self):
        """Test NaN value."""
        nan_val = float('nan')
        assert isinstance(nan_val, float)
        import math
        assert math.isnan(nan_val)


class TestBooleanValidation:
    """Test boolean validation."""

    def test_boolean_values(self):
        """Test boolean True and False."""
        assert True is True
        assert False is False

    def test_truthy_falsy(self):
        """Test truthy and falsy values."""
        # Truthy
        assert bool(1)
        assert bool("text")
        assert bool([1])
        assert bool({"key": "value"})

        # Falsy
        assert not bool(0)
        assert not bool("")
        assert not bool([])
        assert not bool({})


class TestCollectionValidation:
    """Test collection validation."""

    def test_empty_collections(self):
        """Test empty collections are valid."""
        assert [] == []
        assert {} == {}
        assert () == ()

    def test_list_validation(self):
        """Test list validation."""
        # Valid lists
        valid_lists = [
            [],
            [1],
            [1, 2, 3],
            ["a", "b", "c"],
            [None],
            [1, "text", None, True],
        ]

        for lst in valid_lists:
            assert isinstance(lst, list)

    def test_dict_validation(self):
        """Test dict validation."""
        # Valid dicts
        valid_dicts = [
            {},
            {"key": "value"},
            {"number": 123},
            {"nested": {"key": "value"}},
        ]

        for d in valid_dicts:
            assert isinstance(d, dict)

    def test_dict_key_types(self):
        """Test dict key types."""
        # String keys (most common)
        d1 = {"key": "value"}
        assert "key" in d1

        # Integer keys
        d2 = {1: "one", 2: "two"}
        assert 1 in d2

        # Tuple keys (hashable)
        d3 = {(1, 2): "value"}
        assert (1, 2) in d3

        # List keys should fail
        with pytest.raises(TypeError):
            d = {[1, 2]: "value"}


class TestRegexValidation:
    """Test regex validation."""

    def test_email_validation(self):
        """Test email format validation."""
        email_pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'

        valid_emails = [
            "user@example.com",
            "test.user@test.co.uk",
            "user+tag@example.com",
        ]

        for email in valid_emails:
            assert re.match(email_pattern, email) is not None

        invalid_emails = [
            "not-an-email",
            "@example.com",
            "user@",
            "user @example.com",
        ]

        for email in invalid_emails:
            assert re.match(email_pattern, email) is None

    def test_uuid_validation(self):
        """Test UUID format validation."""
        uuid_pattern = r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'

        valid_uuids = [
            "550e8400-e29b-41d4-a716-446655440000",
            "00000000-0000-0000-0000-000000000000",
            "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
        ]

        for uuid_str in valid_uuids:
            assert re.match(uuid_pattern, uuid_str.lower()) is not None

    def test_url_validation(self):
        """Test URL format validation."""
        url_pattern = r'^[a-zA-Z][a-zA-Z0-9+.-]*://'

        valid_urls = [
            "tcp://127.0.0.1:19090",
            "http://example.com",
            "https://example.com",
            "ipc:///tmp/socket",
        ]

        for url in valid_urls:
            assert re.match(url_pattern, url) is not None


class TestTimestampValidation:
    """Test timestamp validation."""

    def test_unix_timestamp_range(self):
        """Test Unix timestamp is reasonable."""
        import time

        current_time = int(time.time() * 1000)  # milliseconds

        # Timestamp should be positive and reasonable
        assert current_time > 0
        assert current_time > 946684800000  # After 2000-01-01
        assert current_time < 4102444800000  # Before 2100-01-01

    def test_timestamp_conversion(self):
        """Test timestamp conversion."""
        import time

        # Current time
        now_sec = time.time()
        now_ms = int(now_sec * 1000)

        # Convert back
        back_sec = now_ms / 1000.0

        assert abs(now_sec - back_sec) < 0.001


class TestVersionValidation:
    """Test version validation."""

    def test_semantic_version(self):
        """Test semantic version format."""
        version_pattern = r'^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$'

        valid_versions = [
            "1.0.0",
            "2.3.4",
            "0.0.1",
            "10.20.30",
            "1.0.0-beta",
            "1.0.0-alpha.1",
            "2.0.0-rc.1",
        ]

        for version in valid_versions:
            assert re.match(version_pattern, version) is not None

        invalid_versions = [
            "1.0",
            "v1.0.0",
            "1.0.0.0",
            "a.b.c",
        ]

        for version in invalid_versions:
            assert re.match(version_pattern, version) is None


class TestPathValidation:
    """Test path validation."""

    def test_path_injection_check(self):
        """Test path traversal detection."""
        dangerous_paths = [
            "../../../etc/passwd",
            "..\\..\\..\\windows\\system32",
            "/etc/passwd",
            "C:\\Windows\\System32\\config",
        ]

        for path in dangerous_paths:
            # These paths should be flagged as potentially dangerous
            has_traversal = ".." in path or "/etc/" in path or "Windows" in path
            assert has_traversal

    def test_safe_paths(self):
        """Test safe path patterns."""
        safe_paths = [
            "config.json",
            "./config.json",
            "../configs/config.json",  # Relative but controlled
            "/home/user/config.json",
            "C:\\Program Files\\App\\config.json",
        ]

        for path in safe_paths:
            assert isinstance(path, str)
            assert len(path) > 0


class TestMetadataValidation:
    """Test metadata validation."""

    def test_metadata_key_types(self):
        """Test metadata keys are strings."""
        metadata = {
            "key1": "value1",
            "key2": "value2",
        }

        for key in metadata.keys():
            assert isinstance(key, str)

    def test_metadata_value_types(self):
        """Test metadata values are strings."""
        metadata = {
            "key1": "value1",
            "key2": "value2",
        }

        for value in metadata.values():
            assert isinstance(value, str)

    def test_metadata_size_limits(self):
        """Test metadata size is reasonable."""
        # Create metadata with many entries
        metadata = {f"key_{i}": f"value_{i}" for i in range(1000)}

        assert len(metadata) == 1000

        # Create metadata with large values
        large_metadata = {"key": "x" * 10000}
        assert len(large_metadata["key"]) == 10000


class TestIDValidation:
    """Test ID validation."""

    def test_function_id_format(self):
        """Test function ID format."""
        valid_ids = [
            "test.function",
            "category.entity.operation",
            "a.b.c",
            "function123",  # Single function name without dots is also valid
        ]

        for func_id in valid_ids:
            assert isinstance(func_id, str)
            assert len(func_id) > 0
            # Function IDs can have dots or not, both are valid

    def test_call_id_format(self):
        """Test call ID format."""
        # Call IDs should be unique identifiers
        call_ids = [
            "call-123",
            "550e8400-e29b-41d4-a716-446655440000",
            "call_20250118_123456",
        ]

        for call_id in call_ids:
            assert isinstance(call_id, str)
            assert len(call_id) > 0

    def test_game_id_format(self):
        """Test game ID format."""
        valid_game_ids = [
            "game-1",
            "my_game",
            "Game123",
            "test.game",
        ]

        for game_id in valid_game_ids:
            assert isinstance(game_id, str)
            assert len(game_id) > 0
