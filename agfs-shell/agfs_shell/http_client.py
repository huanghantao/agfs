"""HTTP client with persistent state for agfs-shell."""

import json
from typing import Dict, Optional, Any
from urllib.parse import urljoin, urlencode
import time


class HTTPResponse:
    """Simplified HTTP response object."""

    def __init__(self, status_code: int, headers: Dict[str, str], body: bytes, duration_ms: float):
        self.status_code = status_code
        self.headers = headers
        self.body = body
        self.duration_ms = duration_ms

    @property
    def text(self) -> str:
        """Get response body as text."""
        return self.body.decode('utf-8', errors='replace')

    @property
    def json(self) -> Any:
        """Parse response body as JSON."""
        return json.loads(self.text)

    @property
    def ok(self) -> bool:
        """Check if status code is 2xx."""
        return 200 <= self.status_code < 300


class HTTPClient:
    """Persistent HTTP client with configurable state."""

    def __init__(self):
        self.base_url: Optional[str] = None
        self.default_headers: Dict[str, str] = {}
        self.timeout: float = 30.0  # seconds

    def set_base_url(self, url: str):
        """Set base URL for all requests."""
        self.base_url = url.rstrip('/')

    def set_header(self, key: str, value: str):
        """Set a default header."""
        self.default_headers[key] = value

    def remove_header(self, key: str):
        """Remove a default header."""
        self.default_headers.pop(key, None)

    def set_timeout(self, timeout_str: str):
        """Set timeout from string (e.g., '5s', '1000ms')."""
        timeout_str = timeout_str.lower().strip()
        if timeout_str.endswith('ms'):
            self.timeout = float(timeout_str[:-2]) / 1000
        elif timeout_str.endswith('s'):
            self.timeout = float(timeout_str[:-1])
        else:
            self.timeout = float(timeout_str)

    def request(
        self,
        method: str,
        url: str,
        headers: Optional[Dict[str, str]] = None,
        body: Optional[bytes] = None,
        query_params: Optional[Dict[str, str]] = None,
    ) -> HTTPResponse:
        """
        Make an HTTP request.

        Args:
            method: HTTP method (GET, POST, etc.)
            url: URL path or full URL
            headers: Request headers (merged with defaults)
            body: Request body
            query_params: Query parameters

        Returns:
            HTTPResponse object
        """
        try:
            import urllib.request
            import urllib.error

            # Build full URL
            if url.startswith('http://') or url.startswith('https://'):
                full_url = url
            elif self.base_url:
                full_url = urljoin(self.base_url + '/', url.lstrip('/'))
            else:
                full_url = url

            # Add query parameters
            if query_params:
                separator = '&' if '?' in full_url else '?'
                full_url += separator + urlencode(query_params)

            # Merge headers
            all_headers = {**self.default_headers}
            if headers:
                all_headers.update(headers)

            # Create request
            req = urllib.request.Request(
                full_url,
                data=body,
                headers=all_headers,
                method=method.upper()
            )

            # Make request and measure time
            start_time = time.time()
            try:
                with urllib.request.urlopen(req, timeout=self.timeout) as response:
                    duration_ms = (time.time() - start_time) * 1000
                    response_body = response.read()
                    response_headers = dict(response.headers)
                    status_code = response.status

                    return HTTPResponse(
                        status_code=status_code,
                        headers=response_headers,
                        body=response_body,
                        duration_ms=duration_ms
                    )
            except urllib.error.HTTPError as e:
                duration_ms = (time.time() - start_time) * 1000
                response_body = e.read()
                response_headers = dict(e.headers)

                return HTTPResponse(
                    status_code=e.code,
                    headers=response_headers,
                    body=response_body,
                    duration_ms=duration_ms
                )

        except Exception as e:
            # Return error as 0 status code
            raise RuntimeError(f"HTTP request failed: {e}")
