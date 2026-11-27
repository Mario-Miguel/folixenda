# Eventos y "Mis eventos"

Esta guía describe cómo el frontend obtiene, muestra y guarda eventos: la página principal, el calendario, el detalle de un evento, el mapa, la página "Mis eventos", los eventos de varios días, las imágenes subidas y el editor de eventos.

Para el login, el registro, el JWT que se envía a la API y los permisos de cada rol, ver [autenticacion.md](autenticacion.md). Para los tipos de cuenta, el perfil, el panel de publicador `/publisher` y la propiedad de los eventos, ver [perfiles.md](perfiles.md).

## Índice

- [Modelo de datos](#modelo-de-datos)
  - [Sesiones (eventos de varios días)](#sesiones-eventos-de-varios-días)
  - [Imágenes](#imágenes)
- [Categorías](#categorías)
- [Acceso a la API](#acceso-a-la-api)
  - [Eventos (`lib/api/events.ts`)](#eventos-libapieventsts)
  - [Imágenes de eventos](#imágenes-de-eventos)
  - [Geocodificación (`lib/api/geocode.ts`)](#geocodificación-libapigeocodets)
  - [Eventos guardados (`lib/api/userSavedEvents.tsx`)](#eventos-guardados-libapiusersavedeventstsx)
- [Almacenamiento de imágenes (backend)](#almacenamiento-de-imágenes-backend)
- [Páginas](#páginas)
  - [`/` Descubrir](#-descubrir)
  - [`/calendar` Calendario](#calendar-calendario)
  - [`/events/[id]` Detalle del evento](#eventsid-detalle-del-evento)
  - [`/my-events` Mis eventos](#my-events-mis-eventos)
  - [`/publisher` Mis publicaciones](#publisher-mis-publicaciones)
  - [Editor de eventos](#editor-de-eventos)
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
  // Primera sesión (para las vistas de un solo día). En eventos de varios días, usar `sessions`.
  date: string;       // "YYYY-MM-DD" de la primera sesión
  startTime: string;  // "HH:MM" de la primera sesión
  endTime: string;    // "HH:MM" de la primera sesión
  endDate: string;    // "YYYY-MM-DD" de la última sesión (== date si es de un día)
  sessions: EventSession[]; // siempre ≥ 1, ordenadas por fecha y hora de inicio
  venue: string;
  address: string;
  price: number;      // 0 = gratis
  imageUrl?: string;  // portada subida si la hay; si no, la URL externa guardada (scrapers)
  images: EventImage[]; // imágenes subidas: portada primero y después la galería por posición
  artistName?: string;
  perks?: string[];   // "Qué incluye"
  lat?: number;
  lon?: number;
  publisherId?: string;  // propietario, si lo creó un publisher (los de scrapers no tienen)
  organizer?: Organizer; // solo en GET /events/{id}, si el propietario tiene perfil de organización
}

// Un día del evento. endTime < startTime = termina después de medianoche (al día siguiente).
interface EventSession { date: string; startTime: string; endTime: string }

type EventImageKind = "cover" | "gallery";
interface EventImage {
  id: string;
  url: string;          // URL pública, lista para <img src>
  kind: EventImageKind; // como mucho 1 portada; hasta 10 de galería
  position: number;     // orden en la galería, desde 0 (la portada siempre 0)
  width?: number;
  height?: number;
}

interface Organizer { id: string; orgName: string; website?: string; phone?: string; description?: string }

// Evento de un publisher con el número de usuarios que lo han guardado (GET /me/events)
interface PublisherEvent extends Event { savesCount: number }

interface ApiEventsResponse { events: Event[]; total: number }

// Eventos guardados
interface SavedEvent extends Event { savedAt: string }
interface UserSavedEvents { userId: string; savedEvents: SavedEvent[] }
```

- El tipo `Event` ya no tiene `isSaved`. La API de Go todavía devuelve ese campo (columna `is_saved` de `events`), pero el frontend lo ignora: si un evento está guardado se pregunta por usuario (ver [Guardar eventos](#guardar-eventos)).
- No hay ningún campo con la URL de venta de entradas.
- `publisherId`, `organizer`, `images` y los campos derivados de las sesiones (`date`, `startTime`, `endTime`, `endDate`) los pone siempre la API: no se envían al crear ni al editar (ver `EventInput` abajo).

### Sesiones (eventos de varios días)

Un evento tiene una o más **sesiones** (`sessions`), una por día (o varias el mismo día con distinta hora de inicio). Es la fuente de verdad de las fechas:

- La API las devuelve siempre ordenadas por fecha y hora de inicio. `date`/`startTime`/`endTime` son los de la primera sesión y `endDate` la fecha de la última, para que el código de un solo día siga funcionando.
- Una sesión con `endTime` anterior a `startTime` (por ejemplo 22:00–02:00) termina al día siguiente, pero sigue perteneciendo a su día de inicio.
- Un evento es "de varios días" (`isMultiDay`) si sus sesiones caen en más de una fecha distinta. Una sola sesión que cruza la medianoche no lo es.
- En la base de datos están en la tabla `event_sessions` (migración `backend/migrations/00004_event_sessions_and_images.sql`). La API borra y vuelve a insertar las sesiones del evento en cada `POST`/`PUT`, dentro de la misma transacción. `events.start_time`/`end_time` guardan el intervalo total (inicio de la primera sesión, fin de la última).
- Los scrapers insertan directamente en `events` y no saben nada de sesiones: el trigger `AFTER INSERT` `events_default_session` crea una sesión a partir de `start_time`/`end_time` de cada evento insertado. La migración también creó una sesión para cada evento que ya existía.

### Imágenes

- `images` son las imágenes **subidas** (tabla `event_images`): como mucho una portada (`cover`, índice único parcial) y hasta 10 de galería ordenadas por `position`. Los eventos de scrapers tienen `images: []`.
- `imageUrl` es la URL de la portada subida si existe; si no, la URL externa guardada en `events.image_url` (las de los scrapers o un enlace puesto a mano). Mientras haya portada, el `imageUrl` del cuerpo de un `PUT` se ignora y se conserva la URL externa guardada, para cuando se borre la portada.
- Dónde se guardan los ficheros: ver [Almacenamiento de imágenes](#almacenamiento-de-imágenes-backend).

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
| `createEvent(event: EventInput)` | `POST /events` | [Editor de eventos](#editor-de-eventos). Requiere `admin` o `publisher` (`RequireCanPublish`). Un `publisher` queda como propietario. |
| `updateEvent(id, event: EventInput)` | `PUT /events/{id}` | Editor de eventos. **Reemplazo completo**: hay que enviar todos los campos (los que falten se vacían). Un `publisher` solo puede editar los suyos (`403`). |
| `deleteEvent(id)` | `DELETE /events/{id}` | `/admin` y `/publisher`. Un `publisher` solo puede borrar los suyos (`403`). También borra los ficheros de sus imágenes. |

```ts
// Lo que se envía al crear o reemplazar un evento. La API fija publisherId a partir del token,
// rellena organizer, deriva date/startTime/endTime/endDate de `sessions` y gestiona las imágenes
// con sus propios endpoints.
type EventInput = Omit<
  Event,
  "id" | "publisherId" | "organizer" | "date" | "startTime" | "endTime" | "endDate" | "images"
>;
```

Los eventos propios de un publicador (con `savesCount`) se piden con `getMyEvents()` de `lib/api/me.ts` (`GET /me/events`, ver [perfiles.md](perfiles.md#apime)).

Filtros de `getEvents` (`EventsQuery`), todos opcionales y combinables:

| Parámetro | Formato | Efecto |
|-----------|---------|--------|
| `date` | `YYYY-MM-DD` | Eventos con alguna sesión que **empieza** ese día (un evento de varios días aparece en cada uno de sus días). |
| `from` / `to` | `YYYY-MM-DD` | Eventos con alguna sesión que empieza en el rango, **ambos incluidos**. |
| `category` | Nombre de categoría | Solo esa categoría. `"All"` no se envía a la API. |

La API devuelve `400` si una fecha no tiene el formato `YYYY-MM-DD` o si `from` es posterior a `to`. Sin filtros de fecha ordena por el inicio del evento; con ellos, por la primera sesión que cumple el filtro (después por `id`). Cada evento sale una sola vez aunque tenga varias sesiones en el rango.

Validación al crear o editar (`prepareEvent` en `backend/handlers/events.go`; el editor replica las mismas reglas en `components/event-editor/model.ts`). Si falla, `400` con el mensaje:

| Campo | Regla |
|-------|-------|
| `title` | Obligatorio (tras `TrimSpace`), máximo 200 caracteres. |
| `description` | Máximo 5000 caracteres. |
| `venue` | Obligatorio. |
| `price` | `0` o más. |
| `perks` | Máximo 20 elementos de 80 caracteres. |
| `sessions` | Obligatorio: entre 1 y 60. Cada una con `date` `YYYY-MM-DD` real y `startTime`/`endTime` `HH:MM`; no puede haber dos con la misma fecha y hora de inicio. Los `date`/`startTime`/`endTime`/`endDate` del cuerpo se ignoran. |
| `imageUrl` | Vacío o URL absoluta `http(s)` (solo se comprueba si el evento no tiene portada subida). |

Al editar o borrar, responde `404` si el evento no existe y `403` si un `publisher` intenta tocar un evento que no es suyo. Las reglas completas de propiedad están en [perfiles.md](perfiles.md#propiedad-de-los-eventos).

### Imágenes de eventos

También en `lib/api/events.ts`. Solo el propietario (`publisher`) o un `admin`, con la misma comprobación que `PUT /events/{id}` (`loadOwned`): `401` sin sesión, `403` si no es suyo, `404` si el evento no existe.

| Función | Petición | Detalles |
|---------|----------|----------|
| `uploadEventImage(eventId, file, kind, onProgress?)` | `POST /events/{id}/images` (multipart: `file` + `kind` `cover`\|`gallery`) | `201` con la `EventImage`. El tipo se detecta por los primeros bytes (no por la extensión ni el `Content-Type`): solo JPEG, PNG y WebP (`400` si no). Máximo 5 MB (`413`). Máximo 10 de galería (`409`). Una portada nueva **reemplaza** a la anterior y borra su fichero. `onProgress` recibe la fracción subida. |
| `deleteEventImage(eventId, imageId)` | `DELETE /events/{id}/images/{imageId}` | `204`; `404` si la imagen no es de ese evento. Borra también el fichero. |
| `reorderEventImages(eventId, imageIds)` | `PUT /events/{id}/images/order` con `{ imageIds }` | `imageIds` debe contener **todas** las imágenes de galería una sola vez (`400` si no). Devuelve todas las imágenes del evento. |

Los ficheros se guardan con la clave `events/{eventId}/{imageId}.{ext}`; como nunca se reutiliza una clave, se sirven con caché inmutable.

### Geocodificación (`lib/api/geocode.ts`)

La API de Go hace de proxy para el editor, así que el navegador nunca llama a CartoCiudad ni a Nominatim. Requiere sesión que pueda publicar (`protect` + `RequireCanPublish`: `admin` o `publisher`); si no, `401`/`403`.

| Función | Petición | Devuelve |
|---------|----------|----------|
| `searchAddress(q, limit = 8, signal?)` | `GET /geocode/search?q=&limit=` | `GeocodeResult[]` (puede ser `[]`). `q` necesita al menos 3 caracteres; `limit` máximo 10. `400` si no. |
| `reverseGeocode(lat, lon, signal?)` | `GET /geocode/reverse?lat=&lon=` | `GeocodeResult` de ese punto; `404` si no hay nada; `400` con coordenadas fuera de rango. |

```ts
interface GeocodeResult {
  label: string;        // texto completo, p. ej. "Calle Uría 10, Oviedo, Asturias"
  address: string;      // calle y número, para Event.address
  municipality?: string;
  province?: string;
  postalCode?: string;
  lat: number;
  lon: number;
}
```

En el backend (`backend/geocode/`): primero CartoCiudad (IGN, muy preciso en España) y, si falla o no encuentra nada, Nominatim (OpenStreetMap). Las llamadas a Nominatim se serializan a como mucho una cada 1,1 s, todas las peticiones llevan un `User-Agent` genérico de la aplicación (sin datos personales) y los resultados se guardan 10 minutos en una caché en memoria. Si fallan los dos servicios, `502`.

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

## Almacenamiento de imágenes (backend)

`backend/storage/` define la interfaz `Storage` (`Put`, `Delete`, `URL`). `main.go` elige la implementación con `STORAGE_DRIVER` (plantilla de variables en `backend/.env.example`):

| `STORAGE_DRIVER` | Implementación | Uso |
|------------------|----------------|-----|
| `local` (por defecto) | `storage.Local`: ficheros en `LOCAL_STORAGE_DIR` (por defecto `backend/uploads/`, ignorado por git y por `air`), servidos por la propia API en `GET /uploads/...` (`LOCAL_STORAGE_PUBLIC_URL`, por defecto `http://localhost:PORT/uploads`). Solo sirve `.jpg`/`.jpeg`/`.png`/`.webp`, nunca lista directorios, rechaza rutas que salgan del directorio y envía `Cache-Control: public, max-age=31536000, immutable`. | Desarrollo. |
| `s3` | `storage.MinIO` (cliente `minio-go`): cualquier servicio compatible con S3 (AWS S3, Cloudflare R2, MinIO...) con `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_USE_SSL`, `S3_REGION`, `S3_PUBLIC_URL` y `S3_SKIP_BUCKET_SETUP`. Al arrancar crea el bucket y le pone una política de lectura pública, salvo con `S3_SKIP_BUCKET_SETUP=true`. | Producción. |

El disco local es el valor por defecto en desarrollo porque la imagen oficial `minio/minio` ya no está en Docker Hub. No hay `docker-compose`. Si el almacenamiento no se puede inicializar, la API no arranca.

`EventImage` usa `<img>` (no `next/image`), así que no hace falta configurar dominios de imágenes en Next.js para ninguno de los dos.

## Páginas

### `/` Descubrir

`app/page.tsx`, componente cliente.

- Hace dos consultas con `useEvents`, ambas filtradas por categoría en la API:
  - los eventos del **día seleccionado** (`date`), que son los que se muestran. Incluye los eventos de varios días que tienen una sesión ese día; se ordenan por la hora de inicio de **esa** sesión (`sessionOn`);
  - los del **mes visible** en el mini calendario (`from`/`to` con `monthRange`), solo para marcar con un punto los días que tienen eventos. Un evento de varios días marca **cada** día de sus sesiones.
- **Columna izquierda** (solo en escritorio, `lg`):
  - `CalendarWidget`: mini calendario. El mes visible lo controla la página (`calendarMonth`) para poder cargar sus eventos.
  - `SwitchMapWidget`: alterna entre "Lista de eventos" y "Mapa".
  - En modo mapa, `CompactEventList` muestra los eventos del día; al seleccionar uno se resalta en el mapa.
- **Columna derecha:**
  - Cabecera con la fecha seleccionada (`formatLongDate`) y el número de eventos.
  - En modo lista: `EventCard` compactas. En modo mapa (escritorio): `MapView` con los eventos del día.
  - Todos reciben `day={selectedDay}`: muestran el horario de la sesión de ese día y, en eventos de varios días, la etiqueta "Día 2 de 3" (ver [`EventSchedule`](#eventschedule)).
  - Estados: cargando, error (`LoadError` con "Reintentar") y vacío ("No hay eventos este día").
  - En móvil, al final hay un mapa con los eventos del día, que comparte la selección con el resto de la página.
- Estado: `selectedDate` (hoy por defecto), `calendarMonth` (mes actual), `selectedCategory` (`"All"`), `showMap`, `selectedEventId`.

### `/calendar` Calendario

`app/calendar/page.tsx`, componente cliente.

- Pestañas superiores: Calendario (activa), Descubrir (`/`) y "Mis entradas" (`/my-events`).
- `CategoryFilter` y navegación entre meses. Empieza en el **mes actual**.
- Carga solo los eventos del mes visible (`from`/`to`), filtrados por categoría en la API. Si falla, muestra `LoadError` en lugar de la cuadrícula o el mapa.
- Cuadrícula mensual que empieza en domingo. Un evento de varios días aparece en **cada** día que tiene una sesión (una sola vez por día aunque tenga dos sesiones ese día). Cada día muestra hasta 2 eventos (ordenados por la hora de inicio de su sesión de ese día) como etiquetas del color de su categoría que enlazan al detalle, y "+N más" si hay más. En eventos de varios días la etiqueta añade "2/3" y el `title` incluye el horario y "Día 2 de 3".
- Debajo de la cuadrícula, una leyenda con todas las categorías (`CATEGORIES`).
- El botón "Mapa" sustituye la cuadrícula por `MapView` con los eventos del mes.

### `/events/[id]` Detalle del evento

`app/events/[id]/page.tsx`, **Server Component**.

- Carga el evento con `getEvent(id)`. Si falla, muestra `notFound()` (404).
- Obtiene el JWT con `getServerApiToken()` y, en paralelo:
  - carga los eventos de la misma categoría (`getEvents({ category })`) y muestra hasta 3 que no hayan terminado (sin contar el actual) como "Más eventos de …";
  - consulta si el evento está guardado con `isEventSaved(id, token)`;
  - si el evento tiene `publisherId`, lee el id del usuario de la sesión (`auth.api.getSession`) para saber si es el propietario.
- Después consulta, también con `isEventSaved`, el estado de cada evento relacionado, para que sus tarjetas muestren el botón de guardar.
- Sin sesión no se hace ninguna consulta de guardados: todo cuenta como no guardado. Si una consulta falla, ese evento se muestra como no guardado y la página no se rompe.
- Contenido:
  - Imagen de cabecera: la portada subida o, si no hay, `imageUrl` (`EventImage` con `loading="eager"` y un degradado de fondo si no hay imagen), `ShareButton`, botón de guardar (icono) y la categoría.
  - Título, artista y descripción.
  - Fecha y horario: en eventos de un día, la fecha larga y el horario (con "(termina al día siguiente)" si cruza la medianoche); en eventos de varios días, el rango (`formatEventDateRange`) y el resumen del horario (`formatScheduleSummary`).
  - Lugar y dirección, y la lista "Qué incluye" (`perks`).
  - **Fechas y horarios** (solo eventos de varios días): todas las sesiones, con las ya terminadas atenuadas ("Finalizada") y la que está en curso o es la siguiente marcada como "Próxima". El año solo se muestra si alguna sesión cae fuera del año actual.
  - **Galería** (`app/events/[id]/EventGallery.tsx`, cliente): cuadrícula de miniaturas de las imágenes `gallery`, en su orden. Al pulsar una se abre un visor (`<dialog>` modal nativo, así que el foco, `Esc` y la vuelta del foco los gestiona el navegador) con anterior/siguiente, flechas del teclado y deslizamiento táctil; bloquea el scroll de la página mientras está abierto.
  - Barra lateral (fija al hacer scroll) con:
    - Tarjeta de precio ("Gratis" si `price === 0`, "por persona" si no), una nota informativa y el botón "Guardar para después". No hay botón de comprar entradas porque el modelo no tiene URL de venta: la nota dice que las entradas las gestiona el organizador o el lugar (`event.ticketsInfo`) o, si es gratis, que se consulte si hace falta inscribirse (`event.freeEntryInfo`).
    - Si el usuario es el propietario: aviso "Has publicado este evento" con enlace a `/publisher`.
    - Si hay `organizer`: tarjeta "Organizado por" con el nombre, la descripción, la web (solo si es `http(s)`) y el teléfono (ver [perfiles.md](perfiles.md#detalle-del-evento-organizado-por)).
- **Enlace "Volver":** por defecto vuelve a `/`. Con `?from=my-events` vuelve a `/my-events` ("Volver a mis eventos") y con `?from=publisher` a `/publisher` ("Volver a mis publicaciones"). Los valores válidos están en la lista blanca `BACK_LINKS` para evitar redirecciones arbitrarias. Para añadir otro origen, se añade una entrada a `BACK_LINKS` y se enlaza con `?from=<clave>`.

### `/my-events` Mis eventos

`app/my-events/page.tsx` (Server Component) y `components/MyEventsView.tsx` (cliente).

1. La página obtiene el JWT con `getServerApiToken()`. Sin sesión, redirige a `/login?redirect=/my-events`.
2. Pide los eventos guardados con `getSavedEvents(token)`.
3. `groupUpcomingByDay()`:
   - toma de cada evento su **próxima sesión** (`nextSession`: la que está en curso o la siguiente) y descarta los eventos sin ninguna, es decir, cuya última sesión ya terminó (ver [Fechas e idiomas](#fechas-e-idiomas));
   - ordena por el inicio de esa sesión;
   - agrupa por el día de esa sesión en `EventDay[]` (`{ date, events }`). Un evento de varios días aparece una sola vez, en el día de su próxima sesión.
4. Tres estados:
   - Error al cargar → "No se han podido cargar tus eventos guardados".
   - Lista vacía → icono, "No tienes eventos guardados próximos" y enlace a `/`.
   - Con eventos → `MyEventsView`.

`MyEventsView`:

- A la izquierda, los eventos agrupados por día, con la fecha larga y el número de eventos de cada día. Las tarjetas reciben `day` con la fecha del grupo, así que muestran el horario de esa sesión.
- A la derecha, un mapa con **todos** los eventos próximos guardados (fijo al hacer scroll en escritorio; encima de la lista en móvil).
- Al pulsar una tarjeta se selecciona y se resalta su marcador en el mapa, y al revés. Volver a pulsarla la deselecciona.
- El botón "+" de cada tarjeta abre el detalle con `?from=my-events`, para que "Volver" regrese aquí.

En la barra de navegación, "Mis eventos" aparece para los `consumer`, los `admin` y sin sesión; los `publisher` ven en su lugar "Mis publicaciones". La página sigue funcionando para cualquier usuario con sesión.

### `/publisher` Mis publicaciones

Panel de los `publisher` para ver sus eventos (próximos y pasados, con el número de guardados), borrarlos y abrir el [editor](#editor-de-eventos) para crearlos o editarlos. Un evento es "pasado" cuando termina su **última** sesión. Está descrito en [perfiles.md](perfiles.md#publisher-mis-publicaciones).

### Editor de eventos

Páginas completas (Server Components) en `app/publisher/events/`, con el formulario cliente en `components/event-editor/`:

| Ruta | Fichero | Quién |
|------|---------|-------|
| `/publisher/events/new` | `app/publisher/events/new/page.tsx` | `publisher` y `admin` |
| `/publisher/events/[id]/edit` | `app/publisher/events/[id]/edit/page.tsx` | El `publisher` propietario, o cualquier `admin` |

Acceso (`app/publisher/events/access.tsx`):

- Sin sesión → `/login?redirect=<ruta del editor>`.
- `editorHomeFor(role)` decide a dónde vuelven "Volver", Cancelar y Guardar: `admin` → `/admin?tab=events`, `publisher` → `/publisher`. Cualquier otro rol ve una tarjeta "solo para publicadores". Es un espejo de `auth.CanPublish`, que sigue siendo la única puerta real.
- Al editar: `404` si el evento no existe, tarjeta de error si falla la carga y tarjeta "no es tuyo" si un `publisher` abre un evento de otro (la API respondería `403` igualmente).
- Al guardar se vuelve al panel con `?saved=created|updated`, que muestra el aviso y se quita de la URL. Rutas en `components/event-editor/routes.ts` (`NEW_EVENT_HREF`, `editHref`, `homeHref`, `publicHref`).

`EventEditor.tsx` tiene cuatro secciones y una barra inferior con Cancelar / Guardar:

1. **Información** (`InfoSection.tsx`): título\*, categoría\*, descripción, artista, gratis / precio y "Qué incluye" como chips (Enter o coma añade, Retroceso con el campo vacío quita el último).
2. **Fechas y horarios** (`ScheduleSection.tsx`):
   - **Un día:** fecha, inicio y fin.
   - **Varios días:** rango de fechas (máximo 60 días), horario común, filtro de días de la semana (lunes primero) y la lista de días generada (`generateDays`). Cada fila se puede ajustar (su propio horario), borrar o añadir a mano; las filas añadidas a mano se conservan aunque queden fuera del rango o del filtro, y los ajustes de un día sobreviven al cambiar el rango. Avisa de las sesiones que terminan al día siguiente.
3. **Lugar** (`PlaceSection.tsx`): lugar\* y dirección con `AddressSearch.tsx` (combobox ARIA 1.2 con autocompletado sobre `searchAddress`), más `LocationPicker.tsx` (mapa Leaflet cargado con `ssr: false`, marcador arrastrable). Elegir un resultado rellena la dirección y `lat`/`lon` y el mapa vuela al punto; hacer clic en el mapa o arrastrar el marcador mueve el punto y busca su dirección con `reverseGeocode` (el nombre del lugar nunca se toca). Se puede quitar el punto (el evento sale como "ubicación aproximada").
4. **Imágenes** (`ImagesSection.tsx` + `useEventImages.ts`): portada y galería (hasta 10), con selector de ficheros y arrastrar y soltar, reordenación (arrastrando o con flechas), barra de progreso y comprobación en cliente de tipo (JPEG/PNG/WebP) y tamaño (5 MB). Sin portada subida, se puede poner una URL externa de imagen.
   - **Al editar**, cada cambio de imágenes se guarda en la API en el momento (subir, borrar, reordenar).
   - **Al crear**, los ficheros se quedan en local; después de crear el evento se suben uno a uno (portada y galería en orden). Si alguno falla, se va a la página de edición del evento nuevo con `?created=1&failedUploads=<nombre>` y un aviso con los ficheros que no se subieron.

Otros detalles:

- `model.ts` contiene el estado del formulario (`EditorForm`), `initialForm(event)`, `validate` (las mismas reglas que la API, ver [Eventos](#eventos-libapieventsts)) y `toPayload`, que produce un `EventInput` **completo** con `sessions` (porque `PUT` es un reemplazo completo).
- Al pulsar Guardar con errores, se hace scroll al primero y se le da el foco. Los errores de la API se traducen en `errors.ts` (`describeApiError`), que asocia los `400` de Go a su campo.
- Enter en un campo de una línea no envía el formulario.
- Si hay cambios sin guardar, `beforeunload` avisa al recargar o cerrar la pestaña.
- Textos en `editor.*` de `i18n/locales/{es,en}.json`.

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
| `EventSchedule` | Cliente | `useEventWhen(event, day?)` y `DayBadge`: cómo se describe el horario de un evento en tarjetas y listas (ver abajo). |
| `EventImage` | Cliente | `<img>` para imágenes remotas (ver abajo). |
| `CalendarWidget` | Cliente | Mini calendario mensual. Props: `selectedDate`, `onSelect`, `eventDates` (`Set` de fechas `YYYY-MM-DD` con eventos), `month` y `onMonthChange` (el mes visible lo controla el padre). Resalta el día seleccionado y el de hoy. |
| `CategoryFilter` | Cliente | Botones de categoría con icono, incluido `"All"`. Props: `selected`, `onChange`. |
| `CompactEventList` | Cliente | Lista seleccionable (título, categoría, hora y lugar) para el modo mapa de `/`. Acepta `day`. |
| `SwitchMapWidget` | Cliente | Conmutador Lista / Mapa. |
| `LoadError` | Cliente | Estado de error al cargar eventos ("No se han podido cargar los eventos"), con botón "Reintentar" si recibe `onRetry`. |
| `MapView` | Cliente | Envoltorio del mapa: crea los marcadores, muestra el evento seleccionado y la leyenda. Acepta `day` y lo pasa a las tarjetas. |
| `EventsMap` | Cliente, sin SSR | Mapa de Leaflet (ver [Mapa](#mapa)). |
| `MyEventsView` | Cliente | Vista de "Mis eventos" (lista por días y mapa). |
| `SaveEventButton` | Cliente | Guardar o quitar un evento. |
| `ShareButton` | Cliente | Compartir o copiar el enlace del evento. |
| `EventGallery` (`app/events/[id]/`) | Cliente | Galería y visor del detalle. |
| `event-editor/*` | Cliente | [Editor de eventos](#editor-de-eventos). |

### `EventCard`

| Prop | Por defecto | Efecto |
|------|-------------|--------|
| `compact` | `false` | Fila de lista sin imagen: categoría, título, horario, lugar, precio y botón "+". |
| `detailed` | `true` | Solo en el formato con imagen. `false` muestra solo imagen, título, fecha y "Detalles" (se usa en las tarjetas del mapa). |
| `approximateLocation` | `false` | Añade "Ubicación aproximada" (`common.approximateLocation`). |
| `selected` / `onSelect` | — | Solo en formato compacto. Con `onSelect`, pulsar la tarjeta la **selecciona** y solo el "+" abre el detalle. Sin `onSelect`, toda la tarjeta es un enlace. |
| `href` | `/events/{id}` | Sustituye el enlace, por ejemplo para añadir `?from=my-events`. |
| `day` | — | Día que se está viendo (`YYYY-MM-DD`). Muestra el horario de la sesión de ese día y, en eventos de varios días, "Día 2 de 3". Sin `day`, un evento de varios días muestra su rango y el resumen del horario. |
| `saved` | — | Solo en el formato con imagen y `detailed`. Si se pasa, muestra `SaveEventButton` (icono) con ese estado; los clics en él no abren el evento. Sin `saved`, no hay botón de guardar. |

La fecha corta del formato con imagen (día de la semana, día y mes abreviados) se formatea en el idioma activo.

### `EventSchedule`

`components/EventSchedule.tsx`. `useEventWhen(event, day?)` devuelve `{ time, nextDay, range, dayBadge, dayBadgeShort, multiDay }`:

- **Con `day`** y una sesión ese día (vistas de un día: `/`, un día del calendario, un grupo de `/my-events`): el horario de esa sesión, si termina al día siguiente y, en eventos de varios días, "Día 2 de 3" (`display.dayOfTotal`) / "2/3" (`display.dayOfTotalShort`).
- **Sin `day`**, evento de varios días: `range` = "18–20 dic" y `time` = "Vie–Dom · 10:00–20:00" (o "Horario variable", `display.variableSchedule`, si las sesiones tienen horas distintas).
- **Sin `day`**, evento de un día: el horario de su sesión.

`DayBadge` es la pastilla con la etiqueta "Día N de M".

### `EventImage`

- Usa `<img>` en lugar de `next/image` porque las imágenes subidas vienen del almacenamiento (disco local o S3) y las de los scrapers de dominios que no se conocen de antemano.
- `loading` es `"lazy"` por defecto (`"eager"` para la cabecera del detalle y el visor). Acepta `width`/`height` (las imágenes subidas los tienen) para reservar la proporción mientras carga, y `draggable`.
- `referrerPolicy="no-referrer"`: algunas webs de origen (por ejemplo feteas.org) rechazan las peticiones que llevan un `Referer` ajeno.
- Si no hay `src` o la imagen falla, muestra `fallback`.
- Guarda un `Image` vivo por URL (`retainedImages`) para que el navegador reutilice la imagen al volver a montarla (tarjetas del mapa al pasar el ratón, detalle tras navegar) sin volver a pedirla. Solo las retiene cuando ya han cargado, para que las diferidas (`lazy`) no se pidan antes de verse.

## Mapa

`MapView` (envoltorio) + `EventsMap` (Leaflet con `react-leaflet`, teselas de OpenStreetMap).

Dónde se usa: modo mapa de `/` en escritorio y mapa móvil de `/` (eventos del día, con `day`), modo mapa de `/calendar` (eventos del mes) y `/my-events` (eventos guardados próximos). El editor tiene su propio mapa (`LocationPicker`, ver [Editor de eventos](#editor-de-eventos)).

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
  - `todayInEventTimeZone(now?)` → el día de hoy en España; `addDays(date, n)` → aritmética sobre `YYYY-MM-DD`.
  - Sesiones (aceptan cualquier objeto con `date` y, opcionalmente, `startTime`/`endTime`/`endDate`/`sessions`; sin `sessions` usan la primera sesión):
    - `eventSessions(event)` → las sesiones ordenadas, nunca vacía.
    - `endsNextDay(session)` → `true` si termina después de medianoche.
    - `sessionOn(event, date)` → la sesión que **empieza** ese día.
    - `nextSession(event, now?)` → la sesión en curso o la siguiente; `undefined` si todas han terminado. `lastSession(event)` → la última.
    - `hasSessionEnded(session, now?)` y `hasEventEnded(event, now?)` → `true` cuando ya ha pasado la hora de fin (de la sesión, o de la **última** sesión del evento) en **`Europe/Madrid`** (`EVENT_TIME_ZONE`), sea cual sea la zona horaria de la máquina. Sin hora de fin, la sesión dura todo el día (`23:59`); si la hora de fin es anterior a la de inicio (por ejemplo, 22:00–02:00), termina al día siguiente.
    - `sessionStart` / `sessionEnd` → instantes absolutos (`Date`) de una sesión, con el cambio de hora resuelto.
    - `sessionDates(event)`, `isMultiDay(event)` (más de una fecha distinta) y `sessionDayIndex(event, date)` (`{ day, total }` para "Día 2 de 3").
    - `compareSessions` / `compareSessionEnds` → comparadores por inicio y por fin.
  - Formato (con `Intl` y el idioma activo): `formatSessionTime` ("20:00–23:00"), `formatSessionDate` ("Viernes, 18 de diciembre"), `formatWeekdayShort` ("Vie"), `formatEventDateRange` ("20–22 dic", "30 nov – 2 dic"; el año solo si el rango cruza años o con `withYear`) y `formatScheduleSummary` ("Vie–Dom · 10:00–20:00", o el texto de horario variable si las horas cambian).
- Formateadores en `i18n/format.ts`, que usan `Intl` con el idioma activo:
  - `formatLongDate(date, locale, withYear?)` → "Martes, 30 de septiembre".
  - `formatMonthYear(year, month, locale)` → "Septiembre de 2026".
  - `weekdayNames(locale, "short" | "narrow")` → nombres de los días empezando en domingo.
- En componentes cliente el idioma se obtiene de `useTranslation().i18n.language`. En Server Components, de `getTranslation()` (`i18n/server.ts`), que devuelve `t` y `locale`.
- Los textos están en `i18n/locales/{es,en}.json`, en las secciones `common`, `categories`, `display` (días y horarios de varios días), `event`, `editor` (editor de eventos) y `myEvents`.

## Limitaciones conocidas y pendientes

**Datos y permisos**

- No hay URL de venta de entradas en el modelo de datos, por eso el detalle solo muestra una nota informativa. Cuando exista, se puede volver a poner un botón de compra.
- El panel de admin carga **todos** los eventos (`getEvents()` sin filtros) en una sola tabla, sin paginación. La columna Propietario solo muestra el id corto del publicador, porque no hay un endpoint para buscar su nombre.
- **Ficheros huérfanos al borrar una cuenta:** borrar un publicador en Better Auth borra en cascada sus eventos y las filas de `event_images`, pero no los ficheros del almacenamiento. Solo `DELETE /api/events/{id}` los borra. Hay un `TODO` en `backend/handlers/event_images.go` (por ejemplo, un proceso periódico que borre los objetos de `events/` sin fila). Los borrados de ficheros tras un `DELETE` son "best effort": si fallan, solo se registran.
- El `Down` de la migración `00004` pierde las sesiones extra (cada evento se queda con su intervalo total) y las filas de imágenes (los ficheros se quedan).
- Ver también los pendientes de publicadores en [perfiles.md](perfiles.md#pendiente) (pagos, `publisherId` de un admin sin validar, etc.).
- "Más eventos de …" en el detalle pide todos los eventos de la categoría y se queda con los 3 primeros por fecha, que pueden ser eventos ya pasados.

**Fechas y sesiones**

- `?date=D` solo encuentra las sesiones que **empiezan** el día `D`: una sesión de 22:00 a 02:00 no aparece en las vistas del día siguiente, aunque siga en curso.
- Un evento con dos sesiones el mismo día aparece una sola vez en las vistas de un día (`/`, cada día de `/calendar`), con el horario de la primera sesión de ese día.
- `/my-events` agrupa cada evento de varios días solo en el día de su próxima sesión, no en todos sus días.

- `/` es un componente cliente, pero Next.js lo prerenderiza en el servidor: la fecha de "hoy" (`new Date()`) se calcula en el servidor y otra vez en el navegador. Si no coinciden (cerca de medianoche, o si el servidor está en otra zona horaria), puede haber un error de hidratación en la cabecera con la fecha y en el mini calendario.

**Editor**

- La navegación dentro de la app (enlaces, "Volver", botón atrás del router) no avisa de los cambios sin guardar del editor; solo lo hace `beforeunload` al recargar o cerrar la pestaña.

