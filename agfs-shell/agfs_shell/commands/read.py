"""
READ command - read a line from stdin and assign to variables.
"""

import sys
from ..process import Process
from ..command_decorators import command
from ..arg_parser import parse_standard_flags
from . import register_command


@command()
@register_command('read')
def cmd_read(process: Process) -> int:
    """
    Read a line from standard input and assign to variables

    Usage: read [-r] [name ...]

    Options:
        -r    Do not interpret backslash escape sequences

    The line is split into fields, and each field is assigned to the
    corresponding variable name. If there are more fields than names,
    the last name gets all remaining fields.

    Returns:
        0 on success, 1 on EOF or error
    """
    # Parse flags
    flags, args = parse_standard_flags(process.args, 'r')
    raw_mode = '-r' in flags

    # Default variable name if none provided
    var_names = args if args else ['REPLY']

    # Read a line from stdin
    try:
        # Try to read from process stdin first (for pipeline/redirected input)
        line_bytes = process.stdin.readline()

        # If stdin is empty (not in pipeline), read from real stdin
        if not line_bytes:
            if not sys.stdin.isatty():
                # Non-interactive mode, EOF
                return 1
            # Interactive mode - read from terminal
            try:
                line = input()
            except EOFError:
                return 1
        else:
            # Decode bytes from stdin
            if isinstance(line_bytes, bytes):
                line = line_bytes.decode('utf-8', errors='replace')
            else:
                line = line_bytes

            # Remove trailing newline
            if line.endswith('\n'):
                line = line[:-1]
            if line.endswith('\r'):
                line = line[:-1]
    except (EOFError, KeyboardInterrupt):
        return 1

    # Process backslash escapes unless in raw mode
    if not raw_mode:
        # Replace common escape sequences
        line = line.replace('\\n', '\n')
        line = line.replace('\\t', '\t')
        line = line.replace('\\r', '\r')
        line = line.replace('\\\\', '\\')

    # Assign to variables
    if len(var_names) == 1:
        # Single variable: assign the entire line (after stripping leading/trailing whitespace)
        if process.env is not None:
            process.env[var_names[0]] = line.strip()
    else:
        # Multiple variables: split the line into fields (by whitespace)
        fields = line.split()

        if not fields:
            # Empty line - set all variables to empty string
            for var_name in var_names:
                if process.env is not None:
                    process.env[var_name] = ''
        else:
            for i, var_name in enumerate(var_names):
                if i < len(var_names) - 1:
                    # Assign single field to each variable except the last
                    if i < len(fields):
                        if process.env is not None:
                            process.env[var_name] = fields[i]
                    else:
                        # More variables than fields - set to empty
                        if process.env is not None:
                            process.env[var_name] = ''
                else:
                    # Last variable gets all remaining fields
                    if i < len(fields):
                        remaining = ' '.join(fields[i:])
                        if process.env is not None:
                            process.env[var_name] = remaining
                    else:
                        # No fields left
                        if process.env is not None:
                            process.env[var_name] = ''

    return 0
