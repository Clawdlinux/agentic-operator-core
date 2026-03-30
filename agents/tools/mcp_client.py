"""
MCP client wrapper with SSRF protection.

Provides secure URL validation and connection handling for MCP endpoints.
Implements SSRF protection by:
- Validating URL schemes (https only)
- Blocking localhost and internal IP ranges
- Supporting configurable URL whitelists
"""

import ipaddress
import logging
import socket
from dataclasses import dataclass
from typing import Optional, List, Set
from urllib.parse import urlparse

import aiohttp

from agents.runtime.persona import ensure_tool_allowed

logger = logging.getLogger(__name__)


class SSRFProtectionError(Exception):
    """Raised when URL violates SSRF protection rules"""
    pass


class URLValidationError(Exception):
    """Raised when URL fails validation"""
    pass


@dataclass
class SSRFConfig:
    """Configuration for SSRF protection"""
    allowed_schemes: Optional[Set[str]] = None
    blocked_ip_ranges: Optional[List[str]] = None
    allowed_hosts: Optional[Set[str]] = None
    allow_localhost: bool = False
    allow_private_ips: bool = False
    
    def __post_init__(self):
        if self.allowed_schemes is None:
            self.allowed_schemes = {"https"}
        if self.blocked_ip_ranges is None:
            self.blocked_ip_ranges = [
                "127.0.0.0/8",
                "10.0.0.0/8",
                "172.16.0.0/12",
                "192.168.0.0/16",
                "169.254.0.0/16",
                "0.0.0.0/8",
                "::1/128",
                "fc00::/7",
                "fe80::/10",
            ]


def validate_url_scheme(url: str, allowed_schemes: Optional[Set[str]] = None) -> str:
    """
    Validate URL scheme is allowed (defaults to https only).
    
    Args:
        url: URL to validate
        allowed_schemes: Set of allowed schemes (default: {"https"})
        
    Returns:
        The validated scheme
        
    Raises:
        URLValidationError: If scheme is not allowed
    """
    if allowed_schemes is None:
        allowed_schemes = {"https"}
    
    parsed = urlparse(url)
    scheme = parsed.scheme.lower()
    
    if not scheme:
        raise URLValidationError(f"URL missing scheme: {url}")
    
    if scheme not in allowed_schemes:
        raise URLValidationError(
            f"Scheme '{scheme}' not allowed. Allowed: {allowed_schemes}"
        )
    
    return scheme


def is_private_ip(ip_str: str) -> bool:
    """
    Check if IP address is in private/internal ranges.
    
    Args:
        ip_str: IP address string
        
    Returns:
        True if IP is private/internal
    """
    try:
        ip = ipaddress.ip_address(ip_str)
        return ip.is_private or ip.is_loopback or ip.is_link_local or ip.is_reserved
    except ValueError:
        return False


def resolve_hostname(hostname: str) -> Optional[str]:
    """
    Resolve hostname to IP address.
    
    Args:
        hostname: Hostname to resolve
        
    Returns:
        First resolved IP address or None
    """
    try:
        result = socket.getaddrinfo(hostname, None)
        if result:
            return str(result[0][4][0])
    except (socket.gaierror, socket.herror, OSError):
        pass
    return None


def validate_url_host(
    url: str,
    blocked_ip_ranges: Optional[List[str]] = None,
    allowed_hosts: Optional[Set[str]] = None,
    allow_localhost: bool = False,
    allow_private_ips: bool = False,
) -> str:
    """
    Validate URL host is not blocked for SSRF.
    
    Checks:
    - Hostname against whitelist (if configured)
    - Resolved IP against blocked ranges
    - Localhost blocking
    
    Args:
        url: URL to validate
        blocked_ip_ranges: CIDR ranges to block
        allowed_hosts: Whitelist of allowed hosts (None = all allowed)
        allow_localhost: Allow localhost/127.0.0.1
        allow_private_ips: Allow private IP ranges
        
    Returns:
        The validated hostname
        
    Raises:
        SSRFProtectionError: If host is blocked
    """
    if blocked_ip_ranges is None:
        blocked_ip_ranges = [
            "127.0.0.0/8",
            "10.0.0.0/8",
            "172.16.0.0/12",
            "192.168.0.0/16",
            "169.254.0.0/16",
            "0.0.0.0/8",
            "::1/128",
            "fc00::/7",
            "fe80::/10",
        ]
    
    parsed = urlparse(url)
    hostname = parsed.hostname
    
    if not hostname:
        raise SSRFProtectionError(f"URL has no hostname: {url}")
    
    hostname_lower = hostname.lower()
    
    if allowed_hosts is not None:
        if hostname_lower not in {h.lower() for h in allowed_hosts}:
            raise SSRFProtectionError(
                f"Host '{hostname}' not in whitelist"
            )
    
    localhost_indicators = {
        "localhost",
        "localhost.localdomain",
        "127.0.0.1",
        "::1",
        "0.0.0.0",
    }
    
    if not allow_localhost and hostname_lower in localhost_indicators:
        raise SSRFProtectionError(
            f"Localhost access blocked: {hostname}"
        )
    
    if hostname_lower.endswith(".local") or hostname_lower.endswith(".localhost"):
        if not allow_localhost:
            raise SSRFProtectionError(
                f"Local domain blocked: {hostname}"
            )
    
    resolved_ip = resolve_hostname(hostname)
    
    if resolved_ip:
        logger.debug(f"Resolved {hostname} -> {resolved_ip}")
        
        try:
            ip = ipaddress.ip_address(resolved_ip)
            
            for cidr in blocked_ip_ranges:
                try:
                    network = ipaddress.ip_network(cidr, strict=False)
                    if ip in network:
                        if ip.is_loopback and allow_localhost:
                            continue
                        if not allow_private_ips:
                            raise SSRFProtectionError(
                                f"IP {resolved_ip} in blocked range {cidr}"
                            )
                except ValueError:
                    continue
            
            if not allow_localhost and ip.is_loopback:
                raise SSRFProtectionError(
                    f"Loopback IP blocked: {resolved_ip}"
                )
                
        except ValueError:
            pass
    
    return hostname


def validate_url(
    url: str,
    config: Optional[SSRFConfig] = None,
) -> str:
    """
    Validate URL for SSRF protection.
    
    Combines scheme and host validation.
    
    Args:
        url: URL to validate
        config: SSRF configuration (uses defaults if None)
        
    Returns:
        The validated URL
        
    Raises:
        URLValidationError: On scheme validation failure
        SSRFProtectionError: On SSRF protection violation
    """
    if config is None:
        config = SSRFConfig()
    
    validate_url_scheme(url, config.allowed_schemes)
    
    validate_url_host(
        url,
        blocked_ip_ranges=config.blocked_ip_ranges,
        allowed_hosts=config.allowed_hosts,
        allow_localhost=config.allow_localhost,
        allow_private_ips=config.allow_private_ips,
    )
    
    return url


def is_url_allowed(
    url: str,
    allowed_hosts: Optional[Set[str]] = None,
    allowed_schemes: Optional[Set[str]] = None,
) -> bool:
    """
    Check if URL passes all validation without raising.
    
    Args:
        url: URL to check
        allowed_hosts: Whitelist of hosts
        allowed_schemes: Allowed schemes (default: https only)
        
    Returns:
        True if URL is allowed, False otherwise
    """
    try:
        validate_url(url, SSRFConfig(
            allowed_schemes=allowed_schemes or {"https"},
            allowed_hosts=allowed_hosts,
        ))
        return True
    except (URLValidationError, SSRFProtectionError):
        return False


class MCPClient:
    """
    MCP client with built-in SSRF protection.
    
    Securely connects to MCP endpoints with URL validation.
    """
    
    def __init__(
        self,
        config: Optional[SSRFConfig] = None,
        timeout: float = 30.0,
        max_retries: int = 3,
    ):
        """
        Initialize MCP client.
        
        Args:
            config: SSRF protection config
            timeout: Request timeout in seconds
            max_retries: Maximum retry attempts
        """
        self.config = config or SSRFConfig()
        self.timeout = timeout
        self.max_retries = max_retries
        self._session: Optional[aiohttp.ClientSession] = None
        
        logger.info(
            f"MCP client initialized: schemes={self.config.allowed_schemes}, "
            f"allow_localhost={self.config.allow_localhost}"
        )
    
    async def __aenter__(self):
        await self._ensure_session()
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.close()
    
    async def _ensure_session(self):
        """Ensure aiohttp session exists"""
        if self._session is None or self._session.closed:
            timeout = aiohttp.ClientTimeout(total=self.timeout)
            self._session = aiohttp.ClientSession(timeout=timeout)
    
    async def close(self):
        """Close the HTTP session"""
        if self._session and not self._session.closed:
            await self._session.close()
            self._session = None
    
    def validate_endpoint(self, url: str) -> str:
        """
        Validate an MCP endpoint URL.
        
        Args:
            url: MCP endpoint URL
            
        Returns:
            Validated URL
            
        Raises:
            URLValidationError: On validation failure
            SSRFProtectionError: On SSRF violation
        """
        return validate_url(url, self.config)
    
    async def connect(self, url: str) -> dict:
        """
        Connect to an MCP endpoint.
        
        Args:
            url: MCP endpoint URL
            
        Returns:
            Connection response
            
        Raises:
            URLValidationError: On validation failure
            SSRFProtectionError: On SSRF violation
        """
        validated_url = self.validate_endpoint(url)
        
        await self._ensure_session()
        
        logger.info(f"Connecting to MCP endpoint: {validated_url}")
        
        assert self._session is not None
        async with self._session.get(validated_url) as response:
            response.raise_for_status()
            return await response.json()
    
    async def call_tool(
        self,
        url: str,
        tool_name: str,
        arguments: dict,
    ) -> dict:
        """
        Call an MCP tool.
        
        Args:
            url: MCP endpoint URL
            tool_name: Name of the tool to call
            arguments: Tool arguments
            
        Returns:
            Tool response
            
        Raises:
            URLValidationError: On validation failure
            SSRFProtectionError: On SSRF violation
        """
        ensure_tool_allowed(tool_name)
        validated_url = self.validate_endpoint(url)
        
        await self._ensure_session()
        
        payload = {
            "tool": tool_name,
            "arguments": arguments,
        }
        
        logger.info(f"Calling MCP tool '{tool_name}' at {validated_url}")
        
        assert self._session is not None
        async with self._session.post(
            validated_url,
            json=payload,
        ) as response:
            response.raise_for_status()
            return await response.json()


_mcp_client: Optional[MCPClient] = None


async def get_mcp_client(config: Optional[SSRFConfig] = None) -> MCPClient:
    """
    Get or create MCP client singleton.
    
    Args:
        config: Optional SSRF config (used only on first call)
        
    Returns:
        MCPClient instance
    """
    global _mcp_client
    if _mcp_client is None:
        _mcp_client = MCPClient(config=config)
    return _mcp_client
