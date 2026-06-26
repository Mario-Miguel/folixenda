

import asyncio
import datetime
import functools
import os
import traceback
from contextlib import asynccontextmanager
from typing import Any, AsyncIterator, Callable, Final, ParamSpec, TypeVar
from uuid import uuid4

# from psycopg.rows import dict_row

from dotenv import dotenv_values
from psycopg import AsyncConnection
from psycopg_pool import AsyncConnectionPool

## Retrieve env vars from .env file
config = dotenv_values(".env")
DB_HOST = os.getenv("DB_HOST", config.get("DB_HOST"))
DB_PORT = os.getenv("DB_PORT", config.get("DB_PORT"))
DB_USER = os.getenv("DB_USER", config.get("DB_USER"))
DB_PASSWORD = os.getenv("DB_PASSWORD", config.get("DB_PASSWORD"))
DB_NAME = os.getenv("DB_NAME", config.get("DB_NAME"))


DSN: str = f"postgresql://{DB_USER}:{DB_PASSWORD}@{DB_HOST}:{DB_PORT}/{DB_NAME}"

TIMEOUT: Final[int] = 120  # seconds
RETRY_AFTER_TIMEOUT: Final[int] = 600  # seconds
MAX_CONNECTIONS: Final[int] = 20

_pool: AsyncConnectionPool | None = None


async def get_pool() -> AsyncConnectionPool:
    global _pool
    if _pool is None or _pool.closed:
        _pool = AsyncConnectionPool(
            DSN,
            configure=config_conn,
            timeout=TIMEOUT,
            min_size=MAX_CONNECTIONS,
            open=False,
        )
        await _pool.open()
    return _pool



async def config_conn(connection: AsyncConnection):
    await connection.set_autocommit(True)


T = TypeVar("T")
P = ParamSpec("P")


def retry(retry_count=3):
    def real_decorator(func: Callable[P, T]) -> Callable[P, T]:
        @functools.wraps(func)
        async def wrapper(*args: P.args, **kwargs: P.kwargs):
            for count in range(retry_count):
                try:
                    return_values = await func(*args, **kwargs)
                    return return_values
                except Exception as error:
                    print(f"PSQL WRAPPER: Catched {error!r} on query try: {count + 1}/{retry_count}")
                    print("".join(traceback.format_exception(type(error), error, error.__traceback__)))
                    if pool := await get_pool():
                        print("PSQL WRAPPER: Start pool check")
                        await pool.check()
                        print("PSQL WRAPPER: Finish pool check")
                    if count == retry_count - 1:
                        print(f"PSQL WRAPPER: Rising error for retry_count: {retry_count}")
                        raise error

        return wrapper

    return real_decorator


@asynccontextmanager
async def get_async_connection() -> AsyncIterator[AsyncConnection]:
    pool: AsyncConnectionPool | None = None
    connection: AsyncConnection | None = None
    try:
        while True:
            try:
                pool: AsyncConnectionPool = await get_pool()
                connection: AsyncConnection = await pool.getconn()
                connection.prepare_threshold = None
                await connection.set_autocommit(True)
                break
            except Exception as e:
                print(f"{e!r}")
                await asyncio.sleep(RETRY_AFTER_TIMEOUT)
        yield connection
    finally:
        if connection and pool:
            try:
                await pool.putconn(connection)
            except RuntimeError as e:
                print(f"Failed returning connection to the pool: {e!r}")


def _pg_array_literal(items: list[str]) -> str:
    """Serialize a list of strings as a PostgreSQL array literal (e.g. '{a,b}')."""
    if not items:
        return "{}"
    escaped = ('"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"' for s in items)
    return "{" + ",".join(escaped) + "}"


@retry()
async def import_events(events: list[dict], chunk_size: int = 2000):
    dedup: dict[tuple, dict[str, Any]] = {}
    for r in events:
        key = (r.get("title", ""), r.get("date", ""))
        dedup[key] = r

    rows = list(dedup.values())
    total = len(rows)
    if not rows:
        return 0

    # perks is passed as text[] of array literals and cast back to text[] per row in the SELECT.
    # This avoids psycopg3 failing to infer the element type of empty list[list[str]] parameters.
    query = """
    INSERT INTO events (id, title, description, category, date, start_time, end_time, price, venue, address, image_url, artist_name, perks)
    SELECT id, title, description, category, date, start_time, end_time, price, venue, address, image_url, artist_name, perks::text[]
    FROM unnest(
        %(id)s::text[],
        %(title)s::text[],
        %(description)s::text[],
        %(category)s::text[],
        %(date)s::date[],
        %(start_time)s::timestamp[],
        %(end_time)s::timestamp[],
        %(price)s::double precision[],
        %(venue)s::text[],
        %(address)s::text[],
        %(image_url)s::text[],
        %(artist_name)s::text[],
        %(perks)s::text[]
    ) AS t(id, title, description, category, date, start_time, end_time, price, venue, address, image_url, artist_name, perks)
    ON CONFLICT (id) DO NOTHING
    """

    processed = 0

    for i in range(0, total, chunk_size):
        chunk = rows[i : i + chunk_size]

        id_arr: list[str] = []
        title_arr: list[str] = []
        description_arr: list[str] = []
        category_arr: list[str] = []
        date_arr: list[datetime.date] = []
        start_time_arr: list[datetime.datetime] = []
        end_time_arr: list[datetime.datetime] = []
        price_arr: list[float] = []
        venue_arr: list[str] = []
        address_arr: list[str] = []
        image_url_arr: list[str] = []
        artist_name_arr: list[str] = []
        perks_arr: list[str] = []

        for r in chunk:
            date_val = datetime.date.fromisoformat(r["date"])
            id_arr.append(str(uuid4()))
            title_arr.append(r["title"])
            description_arr.append(r.get("description"))
            category_arr.append(r.get("category"))
            date_arr.append(date_val)
            start_time_arr.append(
                datetime.datetime.combine(date_val, datetime.time.fromisoformat(r["start_time"])) if r.get("start_time") else None
            )
            end_time_arr.append(
                datetime.datetime.combine(date_val, datetime.time.fromisoformat(r["end_time"]))if r.get("end_time") else None
            )
            price_arr.append(float(r.get("price", 0)))
            venue_arr.append(r.get("venue"))
            address_arr.append(r.get("address"))
            image_url_arr.append(r.get("image_url"))
            artist_name_arr.append(r.get("artist_name"))
            perks_arr.append(_pg_array_literal(r.get("perks") or []))

        params = {
            "id": id_arr,
            "title": title_arr,
            "description": description_arr,
            "category": category_arr,
            "date": date_arr,
            "start_time": start_time_arr,
            "end_time": end_time_arr,
            "price": price_arr,
            "venue": venue_arr,
            "address": address_arr,
            "image_url": image_url_arr,
            "artist_name": artist_name_arr,
            "perks": perks_arr,
        }

        async with get_async_connection() as aconn, aconn.cursor() as cur:
            await cur.execute(query, params)

        processed += len(chunk)

    return processed
