# Eventos y "Mis eventos"

Esta guía describe cómo el frontend obtiene, muestra y guarda eventos: la página principal, el calendario, el detalle de un evento, el mapa y la página "Mis eventos".

Para el login, el registro, el JWT que se envía a la API y los permisos de cada rol, ver [autenticacion.md](autenticacion.md).

## Índice

- [Modelo de datos](#modelo-de-datos)
- [Categorías](#categorías)
- [Acceso a la API](#acceso-a-la-api)
- [Páginas](#páginas)
  - [`/` Descubrir](#-descubrir)
  - [`/calendar` Calendario](#calendar-calendario)
  - [`/events/[id]` Detalle del evento](#eventsid-detalle-del-evento)
  - [`/my-events` Mis eventos](#my-events-mis-eventos)
- [Guardar eventos](#guardar-eventos)
- [Compartir](#compartir)
- [Componentes](#componentes)
- [Mapa](#mapa)
- [Fechas e idiomas](#fechas-e-idiomas)
- [Limitaciones conocidas y pendientes](#limitaciones-conocidas-y-pendientes)

## Modelo de datos

Tipos en `lib/types.ts`:

```ts
type EventCategory = "Music" | "Theater" | "Parties" | "Sports" | "Food" | "Art" | "Wellness";

interface Event {
  id: string;
  title: string;
  description: string;
  category: EventCategory;
  date: string;       // "YYYY-MM-DD", calculado por la API a partir de la hora de inicio
  startTime: string;  // "HH:MM"
  endTime: string;    // "HH:MM"
  venue: string;
  address: string;
  price: number;      // 0 = gratis
  imageUrl?: string;
  artistName?: string;
  perks?: string[];   // "Qué incluye"
  lat?: number;
  lon?: number;
}

interface ApiEventsResponse { events: Event[]; total: number }

// Eventos guardados
interface SavedEvent extends Event { savedAt: string }
interface UserSavedEvents { userId: string; savedEvents: SavedEvent[] }
```

- El tipo `Event` ya no tiene `isSaved`. La API de Go todavía devuelve ese campo (columna `is_saved` de `events`), pero el frontend lo ignora: si un evento está guardado se pregunta por usuario (ver [Guardar eventos](#guardar-eventos)).
- No hay ningún campo con la URL de venta de entradas.

## Categorías

`lib/categories.ts` es la única fuente de los colores de categoría:

- `CATEGORY_STYLES`: para cada categoría, `badge` (clases Tailwind de fondo claro y texto) y `color` (hex sólido para las etiquetas del calendario, los marcadores del mapa y las leyendas).
- `CATEGORIES`: la lista de categorías, en el orden de `CATEGORY_STYLES`. La usan las leyendas de `/calendar` y `MapView` y el selector de `/admin`.
- `categoryBadgeClass(category)` y `categoryColor(category)`: aceptan cualquier cadena; si la API devuelve una categoría desconocida, usan un gris de reserva.

Las categorías se muestran traducidas con `t("categories.<Categoría>")`. `"All"` es un valor especial de los filtros, no una categoría.

## Acceso a la API

Todas las llamadas usan el cliente axios de `lib/api/api.ts` (base `${NEXT_PUBLIC_API_URL}/api`). Los errores se convierten en `ApiError` con `status` y `message`. Desde el navegador, cada petición lleva el JWT del usuario si hay sesión; desde el servidor, el token se pasa como argumento (ver [autenticacion.md](autenticacion.md#jwt-para-la-api-de-go)).

### Eventos (`lib/api/events.ts`)

| Función | Petición | Uso |
|---------|----------|-----|
| `getEvents(params?)` | `GET /events?date=&from=&to=&category=` | Listado. Ver los filtros abajo. |
| `getEvent(id)` | `GET /events/{id}` | Detalle. |
| `createEvent(event)` | `POST /events` | Panel de admin. Requiere rol `admin` o `publisher`. |
| `updateEvent(id, event)` | `PUT /events/{id}` | Panel de admin. Requiere rol `admin` o `publisher`. |
| `deleteEvent(id)` | `DELETE /events/{id}` | Panel de admin. Requiere rol `admin` o `publisher`. |

Filtros de `getEvents` (`EventsQuery`), todos opcionales y combinables:

| Parámetro | Formato | Efecto |
|-----------|---------|--------|
| `date` | `YYYY-MM-DD` | Eventos que empiezan ese día. |
| `from` / `to` | `YYYY-MM-DD` | Rango de fechas de inicio, **ambos incluidos**. |
| `category` | Nombre de categoría | Solo esa categoría. `"All"` no se envía a la API. |

La API devuelve `400` si una fecha no tiene el formato `YYYY-MM-DD` o si `from` es posterior a `to`, y ordena los resultados por hora de inicio. Al crear o editar, exige `title`, `date` (`YYYY-MM-DD`), `startTime` y `endTime` (`HH:MM`).

### Eventos guardados (`lib/api/userSavedEvents.tsx`)

Todas actúan sobre el usuario del token: no reciben el id de usuario.

| Función | Petición | Devuelve |
|---------|----------|----------|
| `getSavedEvents(token?)` | `GET /savedEvents` | `UserSavedEvents`: cada evento completo más `savedAt`. |
| `isEventSaved(eventId, token?)` | `GET /savedEvents/{eventId}` | `true` o `false` (la API responde `{ "saved": bool }`). Un `401` (sin sesión) cuenta como no guardado; el resto de errores se lanzan. |
| `createSavedEvent(eventId)` | `POST /savedEvents` con `{ eventId }` | La fila creada. La API responde `409` si ya estaba guardado y `404` si el evento no existe. |
| `deleteSavedEvent(eventId)` | `DELETE /savedEvents/{eventId}` | — |

`token` solo hace falta en Server Components, que lo obtienen con `getServerApiToken()`. En el navegador lo añade el interceptor.

### Carga en componentes cliente (`lib/hooks/useEvents.ts`)

`useEvents(query)` llama a `getEvents(query)` y vuelve a pedir los datos cada vez que cambia la consulta. Devuelve `{ events, loading, error, retry }`:

- `loading` se calcula comparando la consulta del último resultado con la actual, así que una respuesta tardía de una consulta anterior nunca se muestra como la actual.
- `retry()` repite la misma consulta (lo usa el botón "Reintentar" de `LoadError`).

## Páginas

### `/` Descubrir

`app/page.tsx`, componente cliente.

- Hace dos consultas con `useEvents`, ambas filtradas por categoría en la API:
  - los eventos del **día seleccionado** (`date`), que son los que se muestran;
  - los del **mes visible** en el mini calendario (`from`/`to` con `monthRange`), solo para marcar con un punto los días que tienen eventos.
- **Columna izquierda** (solo en escritorio, `lg`):
  - `CalendarWidget`: mini calendario. El mes visible lo controla la página (`calendarMonth`) para poder cargar sus eventos.
  - `SwitchMapWidget`: alterna entre "Lista de eventos" y "Mapa".
  - En modo mapa, `CompactEventList` muestra los eventos del día; al seleccionar uno se resalta en el mapa.
- **Columna derecha:**
  - Cabecera con la fecha seleccionada (`formatLongDate`) y el número de eventos.
  - En modo lista: `EventCard` compactas. En modo mapa (escritorio): `MapView` con los eventos del día.
  - Estados: cargando, error (`LoadError` con "Reintentar") y vacío ("No hay eventos este día").
  - En móvil, al final hay un mapa con los eventos del día, que comparte la selección con el resto de la página.
- Estado: `selectedDate` (hoy por defecto), `calendarMonth` (mes actual), `selectedCategory` (`"All"`), `showMap`, `selectedEventId`.

### `/calendar` Calendario

`app/calendar/page.tsx`, componente cliente.

- Pestañas superiores: Calendario (activa), Descubrir (`/`) y "Mis entradas" (`/my-events`).
- `CategoryFilter` y navegación entre meses. Empieza en el **mes actual**.
- Carga solo los eventos del mes visible (`from`/`to`), filtrados por categoría en la API. Si falla, muestra `LoadError` en lugar de la cuadrícula o el mapa.
- Cuadrícula mensual que empieza en domingo. Cada día muestra hasta 2 eventos (ordenados por hora de inicio) como etiquetas del color de su categoría que enlazan al detalle, y "+N más" si hay más.
- Debajo de la cuadrícula, una leyenda con todas las categorías (`CATEGORIES`).
- El botón "Mapa" sustituye la cuadrícula por `MapView` con los eventos del mes.

### `/events/[id]` Detalle del evento

`app/events/[id]/page.tsx`, **Server Component**.

- Carga el evento con `getEvent(id)`. Si falla, muestra `notFound()` (404).
- Obtiene el JWT con `getServerApiToken()` y, en paralelo:
  - carga los eventos de la misma categoría (`getEvents({ category })`) y muestra hasta 3 (sin contar el actual) como "Más eventos de …";
  - consulta si el evento está guardado con `isEventSaved(id, token)`.
- Después consulta, también con `isEventSaved`, el estado de cada evento relacionado, para que sus tarjetas muestren el botón de guardar.
- Sin sesión no se hace ninguna consulta de guardados: todo cuenta como no guardado. Si una consulta falla, ese evento se muestra como no guardado y la página no se rompe.
- Contenido:
  - Imagen de cabecera (`EventImage`, con un degradado de fondo si no hay imagen), `ShareButton`, botón de guardar (icono) y la categoría.
  - Título, artista, descripción, fecha, horario, lugar y dirección, y la lista "Qué incluye" (`perks`).
  - Barra lateral con precio ("Gratis" si `price === 0`, "por persona" si no), una nota informativa y el botón "Guardar para después". No hay botón de comprar entradas porque el modelo no tiene URL de venta: la nota dice que las entradas las gestiona el organizador o el lugar (`event.ticketsInfo`) o, si es gratis, que se consulte si hace falta inscribirse (`event.freeEntryInfo`).
- **Enlace "Volver":** por defecto vuelve a `/`. Con `?from=my-events` vuelve a `/my-events` ("Volver a mis eventos"). Los valores válidos están en la lista blanca `BACK_LINKS` para evitar redirecciones arbitrarias. Para añadir otro origen, se añade una entrada a `BACK_LINKS` y se enlaza con `?from=<clave>`.

### `/my-events` Mis eventos

`app/my-events/page.tsx` (Server Component) y `components/MyEventsView.tsx` (cliente).

1. La página obtiene el JWT con `getServerApiToken()`. Sin sesión, redirige a `/login?redirect=/my-events`.
2. Pide los eventos guardados con `getSavedEvents(token)`.
3. `groupUpcomingByDay()`:
   - descarta los eventos que ya han terminado con `hasEventEnded` (ver [Fechas e idiomas](#fechas-e-idiomas));
   - ordena por fecha y hora de inicio;
   - agrupa por día en `EventDay[]` (`{ date, events }`).
4. Tres estados:
   - Error al cargar → "No se han podido cargar tus eventos guardados".
   - Lista vacía → icono, "No tienes eventos guardados próximos" y enlace a `/`.
   - Con eventos → `MyEventsView`.

`MyEventsView`:

- A la izquierda, los eventos agrupados por día, con la fecha larga y el número de eventos de cada día.
- A la derecha, un mapa con **todos** los eventos próximos guardados (fijo al hacer scroll en escritorio; encima de la lista en móvil).
- Al pulsar una tarjeta se selecciona y se resalta su marcador en el mapa, y al revés. Volver a pulsarla la deselecciona.
- El botón "+" de cada tarjeta abre el detalle con `?from=my-events`, para que "Volver" regrese aquí.

## Guardar eventos

`components/SaveEventButton.tsx`, componente cliente.

- Props: `eventId`, `saved` (estado actual, calculado en el servidor) y `variant`:
  - `"button"` (por defecto): botón ancho de la barra lateral ("Guardar para después" / "Guardado").
  - `"icon"`: botón redondo con un marcador, sobre la imagen de cabecera del detalle y de las tarjetas `EventCard`.
- Al pulsarlo llama a `createSavedEvent` o `deleteSavedEvent` y después hace `router.refresh()` dentro de una transición. Así el Server Component vuelve a calcular `saved` y **todos** los botones de la página se actualizan a la vez.
- El botón queda deshabilitado mientras se guarda y mientras se refresca la página (`busy`).
- Si la API responde `401` (sin sesión o caducada), redirige a `/login?redirect=<ruta actual>`. Cualquier otro error se relanza y el estado no cambia; no se muestra ningún mensaje.
- `EventCard` (formato con imagen y `detailed`) muestra este botón solo si recibe la prop `saved`. El padre calcula ese valor; hoy solo lo hace `/events/[id]` para los eventos relacionados.

## Compartir

`components/ShareButton.tsx`, componente cliente. Prop: `title`.

- Si el navegador tiene la Web Share API (`navigator.share`, sobre todo en móvil), abre el panel nativo para compartir con el título y la URL actual. Si el usuario lo cierra, no hace nada más.
- Si no existe o falla por otro motivo, copia la URL al portapapeles y muestra durante 2 segundos "Enlace copiado" (o "No se ha podido copiar el enlace" si falla). El aviso está en una región `aria-live`.

## Componentes

| Componente | Tipo | Descripción |
|------------|------|-------------|
| `EventCard` | Cliente | Tarjeta de evento con dos formatos (ver abajo). Los colores salen de `lib/categories.ts`. |
| `EventImage` | Cliente | `<img>` para imágenes remotas (ver abajo). |
| `CalendarWidget` | Cliente | Mini calendario mensual. Props: `selectedDate`, `onSelect`, `eventDates` (`Set` de fechas `YYYY-MM-DD` con eventos), `month` y `onMonthChange` (el mes visible lo controla el padre). Resalta el día seleccionado y el de hoy. |
| `CategoryFilter` | Cliente | Botones de categoría con icono, incluido `"All"`. Props: `selected`, `onChange`. |
| `CompactEventList` | Cliente | Lista seleccionable (título, categoría, hora y lugar) para el modo mapa de `/`. |
| `SwitchMapWidget` | Cliente | Conmutador Lista / Mapa. |
| `LoadError` | Cliente | Estado de error al cargar eventos ("No se han podido cargar los eventos"), con botón "Reintentar" si recibe `onRetry`. |
| `MapView` | Cliente | Envoltorio del mapa: crea los marcadores, muestra el evento seleccionado y la leyenda. |
| `EventsMap` | Cliente, sin SSR | Mapa de Leaflet (ver [Mapa](#mapa)). |
| `MyEventsView` | Cliente | Vista de "Mis eventos" (lista por días y mapa). |
| `SaveEventButton` | Cliente | Guardar o quitar un evento. |
| `ShareButton` | Cliente | Compartir o copiar el enlace del evento. |

### `EventCard`

| Prop | Por defecto | Efecto |
|------|-------------|--------|
| `compact` | `false` | Fila de lista sin imagen: categoría, título, horario, lugar, precio y botón "+". |
| `detailed` | `true` | Solo en el formato con imagen. `false` muestra solo imagen, título, fecha y "Detalles" (se usa en las tarjetas del mapa). |
| `approximateLocation` | `false` | Añade "Ubicación aproximada" (`common.approximateLocation`). |
| `selected` / `onSelect` | — | Solo en formato compacto. Con `onSelect`, pulsar la tarjeta la **selecciona** y solo el "+" abre el detalle. Sin `onSelect`, toda la tarjeta es un enlace. |
| `href` | `/events/{id}` | Sustituye el enlace, por ejemplo para añadir `?from=my-events`. |
| `saved` | — | Solo en el formato con imagen y `detailed`. Si se pasa, muestra `SaveEventButton` (icono) con ese estado; los clics en él no abren el evento. Sin `saved`, no hay botón de guardar. |

La fecha corta del formato con imagen (día de la semana, día y mes abreviados) se formatea en el idioma activo.

### `EventImage`

- Usa `<img>` en lugar de `next/image` porque las imágenes vienen de dominios de los scrapers que no se conocen de antemano.
- `referrerPolicy="no-referrer"`: algunas webs de origen (por ejemplo feteas.org) rechazan las peticiones que llevan un `Referer` ajeno.
- Si no hay `src` o la imagen falla, muestra `fallback`.
- Guarda un `Image` vivo por URL (`retainedImages`) para que el navegador reutilice la imagen al volver a montarla (tarjetas del mapa al pasar el ratón, detalle tras navegar) sin volver a pedirla.

## Mapa

`MapView` (envoltorio) + `EventsMap` (Leaflet con `react-leaflet`, teselas de OpenStreetMap).

Dónde se usa: modo mapa de `/` en escritorio y mapa móvil de `/` (eventos del día), modo mapa de `/calendar` (eventos del mes) y `/my-events` (eventos guardados próximos).

- `EventsMap` se carga con `next/dynamic` y `ssr: false`, porque Leaflet usa `window` al importarse.
- El mapa se centra en **Oviedo** (`43.3614, -5.8494`) con zoom 13.
- **Marcadores:**
  - Evento con `lat` y `lon` → **pin** SVG.
  - Evento sin coordenadas → se coloca en el centro de Oviedo como **círculo discontinuo** de 400 m ("ubicación aproximada").
  - Cada marcador lleva el color de su categoría (`categoryColor`). El seleccionado se pinta en naranja (`#ec5b13`), más grande y por encima de los demás. Si un marcador no tiene color, `EventsMap` usa gris.
- **Interacción:**
  - Clic en un marcador → selecciona o deselecciona el evento (`onSelect`) y abre su tarjeta.
  - Pasar el ratón → muestra la tarjeta (`EventCard` con `detailed={false}`). Se cierra 250 ms después de salir, para que dé tiempo a mover el cursor del pin a la tarjeta.
  - La tarjeta del evento seleccionado queda abierta, y la del evento bajo el cursor puede aparecer a la vez.
  - Al seleccionar un evento, el mapa se desplaza hasta él (`FlyToSelected`).
- `MapView` muestra encima del mapa una ficha con el evento seleccionado y, debajo, la leyenda con **todas** las categorías (`CATEGORIES`) y sus colores.
- El contenedor lleva `isolate` para que las capas de Leaflet (con `z-index` alto) no tapen la barra de navegación.
- Los iconos de los pines se guardan en caché por color y estado para que `react-leaflet` no los recree en cada render.

## Fechas e idiomas

- Las fechas de los eventos son cadenas `YYYY-MM-DD` con la hora local de España. Para convertirlas en `Date` se les añade `T12:00:00` (o `T00:00:00` en `EventCard`) para que se interpreten en hora local y no en UTC.
- Utilidades en `lib/dates.ts`:
  - `toISODateStr(date)` → `YYYY-MM-DD` a partir de la fecha **local** (no de `toISOString()`, que daría el día en UTC).
  - `monthRange(date)` → `{ from, to }` con el primer y el último día del mes, para los filtros de la API.
  - `startOfMonth(date)` → día 1 del mes.
  - `hasEventEnded(event, now?)` → `true` cuando ya ha pasado la hora de fin en **`Europe/Madrid`** (`EVENT_TIME_ZONE`), sea cual sea la zona horaria de la máquina. Sin hora de fin, el evento dura todo el día (`23:59`); si la hora de fin es anterior a la de inicio (por ejemplo, 22:00–02:00), termina al día siguiente.
- Formateadores en `i18n/format.ts`, que usan `Intl` con el idioma activo:
  - `formatLongDate(date, locale, withYear?)` → "Martes, 30 de septiembre".
  - `formatMonthYear(year, month, locale)` → "Septiembre de 2026".
  - `weekdayNames(locale, "short" | "narrow")` → nombres de los días empezando en domingo.
- En componentes cliente el idioma se obtiene de `useTranslation().i18n.language`. En Server Components, de `getTranslation()` (`i18n/server.ts`), que devuelve `t` y `locale`.
- Los textos están en `i18n/locales/{es,en}.json`, en las secciones `common`, `categories`, `event` y `myEvents`.

## Limitaciones conocidas y pendientes

**Datos y permisos**

- No hay URL de venta de entradas en el modelo de datos, por eso el detalle solo muestra una nota informativa. Cuando exista, se puede volver a poner un botón de compra.
- Los `publisher` pueden crear, editar y borrar eventos en la API, pero no tienen interfaz para hacerlo: `/admin` es solo para `admin`.
- El panel de admin carga **todos** los eventos (`getEvents()` sin filtros) en una sola tabla, sin paginación.
- "Más eventos de …" en el detalle pide todos los eventos de la categoría y se queda con los 3 primeros por fecha, que pueden ser eventos ya pasados.

**Fechas**

- `/` es un componente cliente, pero Next.js lo prerenderiza en el servidor: la fecha de "hoy" (`new Date()`) se calcula en el servidor y otra vez en el navegador. Si no coinciden (cerca de medianoche, o si el servidor está en otra zona horaria), puede haber un error de hidratación en la cabecera con la fecha y en el mini calendario.
