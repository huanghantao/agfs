"""
Command registry for agfs-shell builtin commands.

This module provides the command registration and discovery mechanism.
Each command is implemented in a separate module file under this directory.
"""

from typing import Dict, Callable, Optional
from ..process import Process

# Global command registry
_COMMANDS: Dict[str, Callable[[Process], int]] = {}


def register_command(*names: str):
    """
    Decorator to register a command function.

    Args:
        *names: One or more command names (for aliases like 'test' and '[')

    Example:
        @register_command('echo')
        def cmd_echo(process: Process) -> int:
            ...

        @register_command('test', '[')
        def cmd_test(process: Process) -> int:
            ...
    """
    def decorator(func: Callable[[Process], int]):
        for name in names:
            _COMMANDS[name] = func
        return func
    return decorator


def get_builtin(command: str) -> Optional[Callable[[Process], int]]:
    """
    Get a built-in command executor by name.

    Args:
        command: The command name to look up

    Returns:
        The command function, or None if not found
    """
    return _COMMANDS.get(command)


def load_all_commands():
    """
    Import all command modules to populate the registry.

    This function imports all command modules from this package,
    which causes their @register_command decorators to execute
    and populate the _COMMANDS registry.

    Also loads user plugins from:
    - ~/.agfs/plugins/
    - Directories specified in AGFS_PLUGIN_PATH (colon-separated)
    """
    import importlib
    import importlib.util
    import pkgutil
    import os
    import sys

    # 1. Load built-in commands
    package_dir = os.path.dirname(__file__)

    for _, module_name, _ in pkgutil.iter_modules([package_dir]):
        if module_name != 'base':  # Skip base.py as it's not a command
            try:
                importlib.import_module(f'.{module_name}', package=__name__)
            except Exception as e:
                print(f"Warning: Failed to load command module {module_name}: {e}", file=sys.stderr)

    # 2. Load user plugins
    plugin_dirs = [os.path.expanduser("~/.agfs/plugins")]

    # Add directories from AGFS_PLUGIN_PATH environment variable
    env_path = os.environ.get("AGFS_PLUGIN_PATH", "")
    if env_path:
        plugin_dirs.extend(env_path.split(os.pathsep))

    for plugin_dir in plugin_dirs:
        if plugin_dir and os.path.isdir(plugin_dir):
            _load_plugins_from_dir(plugin_dir)


def _load_plugins_from_dir(plugin_dir: str):
    """Load all .py plugin files from a directory."""
    import importlib.util
    import os
    import sys

    for filename in os.listdir(plugin_dir):
        if filename.endswith('.py') and not filename.startswith('_'):
            filepath = os.path.join(plugin_dir, filename)
            module_name = f"agfs_plugin_{filename[:-3]}"

            try:
                spec = importlib.util.spec_from_file_location(module_name, filepath)
                module = importlib.util.module_from_spec(spec)
                sys.modules[module_name] = module
                spec.loader.exec_module(module)
            except Exception as e:
                print(f"Warning: Failed to load plugin {filepath}: {e}", file=sys.stderr)


# Backward compatibility: BUILTINS dictionary
# This allows old code to use BUILTINS dict while we migrate
BUILTINS = _COMMANDS


__all__ = ['register_command', 'get_builtin', 'load_all_commands', 'BUILTINS']
