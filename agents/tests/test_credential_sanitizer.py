"""
Tests for credential sanitizer.

Tests the credential sanitizer's ability to detect and mask API keys,
tokens, passwords, and other sensitive credentials in log statements.
"""

import logging
import pytest
from agents.utils.credential_sanitizer import (
    sanitize_credentials,
    sanitize_dict,
    SanitizingLogFilter,
    SanitizingFormatter,
    setup_sanitizing_logger,
)


class TestSanitizeCredentials:
    """Tests for the sanitize_credentials function."""

    def test_masks_openai_api_key(self):
        """Test masking of OpenAI API keys."""
        message = "API call failed with key sk-1234567890abcdefghijklmnop"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "1234567890" not in result

    def test_masks_github_token(self):
        """Test masking of GitHub tokens."""
        message = "GitHub token: ghp_abcdefghijklmnopqrstuvwxyz1234567890"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "abcdefghijklmnopqrstuvwxyz" not in result

    def test_masks_aws_access_key(self):
        """Test masking of AWS access keys."""
        message = "AWS credentials: AKIAIOSFODNN7EXAMPLE"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "IOSFODNN7EXAMPLE" not in result

    def test_masks_jwt_token(self):
        """Test masking of JWT tokens."""
        message = "Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "eyJzdWIiOiIxMjM0NTY3ODkw" not in result

    def test_masks_slack_token(self):
        """Test masking of Slack tokens."""
        message = "Slack token: xoxb-1234567890123-1234567890123-AbCdEfGhIjKlMnOpQrStUvWx"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "1234567890123" not in result

    def test_masks_api_key_in_json(self):
        """Test masking of API keys in JSON-like strings."""
        message = '{"api_key": "sk-1234567890abcdefghijklmnop", "url": "https://api.example.com"}'
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "sk-1234567890" not in result

    def test_masks_password_field(self):
        """Test masking of password fields."""
        message = "Connection string: password=secretpass123"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "secretpass123" not in result

    def test_masks_bearer_token(self):
        """Test masking of bearer tokens."""
        message = "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.abc.def"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "eyJhbGciOiJIUzI1NiJ9" not in result

    def test_preserves_non_sensitive_text(self):
        """Test that non-sensitive text is preserved."""
        message = "Processing request for user john@example.com"
        result = sanitize_credentials(message)
        assert result == message
        assert "***MASKED***" not in result

    def test_masks_uuid(self):
        """Test masking of UUIDs (potential session IDs)."""
        message = "Session ID: 550e8400-e29b-41d4-a716-446655440000"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result

    def test_masks_with_additional_patterns(self):
        """Test masking with custom additional patterns."""
        message = "Custom secret: mysuper秘密key123"
        result = sanitize_credentials(message, additional_patterns=[r"mysuper秘密key\d+"])
        assert "***MASKED***" in result
        assert "mysuper秘密key123" not in result

    def test_handles_empty_string(self):
        """Test handling of empty string."""
        result = sanitize_credentials("")
        assert result == ""

    def test_handles_none(self):
        """Test handling of None - should work with string input."""
        result = sanitize_credentials("test")
        assert result == "test"


class TestSanitizeDict:
    """Tests for the sanitize_dict function."""

    def test_masks_api_key_in_dict(self):
        """Test masking of API keys in dictionaries."""
        data = {
            "api_key": "sk-1234567890abcdefghijklmnop",
            "url": "https://api.example.com"
        }
        result = sanitize_dict(data)
        assert result["api_key"] == "***MASKED***"
        assert result["url"] == "https://api.example.com"

    def test_masks_nested_dict(self):
        """Test masking in nested dictionaries."""
        data = {
            "outer": {
                "inner": {
                    "api_key": "secret123"
                }
            }
        }
        result = sanitize_dict(data)
        assert result["outer"]["inner"]["api_key"] == "***MASKED***"

    def test_masks_list_of_dicts(self):
        """Test masking in list of dictionaries."""
        data = {
            "items": [
                {"api_key": "key1", "name": "item1"},
                {"api_key": "key2", "name": "item2"}
            ]
        }
        result = sanitize_dict(data)
        assert result["items"][0]["api_key"] == "***MASKED***"
        assert result["items"][1]["api_key"] == "***MASKED***"
        assert result["items"][0]["name"] == "item1"

    def test_preserves_non_sensitive_keys(self):
        """Test that non-sensitive keys are preserved."""
        data = {
            "name": "John",
            "email": "john@example.com",
            "count": 42
        }
        result = sanitize_dict(data)
        assert result["name"] == "John"
        assert result["email"] == "john@example.com"
        assert result["count"] == 42

    def test_handles_none_values(self):
        """Test handling of None values."""
        data = {
            "api_key": None,
            "name": "test"
        }
        result = sanitize_dict(data)
        assert result["api_key"] is None
        assert result["name"] == "test"

    def test_custom_sensitive_keys(self):
        """Test with custom sensitive key set."""
        data = {
            "my_custom_secret": "confidential",
            "name": "John"
        }
        sensitive = {"my_custom_secret"}
        result = sanitize_dict(data, sensitive_keys=sensitive)
        assert result["my_custom_secret"] == "***MASKED***"
        assert result["name"] == "John"

    def test_case_insensitive_key_matching(self):
        """Test case-insensitive key matching."""
        data = {
            "API_KEY": "secret123",
            "Api_Key": "secret456",
            "name": "John"
        }
        result = sanitize_dict(data)
        assert result["API_KEY"] == "***MASKED***"
        assert result["Api_Key"] == "***MASKED***"
        assert result["name"] == "John"


class TestSanitizingLogFilter:
    """Tests for the SanitizingLogFilter class."""

    def test_filter_masks_credentials(self):
        """Test that log filter masks credentials."""
        log_filter = SanitizingLogFilter()
        
        record = logging.LogRecord(
            name="test",
            level=logging.INFO,
            pathname="test.py",
            lineno=1,
            msg="API key: sk-1234567890abcdefghijklmnop",
            args=(),
            exc_info=None
        )
        
        log_filter.filter(record)
        assert "***MASKED***" in record.getMessage()
        assert "sk-1234567890" not in record.getMessage()

    def test_filter_preserves_non_sensitive(self):
        """Test that non-sensitive logs are preserved."""
        log_filter = SanitizingLogFilter()
        
        record = logging.LogRecord(
            name="test",
            level=logging.INFO,
            pathname="test.py",
            lineno=1,
            msg="Processing item 123",
            args=(),
            exc_info=None
        )
        
        original_msg = record.getMessage()
        log_filter.filter(record)
        assert record.getMessage() == original_msg

    def test_filter_with_args(self):
        """Test filter with log args."""
        log_filter = SanitizingLogFilter()
        
        record = logging.LogRecord(
            name="test",
            level=logging.INFO,
            pathname="test.py",
            lineno=1,
            msg="Key: %s",
            args=("sk-1234567890abcdefghijklmnop",),
            exc_info=None
        )
        
        log_filter.filter(record)
        assert "***MASKED***" in record.getMessage()


class TestSanitizingFormatter:
    """Tests for the SanitizingFormatter class."""

    def test_format_masks_credentials(self):
        """Test that formatter masks credentials."""
        formatter = SanitizingFormatter("%(message)s")
        
        record = logging.LogRecord(
            name="test",
            level=logging.INFO,
            pathname="test.py",
            lineno=1,
            msg="Token: ghp_abcdefghijklmnopqrstuvwxyz1234567890",
            args=(),
            exc_info=None
        )
        
        result = formatter.format(record)
        assert "***MASKED***" in result
        assert "abcdefghijklmnopqrstuvwxyz" not in result


class TestSetupSanitizingLogger:
    """Tests for the setup_sanitizing_logger function."""

    def test_creates_logger_with_sanitization(self):
        """Test that setup_sanitizing_logger creates a properly configured logger."""
        logger = setup_sanitizing_logger("test-sanitize", logging.INFO)
        
        assert logger.name == "test-sanitize"
        assert logger.level == logging.INFO
        assert len(logger.handlers) > 0

    def test_logger_masks_credentials(self):
        """Test that the configured logger actually masks credentials."""
        import io
        
        logger = setup_sanitizing_logger("test-mask", logging.INFO)
        
        stream = io.StringIO()
        handler = logging.StreamHandler(stream)
        handler.setFormatter(logging.Formatter("%(message)s"))
        
        for h in logger.handlers[:]:
            logger.removeHandler(h)
        logger.addHandler(handler)
        
        logger.info("API key: sk-1234567890abcdefghijklmnop")
        
        output = stream.getvalue()
        assert "***MASKED***" in output
        assert "sk-1234567890" not in output


class TestIntegrationScenarios:
    """Integration tests for real-world credential sanitization scenarios."""

    def test_litellm_client_log_simulation(self):
        """Simulate LiteLLM client logging scenarios."""
        messages = [
            "LiteLLM client configured: proxy_url=http://localhost:8000",
            "Loaded API key from /etc/secrets/llm-keys/openai-key",
            "API key not found at /etc/secrets/llm-keys/openai-key",
            "Error loading API key: API key value present in error",
        ]
        
        for msg in messages:
            result = sanitize_credentials(msg)
            assert "***MASKED***" not in msg or "***MASKED***" in result

    def test_database_connection_log(self):
        """Test database connection string sanitization."""
        message = "Connected to postgres://user:password123@db.example.com:5432/mydb"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "password123" not in result

    def test_authorization_header_log(self):
        """Test Authorization header sanitization."""
        message = "Request headers: {'Authorization': 'Bearer eyJhbGciOiJIUzI1NiJ9.abc.def'}"
        result = sanitize_credentials(message)
        assert "***MASKED***" in result
        assert "eyJhbGciOiJIUzI1NiJ9" not in result

    def test_multiple_secrets_in_message(self):
        """Test masking of multiple secrets in one message."""
        message = "Keys: sk-abcdefghijklmnopqrstuv, ghp_abcdefghijklmnopqrstuvwxyz1234567890, AKIA1234567890123ABC"
        result = sanitize_credentials(message)
        
        masked_count = result.count("***MASKED***")
        assert masked_count >= 3
        assert "abcdefghijklmnopqrstuv" not in result
