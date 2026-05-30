"""
Croupier SDK Transport Layer

Provides TCP-based transport for communication with Croupier Agent.
"""

from .tcp import TCPTransport

__all__ = ["TCPTransport"]
