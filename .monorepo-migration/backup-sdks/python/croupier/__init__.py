"""
Croupier Python SDK

A powerful SDK for registering game functions with Croupier's distributed GM backend system.
Supports hot reload, automatic reconnection, and seamless integration with Python web frameworks.
"""

__version__ = "1.0.0"
__author__ = "Croupier Team"
__email__ = "dev@croupier.io"

from .hotreload_client import (
    HotReloadableClient,
    HotReloadConfig,
    HotReloadMetrics,
    create_hotreload_client,
    hotreload_client,
)

__all__ = [
    "HotReloadableClient",
    "HotReloadConfig",
    "HotReloadMetrics",
    "create_hotreload_client",
    "hotreload_client",
]