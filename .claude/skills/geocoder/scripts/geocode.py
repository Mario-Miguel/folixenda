#!/usr/bin/env python3
"""Geocodifica direcciones con CartoCiudad y, si falla, con Nominatim.

Entrada: CSV (columnas id,address) o una consulta a PostgreSQL que devuelva id, address.
Salida:  CSV con columnas id,address,lat,lon (lat/lon vacíos si no se encontró nada).

Ejemplos:
    python geocode.py --csv direcciones.csv -o salida.csv
    cat direcciones.csv | python geocode.py --csv - > salida.csv
    python geocode.py --db --query "SELECT id, address FROM events WHERE address IS NOT NULL" -o salida.csv
"""

from __future__ import annotations

import argparse
import csv
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

CARTOCIUDAD_URL = "https://www.cartociudad.es/geocoder/api/geocoder/findJsonp"
NOMINATIM_URL = "https://nominatim.openstreetmap.org/search"
USER_AGENT = "folixenda-geocoder/1.0"
NOMINATIM_MIN_INTERVAL = 1.1  # política de uso de Nominatim: máx. 1 petición/segundo
DEFAULT_QUERY = "SELECT id, address FROM events WHERE address IS NOT NULL AND address <> ''"


def log(msg: str) -> None:
    print(msg, file=sys.stderr)


def http_get(url: str, params: dict, timeout: float, retries: int = 3) -> str | None:
    full_url = f"{url}?{urllib.parse.urlencode(params)}"
    req = urllib.request.Request(full_url, headers={"User-Agent": USER_AGENT})
    for attempt in range(retries):
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.read().decode("utf-8")
        except (urllib.error.URLError, TimeoutError) as exc:
            log(f"  aviso: fallo en {url} (intento {attempt + 1}/{retries}): {exc}")
            time.sleep(2**attempt)
    return None


def geocode_cartociudad(address: str, timeout: float) -> tuple[float, float] | None:
    body = http_get(CARTOCIUDAD_URL, {"q": address}, timeout)
    if not body:
        return None
    # La respuesta es JSONP: callback({...}) o callback([]) si no hay resultado.
    start, end = body.find("("), body.rfind(")")
    if start == -1 or end == -1:
        return None
    try:
        data = json.loads(body[start + 1 : end])
    except json.JSONDecodeError:
        return None
    if isinstance(data, list):
        data = data[0] if data else None
    if not data or data.get("lat") is None or data.get("lng") is None:
        return None
    return float(data["lat"]), float(data["lng"])


class Nominatim:
    def __init__(self, timeout: float, email: str | None, countrycodes: str | None):
        self.timeout = timeout
        self.email = email
        self.countrycodes = countrycodes
        self._last_call = 0.0

    def geocode(self, address: str) -> tuple[float, float] | None:
        wait = NOMINATIM_MIN_INTERVAL - (time.monotonic() - self._last_call)
        if wait > 0:
            time.sleep(wait)
        params = {"q": address, "format": "jsonv2", "limit": 1}
        if self.countrycodes:
            params["countrycodes"] = self.countrycodes
        if self.email:
            params["email"] = self.email
        body = http_get(NOMINATIM_URL, params, self.timeout)
        self._last_call = time.monotonic()
        if not body:
            return None
        try:
            data = json.loads(body)
        except json.JSONDecodeError:
            return None
        if not data:
            return None
        return float(data[0]["lat"]), float(data[0]["lon"])


def read_csv(path: str, id_col: str, address_col: str) -> list[tuple[str, str]]:
    f = sys.stdin if path == "-" else open(path, newline="", encoding="utf-8-sig")
    try:
        sample = f.read()
    finally:
        if f is not sys.stdin:
            f.close()
    try:
        dialect = csv.Sniffer().sniff(sample[:4096], delimiters=",;\t|")
    except csv.Error:
        dialect = csv.excel
    reader = csv.DictReader(sample.splitlines(), dialect=dialect)
    fields = {name.strip().lower(): name for name in (reader.fieldnames or [])}
    if id_col.lower() not in fields or address_col.lower() not in fields:
        sys.exit(
            f"error: el CSV debe tener columnas '{id_col}' y '{address_col}'. "
            f"Encontradas: {reader.fieldnames}"
        )
    id_key, addr_key = fields[id_col.lower()], fields[address_col.lower()]
    return [((row[id_key] or "").strip(), (row[addr_key] or "").strip()) for row in reader]


def build_dsn() -> str:
    if os.getenv("DATABASE_URL"):
        return os.environ["DATABASE_URL"]
    env = {}
    try:  # mismo convenio que scraper/src/services/database.py
        from dotenv import dotenv_values

        env = dotenv_values(".env")
    except ImportError:
        pass
    get = lambda k, d=None: os.getenv(k, env.get(k, d))  # noqa: E731
    return (
        f"postgresql://{get('DB_USER')}:{get('DB_PASSWORD')}"
        f"@{get('DB_HOST', 'localhost')}:{get('DB_PORT', '5432')}/{get('DB_NAME')}"
    )


def read_db(dsn: str, query: str) -> list[tuple[str, str]]:
    try:
        import psycopg as pg
    except ImportError:
        try:
            import psycopg2 as pg  # type: ignore[no-redef]
        except ImportError:
            sys.exit(
                "error: se necesita psycopg (o psycopg2) para leer de PostgreSQL. "
                "Instálalo con `pip install 'psycopg[binary]'` o ejecuta el script "
                "dentro del entorno de poetry del scraper."
            )
    with pg.connect(dsn) as conn:
        with conn.cursor() as cur:
            cur.execute(query)
            rows = cur.fetchall()
    if rows and len(rows[0]) < 2:
        sys.exit("error: la consulta debe devolver al menos dos columnas: id, address")
    return [(str(r[0]), (r[1] or "").strip()) for r in rows]


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    src = p.add_mutually_exclusive_group(required=True)
    src.add_argument("--csv", help="Ruta al CSV de entrada (o '-' para stdin)")
    src.add_argument("--db", action="store_true", help="Leer de PostgreSQL")
    p.add_argument("--dsn", help="Cadena de conexión postgresql://... (por defecto DATABASE_URL o DB_* de .env)")
    p.add_argument("--query", default=DEFAULT_QUERY, help="Consulta SQL que devuelve id, address")
    p.add_argument("--id-col", default="id")
    p.add_argument("--address-col", default="address")
    p.add_argument("-o", "--output", default="-", help="CSV de salida (por defecto stdout)")
    p.add_argument("--no-nominatim", action="store_true", help="Desactivar el fallback a Nominatim")
    p.add_argument("--countrycodes", default="es", help="Filtro de país para Nominatim ('' para ninguno)")
    p.add_argument("--email", default=os.getenv("NOMINATIM_EMAIL"), help="Email de contacto para Nominatim")
    p.add_argument("--timeout", type=float, default=15.0)
    args = p.parse_args()

    if args.csv:
        rows = read_csv(args.csv, args.id_col, args.address_col)
    else:
        rows = read_db(args.dsn or build_dsn(), args.query)

    nominatim = None if args.no_nominatim else Nominatim(args.timeout, args.email, args.countrycodes or None)
    cache: dict[str, tuple[tuple[float, float] | None, str]] = {}
    stats = {"cartociudad": 0, "nominatim": 0, "not_found": 0, "empty": 0}

    out = sys.stdout if args.output == "-" else open(args.output, "w", newline="", encoding="utf-8")
    try:
        writer = csv.writer(out)
        writer.writerow(["id", "address", "lat", "lon"])
        for i, (row_id, address) in enumerate(rows, 1):
            if not address:
                stats["empty"] += 1
                writer.writerow([row_id, address, "", ""])
                continue
            key = address.lower()
            if key not in cache:
                coords, source = geocode_cartociudad(address, args.timeout), "cartociudad"
                if coords is None and nominatim:
                    coords, source = nominatim.geocode(address), "nominatim"
                cache[key] = (coords, source if coords else "not_found")
            coords, source = cache[key]
            stats[source] += 1
            log(f"[{i}/{len(rows)}] {source:<11} {address}")
            writer.writerow([row_id, address, *(f"{c:.7f}" for c in coords)] if coords else [row_id, address, "", ""])
            out.flush()
    finally:
        if out is not sys.stdout:
            out.close()

    log(
        f"\nResumen: {len(rows)} filas | CartoCiudad: {stats['cartociudad']} | "
        f"Nominatim: {stats['nominatim']} | sin resultado: {stats['not_found']} | "
        f"dirección vacía: {stats['empty']}"
    )


if __name__ == "__main__":
    main()
