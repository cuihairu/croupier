#!/usr/bin/env python3

from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

# Keep runtime deps minimal and inline to avoid external files in build envs.
# Optional features are exposed via extras_require.
requirements = []

setup(
    name="croupier-python-sdk",
    version="0.1.0",
    author="Croupier Team",
    author_email="dev@croupier.io",
    description="Croupier Python SDK for Game Function Registration and File Transfer",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/cuihairu/croupier-sdk-python",
    project_urls={
        "Bug Tracker": "https://github.com/cuihairu/croupier-sdk-python/issues",
        "Documentation": "https://docs.croupier.io/sdk/python",
        "Source Code": "https://github.com/cuihairu/croupier-sdk-python",
    },
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Games/Entertainment",
        "Topic :: Internet :: WWW/HTTP :: Dynamic Content",
    ],
    packages=find_packages(),
    python_requires=">=3.8",
    install_requires=requirements,
    extras_require={
        "dev": [
            "pytest>=7.4.0",
            "pytest-asyncio>=0.21.0",
            "black>=23.0.0",
            "flake8>=6.0.0",
            "mypy>=1.5.0",
        ],
        "monitoring": [
            "prometheus-client>=0.17.0",
            "structlog>=23.0.0",
        ],
        "web": [
            "uvicorn>=0.23.0",
            "gunicorn>=21.0.0",
            "fastapi>=0.100.0",
        ],
    },
    entry_points={
        "console_scripts": [
            "croupier-client=croupier.cli:main",
        ],
    },
    include_package_data=True,
    package_data={
        "croupier": ["py.typed"],
    },
)
