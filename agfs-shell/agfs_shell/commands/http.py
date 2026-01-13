"""HTTP command for making HTTP requests with persistent state."""

import json
from typing import Dict, List, Optional
from ..process import Process
from ..command_decorators import command
from . import register_command


@command()
@register_command('http')
def cmd_http(process: Process) -> int:
    """
    Make HTTP requests with persistent configuration.

    Usage:
        http set base <url>                 Set base URL
        http set header <key> <value>       Set default header
        http set timeout <duration>         Set timeout (e.g., 5s, 1000ms)
        http METHOD URL [options]           Make HTTP request

    Options:
        -H key:value    Add request header
        -j JSON         Send JSON body (sets Content-Type: application/json)
        -d DATA         Send raw body data
        -q key=value    Add query parameter
        -f              Fail (return non-zero) on non-2xx status
        -i              Show response headers
        -o var          Save response to variable
        --stdout        Output only raw response body (for binary downloads)

    Examples:
        http set base https://api.example.com
        http set header Authorization "Bearer token123"
        http GET /users
        http POST /users -j '{"name":"alice"}'
        http GET /search -q term=hello -q limit=10
        http DELETE /users/123 -f -o result
        http GET https://example.com/file.tar.gz --stdout > file.tar.gz
    """
    if not process.shell:
        process.stderr.write("http: shell context not available\n")
        return 1

    # Initialize HTTP client if not present
    if not hasattr(process.shell, 'http_client'):
        from ..http_client import HTTPClient
        process.shell.http_client = HTTPClient()

    client = process.shell.http_client

    if len(process.args) == 0:
        process.stderr.write("http: missing arguments\n")
        process.stderr.write("Usage: http METHOD URL [options] or http set <config>\n")
        return 1

    # Handle 'http set' commands
    if process.args[0] == 'set':
        return _handle_set_command(process, client)

    # Handle HTTP request
    return _handle_http_request(process, client)


def _handle_set_command(process: Process, client) -> int:
    """Handle 'http set' configuration commands."""
    if len(process.args) < 3:
        process.stderr.write("http set: missing arguments\n")
        process.stderr.write("Usage: http set base|header|timeout <args>\n")
        return 1

    subcommand = process.args[1]

    if subcommand == 'base':
        url = process.args[2]
        client.set_base_url(url)
        process.stdout.write(f"Base URL set to: {url}\n")
        return 0

    elif subcommand == 'header':
        if len(process.args) < 4:
            process.stderr.write("http set header: missing value\n")
            process.stderr.write("Usage: http set header <key> <value>\n")
            return 1
        key = process.args[2]
        value = process.args[3]
        client.set_header(key, value)
        process.stdout.write(f"Header set: {key}: {value}\n")
        return 0

    elif subcommand == 'timeout':
        timeout_str = process.args[2]
        try:
            client.set_timeout(timeout_str)
            process.stdout.write(f"Timeout set to: {timeout_str}\n")
            return 0
        except (ValueError, IndexError) as e:
            process.stderr.write(f"http set timeout: invalid timeout '{timeout_str}': {e}\n")
            return 1

    else:
        process.stderr.write(f"http set: unknown subcommand '{subcommand}'\n")
        process.stderr.write("Valid subcommands: base, header, timeout\n")
        return 1


def _handle_http_request(process: Process, client) -> int:
    """Handle HTTP request (http METHOD URL [options])."""
    method = process.args[0].upper()
    if len(process.args) < 2:
        process.stderr.write("http: missing URL\n")
        process.stderr.write("Usage: http METHOD URL [options]\n")
        return 1

    url = process.args[1]

    # Parse options
    args = process.args[2:]
    headers: Dict[str, str] = {}
    query_params: Dict[str, str] = {}
    body: Optional[bytes] = None
    fail_on_error = False
    show_headers = False
    output_var: Optional[str] = None
    stdout_only = False

    i = 0
    while i < len(args):
        arg = args[i]

        if arg == '-H' and i + 1 < len(args):
            # Parse header: key:value
            header_str = args[i + 1]
            if ':' in header_str:
                key, value = header_str.split(':', 1)
                headers[key.strip()] = value.strip()
            else:
                process.stderr.write(f"http: invalid header format '{header_str}' (expected key:value)\n")
                return 1
            i += 2

        elif arg == '-j' and i + 1 < len(args):
            # JSON body
            json_str = args[i + 1]
            try:
                # Validate JSON
                json.loads(json_str)
                body = json_str.encode('utf-8')
                headers['Content-Type'] = 'application/json'
            except json.JSONDecodeError as e:
                process.stderr.write(f"http: invalid JSON: {e}\n")
                return 1
            i += 2

        elif arg == '-d' and i + 1 < len(args):
            # Raw body data
            body = args[i + 1].encode('utf-8')
            i += 2

        elif arg == '-q' and i + 1 < len(args):
            # Query parameter: key=value
            query_str = args[i + 1]
            if '=' in query_str:
                key, value = query_str.split('=', 1)
                query_params[key] = value
            else:
                process.stderr.write(f"http: invalid query format '{query_str}' (expected key=value)\n")
                return 1
            i += 2

        elif arg == '-f':
            fail_on_error = True
            i += 1

        elif arg == '-i':
            show_headers = True
            i += 1

        elif arg == '-o' and i + 1 < len(args):
            output_var = args[i + 1]
            i += 2

        elif arg == '--stdout':
            stdout_only = True
            i += 1

        else:
            process.stderr.write(f"http: unknown option '{arg}'\n")
            return 1

    # Make the request
    try:
        response = client.request(
            method=method,
            url=url,
            headers=headers if headers else None,
            body=body,
            query_params=query_params if query_params else None,
        )

        # Handle --stdout mode (raw output for piping/downloading)
        if stdout_only:
            # Write raw bytes to stdout
            if hasattr(process.stdout, 'buffer'):
                # If stdout has a buffer attribute, write bytes directly
                process.stdout.buffer.write(response.body)
            else:
                # Otherwise write as string (for string-based streams)
                process.stdout.write(response.text)
        else:
            # Normal interactive mode
            # Show status line
            status_msg = f"HTTP {response.status_code} ({int(response.duration_ms)}ms)\n"
            process.stdout.write(status_msg)

            # Show headers if requested
            if show_headers:
                for key, value in response.headers.items():
                    process.stdout.write(f"{key}: {value}\n")
                process.stdout.write("\n")

            # Show body
            process.stdout.write(response.text)
            if response.text and not response.text.endswith('\n'):
                process.stdout.write("\n")

        # Save to variable if requested
        if output_var:
            # Create a simple dict representation
            response_dict = {
                'status': response.status_code,
                'ok': response.ok,
                'headers': response.headers,
                'body': response.text,
                'duration_ms': response.duration_ms,
            }
            # Store as JSON string in shell env
            process.shell.env[output_var] = json.dumps(response_dict)

        # Check for failure
        if fail_on_error and not response.ok:
            return 1

        return 0

    except Exception as e:
        process.stderr.write(f"http: request failed: {e}\n")
        return 1
