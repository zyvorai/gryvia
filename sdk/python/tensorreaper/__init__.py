"""TensorReaper Python SDK -- interact with TensorReaper GPU compute clusters.

Quick start::

    import asyncio
    from tensorreaper import TensorReaper

    async def main():
        async with TensorReaper(api_url="https://tr.example.com", token="secret") as tr:
            stats = await tr.metrics.cluster_stats()
            print(f"Total GPUs: {stats.total_gpus}")

            jobs = await tr.jobs.list()
            for job in jobs.items:
                print(f"  {job.metadata.name}: {job.status.phase}")

    asyncio.run(main())
"""

from tensorreaper.client import TensorReaper
from tensorreaper.exceptions import (
    AuthenticationError,
    AuthorizationError,
    ConfigurationError,
    NotFoundError,
    RateLimitError,
    ServerError,
    TensorReaperError,
    TimeoutError,
    ValidationError,
)

__all__ = [
    "TensorReaper",
    "TensorReaperError",
    "AuthenticationError",
    "AuthorizationError",
    "ConfigurationError",
    "NotFoundError",
    "RateLimitError",
    "ServerError",
    "TimeoutError",
    "ValidationError",
]

__version__ = "0.1.0"
