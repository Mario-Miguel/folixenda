
from enum import StrEnum

import aiohttp


class HTTPMethod(StrEnum):
    GET = "get"
    POST = "post"


_DEFAULT_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/125.0.0.0 Safari/537.36"
    ),
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "es-ES,es;q=0.9",
}


async def request(method: HTTPMethod, url: str, headers: dict | None = None)-> tuple[int, bytes]:
    merged_headers = {**_DEFAULT_HEADERS, **(headers or {})}
    async with aiohttp.ClientSession() as session:
        async with session.request(method=method.value, url=url, headers=merged_headers) as response:
            status = response.status
            return status, await response.read()