"""
Credential sanitizer for masking secrets in log statements.

Provides regex-based detection and masking of API keys, tokens, passwords,
and other sensitive credentials to prevent accidental logging of secrets.
"""

import logging
import re
from typing import Optional, Set, List, Pattern


SENSITIVE_KEY_PATTERNS: List[str] = [
    r'api[_-]?key',
    r'apikey',
    r'auth[_-]?token',
    r'auth_token',
    r'access[_-]?token',
    r'access_token',
    r'refresh[_-]?token',
    r'refresh_token',
    r'bearer',
    r'password',
    r'passwd',
    r'secret',
    r'private[_-]?key',
    r'private_key',
    r'credential',
    r'token',
    r'key',
]

SENSITIVE_VALUE_PATTERNS: List[str] = [
    r'sk-[a-zA-Z0-9]{20,}',
    r'sk-proj-[a-zA-Z0-9]{20,}',
    r'pk-[a-zA-Z0-9]{20,}',
    r'pk-proj-[a-zA-Z0-9]{20,}',
    r'ghp_[a-zA-Z0-9]{36}',
    r'gho_[a-zA-Z0-9]{36}',
    r'github_pat_[a-zA-Z0-9_]{22,}',
    r'xox[baprs]-[a-zA-Z0-9-]{10,}',
    r'AKIA[0-9A-Z]{16}',
    r'eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]*',
    r'[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}',
    r'Bearer\s+[a-zA-Z0-9._\-]+',
    r'://[^:]+:([^@\s]+)@',
]

MASK_PATTERN = r'\1***MASKED***\3'

_COMPILED_KEY_PATTERNS: List[Pattern] = []
_COMPILED_VALUE_PATTERNS: List[Pattern] = []


def _compile_patterns() -> None:
    """Compile regex patterns on first use."""
    global _COMPILED_KEY_PATTERNS, _COMPILED_VALUE_PATTERNS
    
    if _COMPILED_KEY_PATTERNS:
        return
    
    for pattern in SENSITIVE_KEY_PATTERNS:
        regex = rf'({pattern}\s*[=:]\s*["\']?)([^"\s,\}}\'\]]+)(["\']?)'
        _COMPILED_KEY_PATTERNS.append(re.compile(regex, re.IGNORECASE))
    
    for pattern in SENSITIVE_VALUE_PATTERNS:
        _COMPILED_VALUE_PATTERNS.append(re.compile(f'({pattern})'))


def sanitize_credentials(message: str, additional_patterns: Optional[List[str]] = None) -> str:
    """
    Sanitize a message by masking sensitive credentials.
    
    Args:
        message: The log message to sanitize
        additional_patterns: Optional extra regex patterns to mask
        
    Returns:
        Sanitized message with credentials masked
    """
    _compile_patterns()
    
    sanitized = message
    
    for pattern in _COMPILED_KEY_PATTERNS:
        sanitized = pattern.sub(MASK_PATTERN, sanitized)
    
    for pattern in _COMPILED_VALUE_PATTERNS:
        sanitized = pattern.sub(r'***MASKED***', sanitized)
    
    if additional_patterns:
        for pattern in additional_patterns:
            try:
                compiled = re.compile(pattern)
                sanitized = compiled.sub(r'***MASKED***', sanitized)
            except re.error:
                continue
    
    return sanitized


class SanitizingLogFilter(logging.Filter):
    """
    Log filter that sanitizes credentials in log records.
    
    Can be added to any logger to automatically mask sensitive data
    before logs are emitted.
    
    Example:
        logger = logging.getLogger('myapp')
        logger.addFilter(SanitizingLogFilter())
    """
    
    def __init__(self, additional_patterns: Optional[List[str]] = None):
        """
        Initialize the filter.
        
        Args:
            additional_patterns: Optional extra regex patterns to mask
        """
        super().__init__()
        self.additional_patterns = additional_patterns
    
    def filter(self, record: logging.LogRecord) -> bool:
        """Sanitize the log message before emitting."""
        original_msg = record.getMessage()
        sanitized = sanitize_credentials(original_msg, self.additional_patterns)
        
        if sanitized != original_msg:
            record.msg = sanitized
            record.args = ()
        
        return True


class SanitizingFormatter(logging.Formatter):
    """
    Log formatter that sanitizes credentials in formatted output.
    
    Useful when you need sanitization at the formatter level
    rather than as a filter.
    """
    
    def __init__(
        self,
        fmt: Optional[str] = None,
        datefmt: Optional[str] = None,
        additional_patterns: Optional[List[str]] = None
    ):
        """
        Initialize the formatter.
        
        Args:
            fmt: Log format string
            datefmt: Date format string
            additional_patterns: Optional extra regex patterns to mask
        """
        super().__init__(fmt=fmt, datefmt=datefmt)
        self.additional_patterns = additional_patterns
    
    def format(self, record: logging.LogRecord) -> str:
        """Format the log record with credential sanitization."""
        formatted = super().format(record)
        return sanitize_credentials(formatted, self.additional_patterns)


def setup_sanitizing_logger(
    logger_name: Optional[str] = None,
    level: int = logging.INFO,
    format_string: Optional[str] = None,
    additional_patterns: Optional[List[str]] = None
) -> logging.Logger:
    """
    Set up a logger with automatic credential sanitization.
    
    Creates or retrieves a logger configured with a sanitizing formatter
    to prevent secrets from being logged.
    
    Args:
        logger_name: Name for the logger (None for root logger)
        level: Logging level
        format_string: Custom format string
        additional_patterns: Optional extra regex patterns to mask
        
    Returns:
        Configured logger with sanitization
    """
    logger = logging.getLogger(logger_name)
    logger.setLevel(level)
    
    if not logger.handlers:
        handler = logging.StreamHandler()
        handler.setLevel(level)
        
        fmt = format_string or "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
        formatter = SanitizingFormatter(fmt, additional_patterns=additional_patterns)
        handler.setFormatter(formatter)
        logger.addHandler(handler)
    
    logger.addFilter(SanitizingLogFilter(additional_patterns))
    
    return logger


def sanitize_dict(data: dict, sensitive_keys: Optional[Set[str]] = None) -> dict:
    """
    Recursively sanitize a dictionary by masking values of sensitive keys.
    
    Args:
        data: Dictionary to sanitize
        sensitive_keys: Set of key names to mask (case-insensitive)
        
    Returns:
        New dictionary with sensitive values masked
    """
    if sensitive_keys is None:
        sensitive_keys = {
            'api_key', 'apikey', 'key', 'token', 'secret', 'password',
            'passwd', 'credential', 'auth_token', 'access_token',
            'refresh_token', 'private_key', 'bearer'
        }
    
    result = {}
    
    for key, value in data.items():
        key_lower = key.lower()
        
        if any(sensitive in key_lower for sensitive in sensitive_keys):
            if value is not None and str(value).strip():
                result[key] = '***MASKED***'
            else:
                result[key] = value
        elif isinstance(value, dict):
            result[key] = sanitize_dict(value, sensitive_keys)
        elif isinstance(value, list):
            result[key] = [
                sanitize_dict(item, sensitive_keys) if isinstance(item, dict) else item
                for item in value
            ]
        else:
            result[key] = value
    
    return result
