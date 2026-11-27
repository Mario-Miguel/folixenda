# Tipos de cuenta, perfil y publicadores

Esta guía describe los dos tipos de cuenta de folixenda (personas y empresas o plataformas), cómo se elige el tipo al registrarse, la página de perfil, el panel de publicador "Mis publicaciones", el acceso al editor de eventos, la API `/api/me`, la propiedad de los eventos y qué se borra al eliminar una cuenta.

Para el login, la sesión, el JWT y la configuración general de Better Auth, ver [autenticacion.md](autenticacion.md). Para el modelo de eventos y el resto de páginas, ver [eventos.md](eventos.md).

## Índice

- [Arquitectura](#arquitectura)
- [Roles y tipos de cuenta](#roles-y-tipos-de-cuenta)
- [Ficheros](#ficheros)
- [Registro con tipo de cuenta](#registro-con-tipo-de-cuenta)
- [Redirecciones tras el login](#redirecciones-tras-el-login)
- [Páginas](#páginas)
  - [`Navbar`](#navbar)
  - [`/profile` Mi perfil](#profile-mi-perfil)
  - [`/publisher` Mis publicaciones](#publisher-mis-publicaciones)
  - [Detalle del evento: "Organizado por"](#detalle-del-evento-organizado-por)
  - [`/admin`: columna Propietario](#admin-columna-propietario)
- [Editor de eventos](#editor-de-eventos)
- [API](#api)
  - [`/api/me`](#apime)
  - [Propiedad de los eventos](#propiedad-de-los-eventos)
  - [Reglas de validación](#reglas-de-validación)
- [Modelo de datos](#modelo-de-datos)
- [Borrado de cuenta en cascada](#borrado-de-cuenta-en-cascada)
- [Publicación de pago (futuro)](#publicación-de-pago-futuro)
- [Pendiente](#pendiente)

## Arquitectura

```
/signup ──(accountType)──▶ Better Auth ──databaseHooks.user.create.before──▶ auth."user".role
   │                                                                     (consumer | publisher)
   └─ si es publisher: PUT /api/me/publisher-profile ──▶ API de Go ──▶ public.publisher_profiles

/profile, /publisher ──(JWT)──▶ GET /api/me, PUT /api/me/notifications,
                               GET/PUT /api/me/publisher-profile, GET /api/me/events
                               POST/PUT/DELETE /api/events (con control de propiedad)

/publisher/events/new, /publisher/events/[id]/edit ──(JWT)──▶ POST/PUT /api/events,
                               POST/DELETE/PUT /api/events/{id}/images…, GET /api/geocode/*
```

- **Better Auth** es dueño del nombre, el correo, la contraseña, el rol y la fecha de alta (`auth."user"`). El rol se decide **solo en el servidor** al crear el usuario.
- **La API de Go** es dueña del resto: perfil de organización, preferencias de notificaciones y propiedad de los eventos (`events.publisher_id`). El usuario sale siempre del token (`sub`), nunca de la URL.
- Borrar el usuario en Better Auth borra todo lo de Go en cascada (ver [Borrado de cuenta en cascada](#borrado-de-cuenta-en-cascada)).

## Roles y tipos de cuenta

| Rol | Tipo de cuenta en `/signup` | Cómo se obtiene | Qué puede hacer |
|-----|-----------------------------|-----------------|-----------------|
| `consumer` | Persona | Por defecto al registrarse | Guardar eventos (`/my-events`), perfil y notificaciones. |
| `publisher` | Empresa / plataforma | Registrarse con `accountType: "publisher"` | Además: perfil de organización y crear, editar y borrar **sus** eventos (y sus imágenes) desde `/publisher` y el editor. |
| `admin` | — (nunca al registrarse) | `UPDATE` en SQL o desde `/admin` | Gestionar usuarios y **todos** los eventos desde `/admin` (crear y editar con el mismo editor, `/publisher/events/*`). No tiene perfil de organización ni panel `/publisher`. |

**El tipo de cuenta es fijo.** Una persona no puede convertirse en empresa ni al revés: tiene que crear otra cuenta (con otro correo). Se aplica en tres sitios:

- **API de Go:** `auth.AccountTypeChangeAllowed(from, to)` (`backend/auth/roles.go`). `PUT /api/users/{id}` responde `409` si un admin intenta pasar un `consumer` a `publisher` o al revés. Dar o quitar el rol `admin` sigue permitido.
- **`/admin`:** el selector de rol solo ofrece el rol actual y `admin` (función `roleChangeAllowed`, espejo de la de Go), con una nota explicativa. Un `409` muestra `admin.users.accountTypeFixed`.
- **Usuarios:** no pueden cambiar su rol (`role` tiene `input: false` en Better Auth y Go ignora `role` en el `PUT` de un no-admin). `/signup` avisa de que la elección es permanente (`signup.accountType.permanent`) y `/profile` lo recuerda bajo el tipo de cuenta (`profile.account.typeFixed`).

Permisos en la API (el rol siempre es el del JWT):

| Endpoint | `consumer` | `publisher` | `admin` | Sin sesión |
|----------|------------|-------------|---------|------------|
| `GET /api/events`, `GET /api/events/{id}` | Sí | Sí | Sí | Sí |
| `POST /api/events` | `403` | Sí, siempre como propietario | Sí, con `publisherId` opcional | `401` |
| `PUT /api/events/{id}`, `DELETE /api/events/{id}` | `403` | Solo sus eventos (si no, `403`) | Cualquiera | `401` |
| `POST /api/events/{id}/images`, `DELETE /api/events/{id}/images/{imageId}`, `PUT /api/events/{id}/images/order` | `403` | Solo sus eventos (si no, `403`) | Cualquiera | `401` |
| `GET /api/geocode/search`, `GET /api/geocode/reverse` | `403` | Sí | Sí | `401` |
| `GET /api/me`, `PUT /api/me/notifications` | Sí | Sí | Sí | `401` |
| `GET /api/me/publisher-profile`, `PUT /api/me/publisher-profile` | `403` | Sí | `403` | `401` |
| `GET /api/me/events` | `403` | Sí | `403` | `401` |

`POST /api/events` y la geocodificación usan el middleware `RequireCanPublish` (la puerta de publicación, ver [Publicación de pago](#publicación-de-pago-futuro)); `PUT`, `DELETE` y los endpoints de imágenes usan `RequireRole(admin, publisher)` más la comprobación de propiedad del handler (`loadOwned`). Los endpoints de `/api/me` de publicador usan `RequireRole(publisher)`: los `admin` ven los eventos desde `/admin`.

## Ficheros

En el frontend:

| Fichero | Descripción |
|---------|-------------|
| `lib/better-auth/server.ts` | `databaseHooks.user.create.before` decide el rol al registrarse (`signupRole`). `user.deleteUser.enabled` activa el autoborrado de cuenta. |
| `lib/better-auth/redirect.ts` | `safeRedirect(target, fallback = "/")` y `homeForRole(role)` (ver [Redirecciones](#redirecciones-tras-el-login)). |
| `lib/api/me.ts` | `getMe`, `updateNotifications`, `getPublisherProfile`, `savePublisherProfile`, `getMyEvents`. Aceptan `token` para usarse desde Server Components. |
| `lib/api/events.ts` | `EventInput`, `createEvent` / `updateEvent` / `deleteEvent` y `uploadEventImage` / `deleteEventImage` / `reorderEventImages` (ver [eventos.md](eventos.md#eventos-libapieventsts)). |
| `lib/api/geocode.ts` | `searchAddress` y `reverseGeocode`, para el editor (ver [eventos.md](eventos.md#geocodificación-libapigeocodets)). |
| `lib/types.ts` | `Organizer`, `PublisherEvent`, `PublisherProfile`, `PublisherProfileInput`, `NotificationPreferences`, `MeResponse`, y `publisherId` / `organizer` en `Event`. |
| `app/signup/page.tsx` | Selector de tipo de cuenta y datos de la organización. |
| `app/profile/page.tsx` | Server Component de `/profile`: comprueba la sesión, llama a `getMe(token)` y monta las secciones. |
| `app/profile/AccountSection.tsx` | Sección Cuenta: correo, tipo de cuenta, fecha de alta y cambio de nombre. |
| `app/profile/OrganizationSection.tsx` | Sección Organización (solo `publisher`). |
| `app/profile/OrganizationFields.tsx` | Campos de organización compartidos con `/signup`, validación en cliente (`useOrgValidation`), `orgValuesFrom`, `toProfileInput` e `INPUT_CLASS`. |
| `app/profile/NotificationsSection.tsx` | Sección Notificaciones. |
| `app/profile/SecuritySection.tsx` | Sección Seguridad: cambio de contraseña. |
| `app/profile/DangerZone.tsx` | Zona de peligro: borrar la cuenta. |
| `app/profile/ui.tsx` | `Section`, `Toggle`, `Feedback`, `PrimaryButton`, `apiErrorMessage`, `authErrorMessage`, `MIN_PASSWORD_LENGTH`. |
| `app/publisher/page.tsx` | Server Component de `/publisher`: sesión, rol, `getMyEvents` + `getMe`, separación en próximos y pasados. |
| `app/publisher/PublisherDashboard.tsx` | Panel (cliente): estadísticas, pestañas, enlaces al editor y borrar. |
| `app/publisher/events/access.tsx` | `requireSession`, `editorHomeFor(role)`, `AccessCard` y `EditorShell`, compartidos por las páginas del editor. |
| `app/publisher/events/new/page.tsx`, `app/publisher/events/[id]/edit/page.tsx` | Páginas del editor (ver [Editor de eventos](#editor-de-eventos)). |
| `components/event-editor/*` | El editor: `EventEditor`, secciones, `model.ts`, `routes.ts`, `useEventImages.ts` (ver [eventos.md](eventos.md#editor-de-eventos)). |
| `components/Navbar.tsx` | Enlaces según el rol y entrada "Mi perfil" en el menú. |
| `app/events/[id]/page.tsx` | Tarjeta "Organizado por" y aviso para el propietario. |
| `app/admin/AdminDashboard.tsx` | Enlaza al editor para crear y editar y muestra la columna Propietario. |
| `i18n/locales/{es,en}.json` | Textos de `signup.accountType.*`, `signup.org*`, `profile.*`, `publisher.*`, `editor.*`, `event.organizedBy`, `nav.profile`, `nav.myPublications`, `admin.events.owner*`. |

En el backend de Go:

| Fichero | Descripción |
|---------|-------------|
| `auth/roles.go` | `CanPublish(claims)`: el único sitio que decide quién puede publicar (`admin` y `publisher`). |
| `middleware/roles.go` | `RequireCanPublish`: sin claims → `401`; `CanPublish` falso → `403`. |
| `handlers/me.go` | `MeHandler`: rutas `/api/me/*` y validación del perfil y de las notificaciones. |
| `handlers/events.go` | `owns` / `loadOwned`: control de propiedad en `PUT`, `DELETE` y las imágenes; `create` fija `publisherId`; `prepareEvent` valida el cuerpo. |
| `handlers/event_images.go` | Subida, borrado y reordenación de imágenes (con el mismo `loadOwned`). |
| `handlers/geocode.go` | `GET /api/geocode/*` detrás de `protect` + `RequireCanPublish`. |
| `models/profile.go` | `PublisherProfile`, `NotificationPreferences` y `DefaultNotificationPreferences()`. |
| `models/event.go` | `PublisherID`, `Organizer`, `PublisherEvent`, `Categories` y `EventCategory.Valid()`. |
| `store/publisher_profile_store.go` | `Get` y `Upsert` de `publisher_profiles`. |
| `store/notification_preferences_store.go` | `Get` y `Upsert` de `notification_preferences`. |
| `store/postgres_event_store.go` | `Get` (con `LEFT JOIN` al perfil del organizador) y `ListByPublisher` (con número de guardados). |
| `store/store.go` | `eventTimestamps`: fin de cada sesión al día siguiente si la hora de fin es anterior a la de inicio. |
| `migrations/00003_publishers_and_notifications.sql` | Tablas y columnas de publicadores y notificaciones (ver [Modelo de datos](#modelo-de-datos)). |
| `migrations/00004_event_sessions_and_images.sql` | `event_sessions` (con el trigger `events_default_session`) y `event_images` (ver [eventos.md](eventos.md#modelo-de-datos)). |

## Registro con tipo de cuenta

`/signup` empieza con un selector de dos tarjetas (`fieldset` de radios): **Persona** (`consumer`, por defecto) y **Empresa / plataforma** (`publisher`). Con Empresa:

- El campo de nombre pasa a llamarse "Nombre de contacto".
- Aparece la sección "Datos de la organización" con `OrganizationFields` (nombre obligatorio, web, teléfono y descripción), validada en el cliente con las mismas reglas que la API antes de crear la cuenta.

### Cómo se decide el rol

1. El cliente nunca envía `role`. Envía `accountType` junto al resto del cuerpo con `fetchOptions.body`:

   ```ts
   await signUp.email({ name, email, password, fetchOptions: { body: { accountType } } });
   ```

   El esquema del cuerpo de `/sign-up/email` acepta claves extra e ignora las que no son campos del usuario.
2. `user.additionalFields.role` tiene `input: false`, así que el cliente no puede fijar `role` en el registro.
3. `databaseHooks.user.create.before` se ejecuta para **cada** usuario que crea Better Auth y llama a `signupRole(ctx)`:
   - Solo si `ctx.path === "/sign-up/email"` y `body.accountType === "publisher"` devuelve `publisher`.
   - En cualquier otro caso (otro valor, `"admin"`, sin cuerpo, sin contexto de petición) devuelve `consumer`.
   - **Nunca** devuelve `admin`. Se ha comprobado de extremo a extremo que enviar `role: "admin"` o `accountType: "admin"` crea un `consumer`.

### Después del registro

Better Auth inicia la sesión automáticamente. Luego `/signup` llama a `clearApiToken()` (para no reutilizar el JWT de una sesión anterior) y:

- **Persona:** va a `?redirect=` (pasado por `safeRedirect`) o a `/`.
- **Empresa:** guarda la organización con `savePublisherProfile(toProfileInput(org))` y va a `/publisher`. Si falla (la cuenta ya existe), va a `/profile?completeOrg=1`, donde la sección Organización muestra un aviso destacado para terminarla.

En ambos casos termina con `router.refresh()`.

## Redirecciones tras el login

`lib/better-auth/redirect.ts`:

- `safeRedirect(target, fallback = "/")`: devuelve `target` solo si es una ruta del propio sitio (empieza por `/` pero no por `//` ni `/\`); si no, `fallback`.
- `homeForRole(role)`: `/publisher` para `publisher` y `/` para el resto.

`/login` llama a `clearApiToken()` tras iniciar sesión y redirige a `safeRedirect(?redirect, homeForRole(user.role))`: `?redirect=` tiene prioridad; sin él, los publicadores van a su panel y el resto a la portada.

## Páginas

### `Navbar`

- Enlaces: Descubrir (`/`), un enlace personal y Comunidad. El personal depende del rol:
  - `publisher` → "Mis publicaciones" (`/publisher`).
  - El resto (`consumer`, `admin` y sin sesión) → "Mis eventos" (`/my-events`).
- Un enlace está activo en su ruta y en sus subrutas (`isActivePath`).
- `admin` ve además el botón a `/admin`.
- El menú del usuario (`role="menu"`) tiene el correo, el enlace personal (solo en móvil, donde se ocultan los enlaces de navegación), "Mi perfil" (`/profile`) y "Cerrar sesión".

### `/profile` Mi perfil

`app/profile/page.tsx`, **Server Component**.

- Sin sesión o sin token → `/login?redirect=/profile`.
- Lee los datos de Better Auth de la sesión (nombre, correo, rol, fecha de alta) y el resto con `getMe(token)`. Si `getMe` responde `401`, redirige al login; si falla por otro motivo, muestra un aviso de error y oculta Organización y Notificaciones (Cuenta, Seguridad y Zona de peligro siguen disponibles).
- El rol se toma de la **sesión**: cualquier valor distinto de `admin` o `publisher` se trata como `consumer`.

Secciones, en orden:

| Sección | Quién la ve | Qué hace |
|---------|-------------|----------|
| **Cuenta** | Todos | Correo (solo lectura), insignia del tipo de cuenta, "miembro desde" y cambio de **nombre** con `authClient.updateUser({ name })`. Después llama a `clearApiToken()` para que el siguiente JWT lleve el nombre nuevo y `Provision` actualice `public.users`. El botón Guardar está desactivado si el nombre no cambia. |
| **Organización** | `publisher` | `OrganizationFields` + `savePublisherProfile`. Si no hay perfil guardado, muestra un aviso ("incompleto"); con `?completeOrg=1` el aviso es un `role="alert"` con el texto de "falló al registrarte". |
| **Notificaciones** | Todos (si `getMe` fue bien) | Interruptores de `NotificationPreferences` y, con "Novedades de mis categorías" activo, las categorías favoritas. "Actividad en mis eventos" (`publisherActivity`) solo se muestra a `publisher`; para el resto se reenvía el valor guardado. Un aviso indica que de momento **no se envía nada**. Guarda con `updateNotifications`. |
| **Seguridad** | Todos | Cambio de contraseña con `authClient.changePassword({ currentPassword, newPassword, revokeOtherSessions: true })`: cierra la sesión en el resto de dispositivos. Valida longitud mínima (8) y coincidencia. |
| **Zona de peligro** | Todos | Borrar la cuenta (ver abajo). |

Errores: los de la API de Go se traducen con `apiErrorMessage` (`401`, `403`, `400` o genérico); los de Better Auth con `authErrorMessage` (`INVALID_PASSWORD`, `PASSWORD_TOO_SHORT`, `PASSWORD_TOO_LONG`, `SESSION_EXPIRED`, y después por estado).

#### Borrar la cuenta

`DangerZone`:

- Lista lo que se borra: cuenta y sesiones, eventos guardados, preferencias y, si es `publisher` (en rojo), la organización y **todos sus eventos publicados**.
- Pide la contraseña y escribir la palabra de confirmación (`profile.danger.confirmWord`, sin distinguir mayúsculas). El botón está desactivado hasta entonces.
- Llama a `authClient.deleteUser({ password })` (`POST /api/auth/delete-user`, activado con `user.deleteUser.enabled` en `server.ts`). Better Auth verifica la contraseña, borra el usuario, sus sesiones y cuentas, y la cookie.
- Después `clearApiToken()` y `window.location.assign("/")` (recarga completa para que no sobreviva ningún estado del cliente).

### `/publisher` Mis publicaciones

`app/publisher/page.tsx` (Server Component) y `PublisherDashboard.tsx` (cliente).

- Sin sesión → `/login?redirect=/publisher`.
- Con rol distinto de `publisher` muestra una tarjeta en lugar del panel:
  - `admin`: "gestiona los eventos desde el panel de administración" y enlace a `/admin`.
  - `consumer`: explica qué es esta zona, con enlaces a `/profile` y `/my-events`.
- Con `publisher`, pide en paralelo `getMyEvents(token)` y `getMe(token)`:
  - Si falla `getMyEvents`, muestra el título y un error de carga.
  - Si `getMe` va bien pero no hay perfil de organización, muestra un aviso con enlace a `/profile`. Si `getMe` falla, el aviso no aparece.
  - El subtítulo usa `orgName`, o el nombre o el correo del usuario si no hay perfil.
- Separa los eventos con `hasEventEnded` (fin de la **última** sesión, hora de `Europe/Madrid`, calculada en el servidor para que no cambie al hidratar): **próximos** por su próxima sesión (`nextSession`), en orden ascendente, y **pasados** por el fin de su última sesión, en orden descendente.

`PublisherDashboard`:

- Botón "Nuevo evento" en la cabecera.
- Sin eventos: estado vacío con un botón para crear el primero.
- Con eventos: tres estadísticas (próximos, **guardados totales** = suma de `savesCount`, pasados), pestañas Próximos / Pasados con contador y la lista.
- Cada fila muestra fecha (en eventos de varios días, el rango y el resumen del horario), título, categoría, lugar, "Guardado por N personas" (`savesCount`) y acciones: ver (`/events/{id}?from=publisher`, así "Volver" regresa aquí), editar (`/publisher/events/{id}/edit`) y borrar.
- "Nuevo evento" lleva a `/publisher/events/new`. El editor vuelve con `?saved=created|updated`, que muestra el aviso correspondiente y se quita de la URL para que no se repita al recargar.
- Borrar pide confirmación con `confirm()`. Después, `router.refresh()` y un mensaje (`role="status"`). Errores: `401`, `403`, `400` (con el mensaje de la API), `404` (también refresca la lista) o genérico.

### Detalle del evento: "Organizado por"

En `/events/[id]` (ver [eventos.md](eventos.md#eventsid-detalle-del-evento)):

- Si el evento trae `organizer`, la barra lateral añade una tarjeta "Organizado por" con el nombre de la organización, la descripción, la web (solo se enlaza si es `http(s)`, mostrando el host) y el teléfono (enlace `tel:`).
- Si el evento tiene `publisherId` y coincide con el id del usuario de la sesión, aparece un aviso "Has publicado este evento" con enlace a `/publisher`. La sesión solo se consulta cuando el evento tiene `publisherId`.

### `/admin`: columna Propietario

- La pestaña Eventos enlaza al editor: "Nuevo evento" a `/publisher/events/new` y el lápiz de cada fila a `/publisher/events/{id}/edit` (un admin puede editar cualquier evento). Al terminar, el editor vuelve a `/admin?tab=events&saved=created|updated`, que abre la pestaña Eventos y muestra el aviso.
- La columna de fecha muestra el rango (`formatEventDateRange`) en los eventos de varios días.
- Columna **Propietario**: "Publicador `<8 primeros caracteres del id>`" (con el id completo en el `title`) si hay `publisherId`, o "Sin publicador" si no (eventos de scrapers o creados por un admin). No hay un endpoint para buscar el nombre del publicador.
- El editor no envía `publisherId`: un admin crea eventos sin propietario, y al editar se conserva el propietario que tuviera.

## Editor de eventos

Crear y editar eventos se hace en páginas completas: `/publisher/events/new` y `/publisher/events/[id]/edit` (el antiguo modal `EventForm` ya no existe). El formulario, sus secciones y el flujo de imágenes están descritos en [eventos.md](eventos.md#editor-de-eventos). Lo relativo a roles:

- `requireSession(returnTo)`: sin sesión → `/login?redirect=<ruta del editor>`.
- `editorHomeFor(role)`: `publisher` → vuelve a `/publisher`; `admin` → vuelve a `/admin?tab=events`; `consumer` → tarjeta "solo para publicadores" con enlace a `/`. Es un espejo de `auth.CanPublish`; la puerta real sigue siendo la API.
- Al editar, un `publisher` solo puede abrir sus eventos (si no, tarjeta "no es tuyo"); un `admin`, cualquiera. La API aplica la misma regla (`403`) a `PUT` y a las imágenes.
- La búsqueda de direcciones del editor (`GET /api/geocode/*`) también exige `CanPublish`.

## API

### `/api/me`

Todas las rutas requieren sesión (`Authenticate` → `Provision`) y actúan sobre el usuario del token.

| Método | Ruta | Quién | Cuerpo | Respuesta |
|--------|------|-------|--------|-----------|
| `GET` | `/api/me` | Cualquier rol | — | `200` `MeResponse`. `404` si no existe el perfil de Go, `500` si falla la lectura. |
| `PUT` | `/api/me/notifications` | Cualquier rol | `NotificationPreferences` completo | `200` con lo guardado. `400` cuerpo inválido o categoría desconocida, `404` usuario sin perfil de Go, `500`. |
| `GET` | `/api/me/publisher-profile` | `publisher` | — | `200` `PublisherProfile`. `404` si aún no lo ha creado, `403` otros roles. |
| `PUT` | `/api/me/publisher-profile` | `publisher` | `{ orgName, website?, phone?, description? }` | `200` con lo guardado (crea o reemplaza). `400` validación, `403` otros roles, `404` usuario sin perfil de Go, `500`. |
| `GET` | `/api/me/events` | `publisher` | — | `200` `{ events: PublisherEvent[], total }`: todos sus eventos (pasados y futuros) por hora de inicio, con `savesCount`. `403` otros roles. |

Sin token o con uno inválido, todas responden `401`.

`MeResponse`:

```ts
interface MeResponse {
  user: User;                                 // perfil de public.users
  publisherProfile: PublisherProfile | null;  // solo se busca para publisher; null si no lo tiene
  notifications: NotificationPreferences;     // valores por defecto si nunca las guardó
}
```

### Propiedad de los eventos

- `POST /api/events` (puerta `RequireCanPublish`):
  - `publisher`: el `publisherId` del cuerpo se ignora y se fuerza al `sub` del token.
  - `admin`: puede enviar `publisherId` para crear en nombre de otro usuario, u omitirlo para un evento sin propietario. Si no corresponde a ningún usuario (o no es un uuid) → `400 "publisherId does not match any user"`.
  - `organizer` del cuerpo siempre se ignora.
- `PUT /api/events/{id}` y `DELETE /api/events/{id}` (`RequireRole(admin, publisher)` + `loadOwned`):
  - Primero se carga el evento: `404` si no existe.
  - `admin` puede con cualquiera; `publisher` solo si `publisherId` es su `sub`. Los eventos sin propietario (scrapers) nunca son de un publicador. Si no → `403 "you can only manage your own events"`.
  - `PUT` es un **reemplazo completo** de los campos; el `id` y el propietario guardados se conservan siempre (el `publisherId` del cuerpo se ignora) y se actualiza `updated_at`.
- Imágenes (`POST /api/events/{id}/images`, `DELETE /api/events/{id}/images/{imageId}`, `PUT /api/events/{id}/images/order`): mismas reglas que `PUT` (`RequireRole(admin, publisher)` + `loadOwned`). Detalles en [eventos.md](eventos.md#imágenes-de-eventos).
- `DELETE /api/events/{id}` borra también los ficheros de las imágenes del evento, después de confirmar el borrado en la base de datos (si falla, solo se registra).
- `GET /api/events` y `GET /api/events/{id}` devuelven `publisherId` cuando lo hay. Solo `GET /api/events/{id}` añade `organizer` (`LEFT JOIN publisher_profiles`), y solo si el propietario tiene perfil de organización.

### Reglas de validación

Perfil de organización (`validatePublisherProfile` en `handlers/me.go`; `OrganizationFields` replica las mismas reglas en el cliente). Todos los campos se recortan (`TrimSpace`) y las longitudes se cuentan en caracteres:

| Campo | Regla |
|-------|-------|
| `orgName` | Obligatorio, entre 2 y 120 caracteres. |
| `website` | Opcional. Máximo 300 caracteres y URL absoluta `http` o `https` con host. |
| `phone` | Opcional. Máximo 30 caracteres; solo dígitos, espacios y `+ - ( ) .`. |
| `description` | Opcional. Máximo 1000 caracteres. |

Los opcionales vacíos se guardan como `NULL` y no aparecen en el JSON. Al actualizar se conserva `createdAt` y cambia `updatedAt` (RFC 3339, UTC).

Notificaciones (`normalizeNotifications`):

- El cuerpo es el conjunto **completo**: los booleanos que faltan valen `false`.
- Cada categoría debe ser una de `models.Categories` (si no, `400 unknown category "…"`). Las repetidas se eliminan conservando el orden de la primera aparición. `categories` nunca es `null` (se devuelve `[]`).

Eventos: ver la tabla de validación en [eventos.md](eventos.md#eventos-libapieventsts) (`sessions` obligatorias, título ≤ 200, descripción ≤ 5000, `perks` ≤ 20 × 80, lugar obligatorio, precio ≥ 0). En cada sesión, si `endTime` es anterior a `startTime` (por ejemplo 22:00–02:00) el fin se guarda al **día siguiente**. Si son iguales, es el mismo instante.

## Modelo de datos

Migración `backend/migrations/00003_publishers_and_notifications.sql`:

| Tabla / columna | Descripción |
|-----------------|-------------|
| `publisher_profiles` | `user_id` (PK, FK a `users(id)` `ON DELETE CASCADE`), `org_name` (`NOT NULL`), `website`, `phone`, `description`, `created_at`, `updated_at`. |
| `notification_preferences` | `user_id` (PK, FK a `users(id)` `ON DELETE CASCADE`), `saved_event_reminders` (por defecto `true`), `category_news` (`false`), `categories` (`TEXT[]`, `'{}'`), `newsletter` (`false`), `publisher_activity` (`true`), `updated_at`. Sin fila → la API devuelve los valores por defecto (`DefaultNotificationPreferences`, que coinciden con los de las columnas). |
| `events.publisher_id` | `UUID NULL`, FK a `users(id)` `ON DELETE CASCADE`, con índice `events_publisher_id_idx`. `NULL` para los eventos de scrapers. |
| `events.created_at`, `events.updated_at` | `TIMESTAMPTZ DEFAULT now()`. `Update` actualiza `updated_at`. |

El `Down` de la migración pierde la propiedad de los eventos, los perfiles de organización y las preferencias.

La migración `00004_event_sessions_and_images.sql` añade `event_sessions` y `event_images`, ambas con `event_id` FK a `events(id)` `ON DELETE CASCADE` (ver [eventos.md](eventos.md#modelo-de-datos)).

Tipos del frontend (`lib/types.ts`):

```ts
interface Organizer { id: string; orgName: string; website?: string; phone?: string; description?: string }

interface PublisherEvent extends Event { savesCount: number }

interface PublisherProfile {
  userId: string;
  orgName: string;
  website?: string;
  phone?: string;
  description?: string;
  createdAt: string; // RFC 3339
  updatedAt: string;
}
type PublisherProfileInput = Pick<PublisherProfile, "orgName" | "website" | "phone" | "description">;

interface NotificationPreferences {
  savedEventReminders: boolean; // recordatorios de eventos guardados (por defecto true)
  categoryNews: boolean;        // nuevos eventos de mis categorías (false)
  categories: EventCategory[];  // categorías favoritas para categoryNews
  newsletter: boolean;          // novedades de folixenda (false)
  publisherActivity: boolean;   // publishers: actividad en mis eventos (true)
}
```

## Borrado de cuenta en cascada

Una cuenta se borra de dos formas, ambas en Better Auth:

- **El propio usuario**, desde la Zona de peligro de `/profile` (`authClient.deleteUser({ password })`).
- **Un admin**, desde `/admin` con la Server Action `deleteUserAction` (`app/admin/actions.ts`, `internalAdapter.deleteUser`). No puede borrarse a sí mismo.

Al borrar la fila de `auth."user"`, Postgres borra en cascada:

```
auth."user"
  ├─ auth.session, auth.account                 (Better Auth)
  └─ public.users                                (perfil de Go)
       ├─ user_saved_events                      (sus eventos guardados)
       ├─ publisher_predefined_events
       ├─ publisher_profiles                     (su organización)
       ├─ notification_preferences               (sus preferencias)
       └─ events (publisher_id)                  (los eventos que publicó)
            ├─ user_saved_events (event_id)      (los guardados de otros usuarios sobre esos eventos)
            ├─ event_sessions                    (sus días)
            └─ event_images                      (las filas de sus imágenes)
```

La cascada solo borra **filas**: los ficheros de las imágenes (disco local o bucket S3) se quedan huérfanos. Solo `DELETE /api/events/{id}` los borra. Ver el `TODO` de `backend/handlers/event_images.go` y [Pendiente](#pendiente).

El JWT que ya tuviera el usuario sigue siendo válido hasta 15 minutos, pero sus peticiones fallan porque su perfil ya no existe (ver [autenticacion.md](autenticacion.md#pendiente)).

## Publicación de pago (futuro)

Publicar eventos será una función de pago para los publicadores. El código está preparado para que el cambio se haga en pocos sitios, marcados con comentarios `Future:`:

- **Go:** `auth.CanPublish` (`backend/auth/roles.go`) es la **única** puerta que decide quién puede publicar; `middleware.RequireCanPublish` la aplica a `POST /api/events`. Basta con añadir ahí la comprobación de suscripción (un claim o una consulta).
- **`lib/better-auth/server.ts`:** `signupRole` (se podría crear el publicador "bloqueado" hasta pagar, por ejemplo con un campo `publisherStatus`) y `user.deleteUser` (cancelar la suscripción en `beforeDelete`).
- **`app/signup/page.tsx`:** el paso de pago iría antes o justo después de guardar la organización.
- **`app/publisher/page.tsx`:** comprobar la suscripción y mostrar un aviso de "suscripción necesaria".
- **`app/publisher/events/access.tsx`:** mostrar una tarjeta de "suscripción necesaria" en lugar del editor.
- **`PublisherDashboard.tsx`:** bloquear el enlace "Nuevo evento".
- **`components/event-editor/EventEditor.tsx`:** tratar una respuesta `402` / "suscripción necesaria" al guardar.
- **`DangerZone.tsx`:** cancelar la suscripción antes de borrar la cuenta (en el servidor).

Ojo: `PUT` y `DELETE /api/events/{id}` y los endpoints de imágenes usan `RequireRole(admin, publisher)`, **no** `CanPublish`. Hoy da igual porque son los mismos roles, pero al añadir la suscripción habrá que decidir qué pasa con ellos (ver [Pendiente](#pendiente)).

## Pendiente

- **Pagos para publicadores:** no existen todavía (ver [Publicación de pago](#publicación-de-pago-futuro)). Cualquiera puede registrarse como `publisher` y publicar gratis.
- **Envío de notificaciones:** solo se guardan las preferencias. No se envía ningún correo ni aviso.
- **Cambio de correo:** no se puede cambiar el correo desde `/profile` (se muestra como solo lectura). Habría que activar `changeEmail` en Better Auth, con verificación del correo nuevo, y propagarlo a `public.users`.
- **Suscripción caducada y `PUT`/`DELETE`:** cuando la publicación sea de pago, hay que decidir si un publicador sin suscripción activa puede seguir editando o borrando sus eventos y sus imágenes. Hoy `PUT`, `DELETE` y las imágenes no pasan por `CanPublish`, así que podría.
- **Ficheros huérfanos al borrar una cuenta:** la cascada borra los eventos y las filas de `event_images` del publicador, pero no los ficheros del almacenamiento (ver [Borrado de cuenta en cascada](#borrado-de-cuenta-en-cascada)).
- **`publisherId` de un admin no se valida como publicador:** al crear, un admin puede asignar cualquier usuario existente como propietario (también un `consumer` o un `admin`); solo se comprueba la clave foránea. Ese usuario no podría gestionarlo (`owns` exige rol `publisher`) ni verlo en `/publisher`.
- **Quitar el rol `admin`:** al quitárselo a un admin se le puede asignar `consumer` o `publisher` libremente, porque no se guarda cuál era su tipo de cuenta original. Si se quiere ser estricto, habría que guardar el tipo de cuenta aparte del rol.
