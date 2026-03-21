"""
Tests for Browserless CDP client.

Mock WebSocket connection to test scraping logic.
"""

import asyncio
import base64
import json
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from agents.tools.browserless import (
    BrowserlessClient,
    BrowserlessError,
    BrowserlessUnavailableError,
    BrowserlessTimeoutError,
)


@pytest.mark.asyncio
async def test_browserless_client_init():
    """Test client initialization"""
    client = BrowserlessClient("ws://localhost:3000", timeout=30)
    assert client.browserless_url == "ws://localhost:3000"
    assert client.timeout == 30
    print("✅ Client initialized")


@pytest.mark.asyncio
async def test_browserless_extract_title():
    """Test HTML title extraction"""
    client = BrowserlessClient()
    html = "<html><head><title>My Page</title></head></html>"
    
    title = client._extract_title(html)
    assert title == "My Page"
    print("✅ Title extraction working")


@pytest.mark.asyncio
async def test_browserless_dom_extraction():
    """Test DOM structure extraction"""
    client = BrowserlessClient()
    html = """
    <html>
        <body>
            <nav>Menu</nav>
            <button>Get Started</button>
            <span>$99/month</span>
        </body>
    </html>
    """
    
    structure = await client.extract_dom_structure(html)
    
    assert structure["has_pricing"] == True
    assert structure["has_cta"] == True
    assert structure["has_navigation"] == True
    print("✅ DOM structure extraction working")


@pytest.mark.asyncio
async def test_browserless_scrape_url_timeout():
    """Test timeout handling"""
    client = BrowserlessClient(timeout=0.1)  # Very short timeout
    
    with patch("agents.tools.browserless.websockets.connect") as mock_connect:
        mock_ws = AsyncMock()
        
        async def slow_recv():
            await asyncio.sleep(10)
        
        mock_ws.recv = slow_recv
        mock_ws.send = AsyncMock()
        mock_ws.__aenter__.return_value = mock_ws
        mock_ws.__aexit__.return_value = None
        mock_connect.return_value = mock_ws
        
        with pytest.raises(BrowserlessTimeoutError):
            await client.scrape_url("https://example.com")
    
    print("✅ Timeout handling works")


@pytest.mark.asyncio
async def test_browserless_connection_refused():
    """Test connection refused error"""
    client = BrowserlessClient("ws://invalid:9999")
    
    with pytest.raises(BrowserlessUnavailableError):
        with patch("websockets.connect") as mock_connect:
            mock_connect.side_effect = ConnectionRefusedError("Connection refused")
            await client.scrape_url("https://example.com")
    
    print("✅ Connection error handling works")


@pytest.fixture
def mock_websocket():
    """Mock WebSocket for testing"""
    mock_ws = AsyncMock()
    
    # Mock responses for CDP commands
    responses = [
        json.dumps({"result": {"frameId": "123"}}),  # navigate
        json.dumps({"result": {"value": "<html>Test</html>"}}),  # getHTML
        json.dumps({"result": {"data": base64.b64encode(b"fake-png").decode()}}),  # screenshot
    ]
    
    mock_ws.recv.side_effect = responses
    mock_ws.send.return_value = None
    
    return mock_ws


@pytest.mark.asyncio
async def test_browserless_successful_scrape():
    """Test successful scraping"""
    mock_html = "<html><body>Success</body></html>"
    mock_screenshot = b"png-bytes"
    
    with patch("agents.tools.browserless.websockets.connect") as mock_connect:
        mock_ws = AsyncMock()
        
        # Setup mock responses
        call_count = [0]
        
        async def mock_recv():
            call_count[0] += 1
            if call_count[0] == 1:
                return json.dumps({"result": {"frameId": "123"}})
            elif call_count[0] == 2:
                return json.dumps({"result": {"value": mock_html}})
            else:
                return json.dumps({"result": {"data": base64.b64encode(mock_screenshot).decode()}})
        
        mock_ws.recv = mock_recv
        mock_ws.send = AsyncMock()
        mock_ws.__aenter__.return_value = mock_ws
        mock_ws.__aexit__.return_value = None
        
        mock_connect.return_value = mock_ws
        
        client = BrowserlessClient()
        html, screenshot = await client.scrape_url("https://example.com")
        
        assert mock_html in html
        assert screenshot == mock_screenshot
        print("✅ Successful scrape test passed")


def test_browserless_error_types():
    """Test error type definitions"""
    error = BrowserlessError("Test error")
    assert isinstance(error, Exception)
    
    unavailable = BrowserlessUnavailableError("Unavailable")
    assert isinstance(unavailable, BrowserlessError)
    
    timeout = BrowserlessTimeoutError("Timeout")
    assert isinstance(timeout, BrowserlessError)
    
    print("✅ Error types defined correctly")
