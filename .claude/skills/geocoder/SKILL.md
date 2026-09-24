---
name: geocoder
description: Geocodifica direcciones (sobre todo españolas) a coordenadas lat/lon usando CartoCiudad como geocodificador principal y Nominatim (OpenStreetMap) como respaldo, y devuelve un CSV con id,address,lat,lon. La entrada puede ser un CSV/texto con columnas id,address o una consulta a PostgreSQL (p. ej. la tabla events). Usa esta skill siempre que el usuario quiera obtener coordenadas, geocodificar, georreferenciar o "sacar lat/lon" de direcciones, lugares o eventos, rellenar la columna location de la base de datos, o convertir un listado de direcciones en puntos para un mapa, aunque no mencione CartoCiudad ni Nominatim.
---

# Geocoder (CartoCiudad → Nominatim)

Convierte direcciones en coordenadas. Para cada dirección se consulta primero **CartoCiudad** (servicio oficial del IGN/CNIG, muy preciso para España: resuelve portales, calles y topónimos como "Teatro Campoamor, Oviedo"). Solo si CartoCiudad no devuelve nada se prueba **Nominatim**, que tiene más cobertura de lugares y del extranjero pero es menos preciso con portales y tiene un límite estricto de 1 petición/segundo.

Toda la lógica está en `scripts/geocode.py` (solo usa la librería estándar de Python; `psycopg` únicamente para el modo base de datos). Úsalo en vez de escribir otro geocodificador: ya gestiona el formato JSONP de CartoCiudad, el ritmo de Nominatim, reintentos, caché de direcciones repetidas y separadores de CSV (`,` `;` tabulador).

## Salida

Un CSV con exactamente estas columnas, en este orden:

```
id,address,lat,lon
1,"Calle Uría 10, Oviedo",43.3626497,-5.8489449
2,dirección inventada,,
```

Si ninguna fuente encuentra la dirección, `lat` y `lon` quedan vacíos (la fila se mantiene para que el usuario vea qué falló). El progreso y un resumen por fuente (CartoCiudad / Nominatim / sin resultado) se escriben por stderr — muéstraselo al usuario al terminar.

## Modo 1: entrada CSV

La entrada debe tener cabecera con columnas `id` y `address` (si se llaman distinto, usa `--id-col` / `--address-col`).

```bash
python .claude/skills/geocoder/scripts/geocode.py --csv entrada.csv -o geocoded.csv
```

Si el usuario pega el CSV directamente en el chat, guárdalo primero en un fichero del scratchpad (o pásalo por stdin con `--csv -`). Si pega direcciones sueltas sin id, numéralas 1..N y crea el CSV tú.

## Modo 2: PostgreSQL

```bash
python .claude/skills/geocoder/scripts/geocode.py --db \
  --query "SELECT id, address FROM events WHERE address IS NOT NULL AND address <> ''" \
  -o geocoded.csv
```

- La consulta debe devolver `id, address` como dos primeras columnas. Si el usuario no especifica, la consulta por defecto es la de arriba (tabla `events` de este proyecto).
- En `events`, muchas filas tienen `address` vacío pero sí `venue` (nombre del recinto, p. ej. "TEATRO TORENO de Cangas de Narcea"). Si la consulta por defecto devuelve 0 filas, pregunta al usuario si quiere usar `coalesce(nullif(address, ''), venue)`. Los nombres de recintos son ambiguos: si no llevan municipio, Nominatim puede resolverlos en otra comunidad, así que avisa de ello en el resumen.
- Conexión: `--dsn postgresql://...`, o la variable `DATABASE_URL`, o las variables `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` (mismo convenio que `scraper/src/services/database.py`, también leídas de un `.env` en el directorio actual). Ejecuta el script desde `scraper/` si las credenciales están en `scraper/.env`.
- Necesita `psycopg`. Si no está instalado globalmente, el entorno de poetry del scraper ya lo tiene:
  ```bash
  cd scraper && poetry run python ../.claude/skills/geocoder/scripts/geocode.py --db -o ../geocoded.csv
  ```
- El script **solo lee** de la base de datos. Si el usuario quiere además guardar las coordenadas (p. ej. en `events.location`, que es `DOUBLE PRECISION[]` con `[lat, lon]`), confírmalo antes y hazlo como un paso aparte a partir del CSV generado.

## Opciones útiles

| Opción | Uso |
|---|---|
| `-o salida.csv` | Fichero de salida (por defecto stdout) |
| `--countrycodes es` | Restringe Nominatim a un país (por defecto `es`; `""` para quitar el filtro si hay direcciones extranjeras) |
| `--email tu@correo` | Email de contacto para Nominatim (recomendado por su política de uso en lotes grandes); también `NOMINATIM_EMAIL` |
| `--no-nominatim` | Solo CartoCiudad |

## Tiempos y buenas prácticas

- CartoCiudad es rápido; Nominatim añade ~1 s por cada dirección que llegue a él. Con cientos de direcciones fallidas, lanza el script en segundo plano y avisa al usuario.
- Las direcciones más completas (calle, número, municipio) dan mejores resultados. Si hay muchas filas sin resultado, revisa algunas y sugiere al usuario limpiarlas (p. ej. añadir el municipio) en lugar de reintentar a ciegas.
- CartoCiudad a veces devuelve el centro de una calle o municipio cuando no encuentra el portal exacto; es esperado y suele ser suficiente para un mapa de eventos.
