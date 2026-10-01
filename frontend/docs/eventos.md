# Eventos y "Mis eventos"

Esta guía describe cómo el frontend obtiene, muestra y guarda eventos: la página principal, el calendario, el detalle de un evento, el mapa y la página "Mis eventos".

Para el login, el registro y el JWT que se envía a la API, ver [autenticacion.md](autenticacion.md).

## Índice

- [Modelo de datos](#modelo-de-datos)
- [Acceso a la API](#acceso-a-la-api)
- [Páginas](#páginas)
  - [`/` Descubrir](#-descubrir)
  - [`/calendar` Calendario](#calendar-calendario)
  - [`/events/[id]` Detalle del evento](#eventsid-detalle-del-evento)
  - [`/my-events` Mis eventos](#my-events-mis-eventos)
- [Guardar eventos](#guardar-eventos)
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
  isSaved?: boolean;  // lo usa EventCard, pero la API no lo rellena
  lat?: number;
  lon?: number;
}

interface ApiEventsResponse { events: Event[]; total: number }

// Eventos guardados
interface SavedEvent extends Event { savedAt: string }
interface UserSavedEvents { userId: string; savedEvents: SavedEvent[] }
interface UserSavedEvent { id: string; userId: string; eventId: string }
```

Las categorías se muestran traducidas con `t("categories.<Categoría>")`. `"All"` es un valor especial de los filtros, no una categoría.

## Acceso a la API

Todas las llamadas usan el cliente axios de `lib/api/api.ts` (base `${NEXT_PUBLIC_API_URL}/api`). Los errores se convierten en `ApiError` con `status` y `message`. Desde el navegador, cada petición lleva el JWT del usuario si hay sesión (ver [autenticacion.md](autenticacion.md#jwt-para-la-api-de-go)).

### Eventos (`lib/api/events.ts`)

| Función | Petición | Uso |
|---------|----------|-----|
| `getEvents(params?)` | `GET /events?date=&category=` | Listado. Los filtros son opcionales. |
| `getEvent(id)` | `GET /events/{id}` | Detalle. |
| `createEvent(event)` | `POST /events` | Panel de admin. |
| `updateEvent(id, event)` | `PUT /events/{id}` | Panel de admin. |
| `deleteEvent(id)` | `DELETE /events/{id}` | Panel de admin. |

### Eventos guardados (`lib/api/userSavedEvents.tsx`)

| Función | Petición | Devuelve |
|---------|----------|----------|
| `getSavedEvents(userId)` | `GET /user/{userId}/savedEvents` | `UserSavedEvents`: el evento completo más `savedAt`. |
| `createSavedEvent(userId, { userId, eventId })` | `POST /user/{userId}/savedEvents` | `SavedEvent` |
| `deleteSavedEvent(userId, eventId)` | `DELETE /user/{userId}/savedEvents/{eventId}` | — |
| `isEventSaved(userId, eventId)` | Usa `getSavedEvents` | `true` si el evento está en la lista. Descarga la lista completa. |

> Por ahora todas estas llamadas usan un usuario fijo, `ConsumerUser` (`data/users.ts`, id `"2"`), no el usuario de la sesión. Ver [Limitaciones](#limitaciones-conocidas-y-pendientes).

## Páginas

### `/` Descubrir

`app/page.tsx`, componente cliente.

- Al montarse descarga **todos** los eventos con `getEvents()` y filtra en el cliente por día y categoría (`useMemo`).
- **Columna izquierda** (solo en escritorio, `lg`):
  - `CalendarWidget`: mini calendario. Los días con eventos llevan un punto debajo.
  - `SwitchMapWidget`: alterna entre "Lista de eventos" y "Mapa".
  - En modo mapa, `CompactEventList` muestra los eventos del día; al seleccionar uno se resalta en el mapa.
- **Columna derecha:**
  - Cabecera con la fecha seleccionada (`formatLongDate`) y el número de eventos.
  - En modo lista: `EventCard` compactas. En modo mapa (escritorio): `MapView` con los eventos del día.
  - Estado vacío: "No hay eventos este día".
  - En móvil, al final hay un mapa (`MapView` sin eventos).
- Estado: `selectedDate` (hoy por defecto), `selectedCategory` (`"All"`), `showMap`, `selectedEventId`.

### `/calendar` Calendario

`app/calendar/page.tsx`, componente cliente.

- Pestañas superiores: Calendario (activa), Descubrir (`/`) y "Mis entradas" (`/my-events`).
- `CategoryFilter` y navegación entre meses.
- Cuadrícula mensual que empieza en domingo. Cada día muestra hasta 2 eventos como etiquetas de color que enlazan al detalle, y "+N más" si hay más.
- Colores por categoría en `CATEGORY_DOT_COLORS`, con una leyenda debajo de la cuadrícula.
- El botón "Mapa" sustituye la cuadrícula por `MapView`, pero sin eventos.
- El mes inicial está fijado a **octubre de 2024** (`new Date(2024, 9, 1)`).

### `/events/[id]` Detalle del evento

`app/events/[id]/page.tsx`, **Server Component**.

- Carga el evento con `getEvent(id)`. Si falla, muestra `notFound()` (404).
- En paralelo carga:
  - los eventos de la misma categoría (`getEvents({ category })`), de los que muestra hasta 3 como "Más eventos de …";
  - si el evento está guardado (`isEventSaved(ConsumerUser.id, id)`). Si esa llamada falla, se muestra como no guardado y la página no se rompe.
- Contenido:
  - Imagen de cabecera (`EventImage`, con un degradado de fondo si no hay imagen), botón de compartir, botón de guardar (icono) y la categoría.
  - Título, artista, descripción, fecha, horario, lugar y dirección, y la lista "Qué incluye" (`perks`).
  - Barra lateral con precio ("Gratis" si `price === 0`), etiqueta "Se agotan rápido", botón de entradas o inscripción y botón "Guardar para después".
- **Enlace "Volver":** por defecto vuelve a `/`. Con `?from=my-events` vuelve a `/my-events` ("Volver a mis eventos"). Los valores válidos están en la lista blanca `BACK_LINKS` para evitar redirecciones arbitrarias. Para añadir otro origen, se añade una entrada a `BACK_LINKS` y se enlaza con `?from=<clave>`.

### `/my-events` Mis eventos

`app/my-events/page.tsx` (Server Component) y `components/MyEventsView.tsx` (cliente).

1. La página pide los eventos guardados con `getSavedEvents(ConsumerUser.id)`.
2. `groupUpcomingByDay()`:
   - descarta los eventos que ya han terminado (`hasEnded`: `date` + `endTime`, o `23:59` si no hay hora de fin, comparado con la hora local del **servidor**);
   - ordena por fecha y hora de inicio;
   - agrupa por día en `EventDay[]` (`{ date, events }`).
3. Tres estados:
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
  - `"icon"`: botón redondo con un marcador sobre la imagen de cabecera.
- Al pulsarlo llama a `createSavedEvent` o `deleteSavedEvent` y después hace `router.refresh()` dentro de una transición. Así el Server Component vuelve a calcular `saved` y **todos** los botones de la página se actualizan a la vez.
- El botón queda deshabilitado mientras se guarda y mientras se refresca la página (`busy`).
- Si la llamada falla, el estado no cambia y no se muestra ningún mensaje de error.

## Componentes

| Componente | Tipo | Descripción |
|------------|------|-------------|
| `EventCard` | Cliente | Tarjeta de evento con dos formatos (ver abajo). Exporta `CATEGORY_COLORS` (clases Tailwind por categoría). |
| `EventImage` | Cliente | `<img>` para imágenes remotas (ver abajo). |
| `CalendarWidget` | Cliente | Mini calendario mensual. Props: `selectedDate`, `onSelect`, `eventDates` (`Set` de fechas `YYYY-MM-DD` con eventos). Resalta el día seleccionado y el de hoy. |
| `CategoryFilter` | Cliente | Botones de categoría con icono, incluido `"All"`. Props: `selected`, `onChange`. |
| `CompactEventList` | Cliente | Lista seleccionable (título, categoría, hora y lugar) para el modo mapa de `/`. |
| `SwitchMapWidget` | Cliente | Conmutador Lista / Mapa. |
| `MapView` | Cliente | Envoltorio del mapa: crea los marcadores, muestra el evento seleccionado y la leyenda. |
| `EventsMap` | Cliente, sin SSR | Mapa de Leaflet (ver [Mapa](#mapa)). |
| `MyEventsView` | Cliente | Vista de "Mis eventos" (lista por días y mapa). |
| `SaveEventButton` | Cliente | Guardar o quitar un evento. |

### `EventCard`

| Prop | Por defecto | Efecto |
|------|-------------|--------|
| `compact` | `false` | Fila de lista sin imagen: categoría, título, horario, lugar, precio y botón "+". |
| `detailed` | `true` | Solo en el formato con imagen. `false` muestra solo imagen, título, fecha y "Detalles" (se usa en las tarjetas del mapa). |
| `approximateLocation` | `false` | Añade "Approximate location" (ubicación aproximada). |
| `selected` / `onSelect` | — | Solo en formato compacto. Con `onSelect`, pulsar la tarjeta la **selecciona** y solo el "+" abre el detalle. Sin `onSelect`, toda la tarjeta es un enlace. |
| `href` | `/events/{id}` | Sustituye el enlace, por ejemplo para añadir `?from=my-events`. |

### `EventImage`

- Usa `<img>` en lugar de `next/image` porque las imágenes vienen de dominios de los scrapers que no se conocen de antemano.
- `referrerPolicy="no-referrer"`: algunas webs de origen (por ejemplo feteas.org) rechazan las peticiones que llevan un `Referer` ajeno.
- Si no hay `src` o la imagen falla, muestra `fallback`.
- Guarda un `Image` vivo por URL (`retainedImages`) para que el navegador reutilice la imagen al volver a montarla (tarjetas del mapa al pasar el ratón, detalle tras navegar) sin volver a pedirla.

## Mapa

`MapView` (envoltorio) + `EventsMap` (Leaflet con `react-leaflet`, teselas de OpenStreetMap).

- `EventsMap` se carga con `next/dynamic` y `ssr: false`, porque Leaflet usa `window` al importarse.
- El mapa se centra en **Oviedo** (`43.3614, -5.8494`) con zoom 13.
- **Marcadores:**
  - Evento con `lat` y `lon` → **pin** SVG.
  - Evento sin coordenadas → se coloca en el centro de Oviedo como **círculo discontinuo** de 400 m ("ubicación aproximada").
  - Todos los marcadores usan el mismo color (`#14b8a6`). El seleccionado se pinta en naranja (`#ec5b13`), más grande y por encima de los demás.
- **Interacción:**
  - Clic en un marcador → selecciona o deselecciona el evento (`onSelect`) y abre su tarjeta.
  - Pasar el ratón → muestra la tarjeta (`EventCard` con `detailed={false}`). Se cierra 250 ms después de salir, para que dé tiempo a mover el cursor del pin a la tarjeta.
  - La tarjeta del evento seleccionado queda abierta, y la del evento bajo el cursor puede aparecer a la vez.
  - Al seleccionar un evento, el mapa se desplaza hasta él (`FlyToSelected`).
- `MapView` muestra encima del mapa una ficha con el evento seleccionado y, debajo, la leyenda de categorías.
- El contenedor lleva `isolate` para que las capas de Leaflet (con `z-index` alto) no tapen la barra de navegación.
- Los iconos de los pines se guardan en caché por color y estado para que `react-leaflet` no los recree en cada render.

## Fechas e idiomas

- Las fechas de los eventos son cadenas `YYYY-MM-DD`. Para convertirlas en `Date` se les añade `T12:00:00` (o `T00:00:00` en `EventCard`) para que se interpreten en hora local y no en UTC.
- Formateadores en `i18n/format.ts`, que usan `Intl` con el idioma activo:
  - `formatLongDate(date, locale, withYear?)` → "Martes, 30 de septiembre".
  - `formatMonthYear(year, month, locale)` → "Septiembre de 2026".
  - `weekdayNames(locale, "short" | "narrow")` → nombres de los días empezando en domingo.
- En componentes cliente el idioma se obtiene de `useTranslation().i18n.language`. En Server Components, de `getTranslation()` (`i18n/server.ts`), que devuelve `t` y `locale`.
- Los textos están en `i18n/locales/{es,en}.json`, en las secciones `common`, `categories`, `event` y `myEvents`.

## Limitaciones conocidas y pendientes

**Usuario de los eventos guardados**

- `SaveEventButton`, `/events/[id]` y `/my-events` usan el usuario fijo `ConsumerUser` en vez del usuario de la sesión. Cuando el backend verifique el JWT (ver [autenticacion.md](autenticacion.md#pendiente)), hay que:
  - cambiar a rutas `/api/me/saved-events`, en las que Go saca el usuario del token;
  - enviar el JWT desde los Server Components (`auth.api.getToken({ headers: await headers() })`), porque el interceptor de axios solo lo añade en el navegador;
  - ocultar o redirigir a `/login` las acciones de guardar cuando no hay sesión.
- `isEventSaved` descarga la lista completa de eventos guardados para comprobar uno solo.

**Datos y rendimiento**

- `/` y `/calendar` descargan todos los eventos y filtran en el cliente, aunque la API ya admite `?date=` y `?category=`.
- `/` pierde un posible error de carga (`console.error`) y lo muestra como "no hay eventos".

**Fechas**

- En `/` y en `CalendarWidget`, `toISODateStr` usa `toISOString()`, que da la fecha en **UTC**. Justo después de medianoche en España (UTC+1/+2), el día de "hoy" se interpreta como el día anterior. Los días que se eligen en el calendario no se ven afectados, porque se crean a las 12:00.
- `/calendar` empieza siempre en octubre de 2024.
- `hasEnded` (en `/my-events`) compara con la hora del servidor, no con la del usuario.
- `EventCard` formatea la fecha corta siempre en inglés (`"en-US"`).

**Mapa**

- Todos los marcadores tienen el mismo color. Hay un `TODO` en `MapView` para colorearlos por categoría con `LEGEND_ITEMS`, a la que además le falta `Sports`.
- El mapa de `/calendar` y el mapa móvil de `/` se muestran sin eventos.

**Interfaz sin funcionalidad todavía**

- En el detalle: los botones de compartir y de comprar entradas no hacen nada, y "Se agotan rápido" se muestra siempre.
- El marcador de la tarjeta con imagen (`EventCard`) no guarda nada y depende de `isSaved`, que la API no rellena.
- "Approximate location" en `EventCard` está en inglés fijo, aunque existe la clave `common.approximateLocation`.
- Los colores de categoría están repetidos en `EventCard` (`CATEGORY_COLORS`), `/events/[id]`, `/calendar` (`CATEGORY_DOT_COLORS`) y `MapView` (`LEGEND_ITEMS`).
