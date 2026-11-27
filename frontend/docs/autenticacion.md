# Autenticación en el frontend

Esta guía describe cómo funciona la autenticación de usuarios en el frontend de folixenda: registro, login, sesión y cómo se identifica al usuario ante la API de Go.

La autenticación se gestiona con [Better Auth](https://better-auth.com), que se ejecuta **en el servidor de Next.js** (nunca en el navegador). La API de Go no conoce Better Auth: solo recibe un JWT y lo verifica con una clave pública.

## Índice

- [Arquitectura](#arquitectura)
- [Ficheros](#ficheros)
- [Configuración de Better Auth](#configuración-de-better-auth)
- [Base de datos](#base-de-datos)
- [Variables de entorno](#variables-de-entorno)
- [Páginas y componentes](#páginas-y-componentes)
- [JWT para la API de Go](#jwt-para-la-api-de-go)
- [Permisos por rol](#permisos-por-rol)
- [Puesta en marcha](#puesta-en-marcha)
- [Tareas habituales](#tareas-habituales)
- [Pendiente](#pendiente)
- [Otros cambios en el frontend](#otros-cambios-en-el-frontend)

## Arquitectura

```
Navegador ──(cookie de sesión)──▶ Next.js /api/auth/*  ──▶ Postgres, schema "auth"
    │                                   │
    │                                   └─ GET /api/auth/jwks (claves públicas)
    │                                                 ▲
    └──(Authorization: Bearer <JWT>)──▶ API de Go ────┘ verifica el JWT
```

1. El usuario se registra o inicia sesión en `/signup` o `/login`. Better Auth valida las credenciales contra el schema `auth` de Postgres y devuelve una **cookie de sesión HttpOnly** (7 días).
2. El resto del frontend lee la sesión con `useSession()`.
3. Cuando el navegador llama a la API de Go, un interceptor de axios pide a Better Auth un **JWT de corta duración** (15 minutos) y lo envía en la cabecera `Authorization`.
4. Go verifica el JWT con las claves públicas de `/api/auth/jwks`, obtiene el usuario del claim `sub` y autoriza según el claim `role` (ver [Permisos por rol](#permisos-por-rol)).
5. En cada petición autenticada, Go crea o actualiza el perfil del usuario en `public.users` con los datos del token.

Así Next.js es el único que tiene credenciales de la base de datos de auth, y Go solo depende del contrato del JWT. Si en el futuro la autenticación se mueve a Go, basta con que Go emita tokens con el mismo formato.

## Ficheros

| Fichero | Descripción |
|---------|-------------|
| `lib/better-auth/server.ts` | Configuración de Better Auth (solo servidor). Exporta `auth` y el tipo `Session`. |
| `lib/better-auth/client.ts` | Cliente para React: `signIn`, `signUp`, `signOut`, `useSession`, y `getApiToken()` / `clearApiToken()` para el JWT. |
| `lib/better-auth/server-token.ts` | `getServerApiToken()`: obtiene el JWT en el servidor a partir de las cookies de la petición (`null` si no hay sesión). |
| `app/api/auth/[...all]/route.ts` | Expone todos los endpoints de Better Auth en `/api/auth/*`. |
| `lib/api/api.ts` | Cliente axios de la API de Go. Añade el JWT a cada petición hecha desde el navegador. |
| `lib/api/users.ts` | `getUsers`, `getUser`, `updateUser` y `deleteUser` contra `/api/users`. No hay creación: las cuentas se crean con el registro de Better Auth. |
| `app/login/page.tsx` | Página de inicio de sesión (correo y contraseña). |
| `app/signup/page.tsx` | Página de registro (nombre, correo, contraseña y confirmación). |
| `app/admin/page.tsx` | Server Component que comprueba la sesión y el rol antes de mostrar el panel. |
| `app/admin/AdminDashboard.tsx` | Panel de administración (cliente): pestañas de usuarios y eventos. |
| `components/Navbar.tsx` | Muestra la inicial del correo del usuario y el menú para cerrar sesión, o un icono que lleva a `/login`. |
| `i18n/locales/{es,en}.json` | Textos de `login.*`, `signup.*`, `admin.*`, `nav.login` y `nav.logout`. |

En el backend de Go:

| Fichero | Descripción |
|---------|-------------|
| `auth/verifier.go` | Verifica los JWT con el JWKS de Better Auth. |
| `auth/claims.go`, `auth/context.go` | Claims del token (`email`, `name`, `role` y los registrados) y cómo se guardan en el `context`. |
| `auth/roles.go` | Constantes `RoleAdmin`, `RoleConsumer`, `RolePublisher`, `ValidRole()` y `Claims.IsAdmin()`. |
| `middleware/authenticate.go` | `Authenticate`: lee `Authorization: Bearer <jwt>`, lo verifica y guarda los claims. Sin token o con uno inválido → `401`. |
| `middleware/provision.go` | `Provision`: crea o actualiza el perfil en `public.users`. |
| `middleware/roles.go` | `RequireRole` y `RequireSelfOrRole` (ver [Permisos por rol](#permisos-por-rol)). |
| `handlers/*.go` | Cada `Register` recibe `protect` (`Authenticate` → `Provision`) y añade encima las comprobaciones de rol. |

El sistema anterior (login contra Go, `lib/auth.ts` con `localStorage`, `lib/api/auth.ts` y el hashing con bcrypt en Go) ya se ha eliminado.

## Configuración de Better Auth

Todo está en `lib/better-auth/server.ts`:

| Opción | Valor | Motivo |
|--------|-------|--------|
| `database` | `pg.Pool` con `search_path=auth` | Las tablas de auth viven en su propio schema, separadas de las de negocio. |
| `advanced.database.generateId` | `"uuid"` | IDs estables y estándar; son el `sub` del JWT y lo que Go usará como id de usuario. |
| `emailAndPassword` | Activado, hash con **argon2id** | Better Auth usa scrypt por defecto. Con argon2id (formato PHC estándar `$argon2id$v=19$m=19456,t=2,p=1$…`) las contraseñas se pueden verificar desde Go si se migra la auth, sin obligar a los usuarios a cambiarla. |
| `user.additionalFields.role` | `string`, por defecto `"consumer"`, `input: false` | Rol del usuario (`admin`, `consumer`, `publisher`). El usuario no puede elegirlo al registrarse. |
| `session.expiresIn` / `updateAge` | 7 días / 1 día | La sesión dura 7 días y se renueva como mucho una vez al día. |
| `session.cookieCache` | 5 minutos, estrategia `jwt` | Evita consultar la base de datos en cada lectura de la sesión. |
| Plugin `jwt` | EdDSA (Ed25519), 15 minutos | Emite los JWT para Go. Payload: `sub`, `email`, `name`, `role`, `iss`, `aud`, `iat`, `exp`. |
| Plugin `nextCookies` | Último plugin | Necesario para que las Server Actions puedan fijar cookies. |

Parámetros de argon2id: `m=19456` (19 MiB), `t=2`, `p=1`, que son los recomendados por OWASP.

## Base de datos

Better Auth usa la misma base de datos `folixenda` que el backend, pero:

- Sus tablas están en el schema **`auth`**: `user`, `session`, `account`, `verification` y `jwks`.
- Se conecta con un rol propio, **`folixenda_auth`**, que solo tiene `USAGE` y `CREATE` sobre el schema `auth`. No puede leer ni modificar las tablas de la aplicación (`events`, `users`, `user_saved_events`…).

Detalles a tener en cuenta:

- Las contraseñas **no** están en `auth."user"`, sino en `auth.account` (fila con `providerId = 'credential'`).
- `auth.jwks` guarda los pares de claves con los que se firman los JWT. La clave privada está cifrada con `BETTER_AUTH_SECRET`.

### Perfil de Go (`public.users`)

- `public.users` es la tabla de **perfil** del usuario en Go. Su `id` es `uuid` (el mismo que `auth."user".id` y que el `sub` del JWT) y es clave foránea hacia `auth."user"(id)` con `ON DELETE CASCADE`.
- `user_saved_events.user_id` apunta a `public.users(id)` y `event_id` a `events(id)`, ambos con `ON DELETE CASCADE`. Borrar un usuario de Better Auth borra su perfil y sus eventos guardados; borrar un evento borra los guardados que lo referencian.
- Las filas las crea el middleware `Provision` la primera vez que el usuario llama a un endpoint protegido. Un usuario que se ha registrado pero nunca ha llamado a la API de Go no aparece en `public.users` (ni, por tanto, en `/admin`).
- `PUT /api/users/{id}` actualiza `public.users` y también `role` y `name` de `auth."user"` en **una misma transacción**. Better Auth es el origen de los claims del JWT, así que sin esa segunda escritura el cambio no llegaría nunca al token (y `Provision` lo desharía en el siguiente upsert).

Para eso, el rol con el que se conecta Go necesita estos permisos sobre el schema `auth`:

```sql
GRANT USAGE ON SCHEMA auth TO <rol de Go>;
GRANT SELECT, REFERENCES, UPDATE (role, name, "updatedAt") ON auth."user" TO <rol de Go>;
```

### Orden de creación

1. `db_queries/better_auth.sql` (local, no versionado), como superusuario: crea el schema `auth` y el rol `folixenda_auth`.

   ```sql
   CREATE SCHEMA auth;
   CREATE ROLE folixenda_auth LOGIN PASSWORD '...';
   GRANT USAGE, CREATE ON SCHEMA auth TO folixenda_auth;
   ```

2. Migraciones de Better Auth (ver [Puesta en marcha](#puesta-en-marcha)): crean `auth."user"` y el resto de tablas.
3. Migraciones de Go, en `backend/migrations/` (con [goose](https://github.com/pressly/goose), embebidas en el binario). El servidor aplica las pendientes al arrancar; también se pueden lanzar a mano desde `backend/` con `go run ./cmd/migrate` (`up`, `status` o `down`). Las versiones aplicadas se guardan en la tabla `goose_db_version`.
   - `00001_initial_schema.sql`: crea `events`, `users`, `publisher_predefined_events` y `user_saved_events` (si no existen). Necesita que exista `auth."user"` por la clave foránea.
   - `00002_user_cascade.sql`: para bases de datos creadas antes de las migraciones. Convierte los `id` de usuario a `uuid` y rehace las claves foráneas con `ON DELETE CASCADE`. **Borra** las filas de `public.users` (y sus eventos guardados) que no tengan un usuario en `auth."user"`, porque si existen no se puede crear la clave foránea. Es irreversible.

   Para cambiar el esquema, añade un fichero nuevo `0000N_descripcion.sql` con `-- +goose Up` / `-- +goose Down`; no edites migraciones ya aplicadas.

## Variables de entorno

En `frontend/.env` (ignorado por git):

| Variable | Ejemplo | Descripción |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | URL de la API de Go. Es pública (llega al navegador). |
| `BETTER_AUTH_SECRET` | `openssl rand -base64 32` | Secreto para firmar cookies y cifrar las claves privadas de `jwks`. Si cambia, se invalidan las sesiones. |
| `BETTER_AUTH_URL` | `http://localhost:3000` | URL base de la app. También es el `iss` y el `aud` de los JWT. |
| `DATABASE_URL` | `postgres://folixenda_auth:<password>@localhost:5432/folixenda` | Conexión a Postgres con el rol `folixenda_auth`. |

Solo las variables con prefijo `NEXT_PUBLIC_` se envían al navegador. **Nunca** añadas ese prefijo a `DATABASE_URL` ni a `BETTER_AUTH_SECRET`.

En `backend/.env`, además de la conexión a Postgres (`POSTGRES_*`) y `PORT`:

| Variable | Ejemplo | Descripción |
|----------|---------|-------------|
| `AUTH_JWKS_URL` | `http://localhost:3000/api/auth/jwks` | Claves públicas para verificar los JWT. |
| `AUTH_ISSUER` | `http://localhost:3000` | `iss` esperado. Debe coincidir con `BETTER_AUTH_URL`. |
| `AUTH_AUDIENCE` | `http://localhost:3000` | `aud` esperado. Debe coincidir con `BETTER_AUTH_URL`. |

## Páginas y componentes

### `/login`

- Formulario de correo y contraseña que llama a `signIn.email()`.
- Si las credenciales son correctas, redirige a la ruta de `?redirect=` (o a `/` si no hay) y refresca la página para que se renderice con la sesión. `safeRedirect()` solo acepta rutas del propio sitio (empiezan por `/` pero no por `//` ni `/\`), para que el parámetro no pueda llevar a otro dominio.
- Las páginas protegidas envían aquí con `?redirect=`: `/my-events` y `/admin` al no haber sesión, y `SaveEventButton` cuando la API responde `401`.
- Errores: `401` → "Correo o contraseña incorrectos"; cualquier otro → error genérico.
- Enlace "¿No tienes cuenta? Crear cuenta" a `/signup`.

### `/signup`

- Formulario de nombre, correo, contraseña y repetición de contraseña que llama a `signUp.email()`.
- Validación en el cliente: las contraseñas deben coincidir y tener al menos 8 caracteres (`MIN_PASSWORD_LENGTH`, que debe coincidir con `minPasswordLength` de Better Auth, 8 por defecto).
- Better Auth inicia la sesión automáticamente tras el registro, así que redirige a `/`.
- Los usuarios nuevos tienen rol `consumer`.
- Errores traducidos según el código de Better Auth:

  | Código | Mensaje |
  |--------|---------|
  | `USER_ALREADY_EXISTS`, `USER_ALREADY_EXISTS_USE_ANOTHER_EMAIL` | Ya existe una cuenta con ese correo |
  | `PASSWORD_TOO_SHORT` | La contraseña es demasiado corta |
  | `INVALID_EMAIL` | El correo no es válido |
  | Otro | Error genérico |

### `Navbar`

- Lee la sesión con `useSession()`.
- **Sin sesión:** icono de usuario que enlaza a `/login`.
- **Con sesión:** círculo con la primera letra del correo en mayúscula. Al pulsarlo se abre un menú con el correo y "Cerrar sesión", que llama a `signOut()`, borra el JWT en caché y redirige a `/`.
- Si el rol es `admin`, muestra el enlace a `/admin`.

### `/admin`

`app/admin/page.tsx` es un **Server Component** que lee la sesión con `auth.api.getSession()`:

- **Sin sesión:** redirige a `/login?redirect=/admin`.
- **Con sesión pero sin rol `admin`:** muestra una tarjeta de "Acceso de administrador" con el mensaje de que la cuenta no tiene acceso y un enlace a `/`.
- **Con rol `admin`:** renderiza `AdminDashboard` (cliente) con el nombre (o el correo) del usuario.

`AdminDashboard` tiene dos pestañas:

- **Usuarios:** lista de `GET /api/users`. Se puede editar el **nombre** y el **rol** (el correo se muestra pero no se puede cambiar, porque es el login de Better Auth) y eliminar. No hay creación de usuarios: las cuentas se crean con `/signup`.
- **Eliminar usuario:** el botón llama a la Server Action `deleteUserAction` (`app/admin/actions.ts`), no a la API de Go. Comprueba en el servidor que la sesión es de un `admin` y que no se borra a sí mismo (el botón de su propia fila está desactivado), y borra el usuario con `internalAdapter.deleteUser` de Better Auth (usuario, sesiones y cuentas). El perfil de `public.users` y sus eventos guardados se borran solos por `ON DELETE CASCADE`. El JWT que el usuario ya tuviera sigue siendo válido hasta 15 minutos, pero sus peticiones fallan porque su perfil ya no existe.
- **Eventos:** lista, creación, edición y borrado de eventos.

Los errores `403` muestran "No tienes permiso para realizar esta acción" y los `401` "Tu sesión ha caducado. Vuelve a iniciar sesión" (`admin.errors.*`); el resto, un mensaje genérico de cada acción. "Cerrar sesión" llama a `signOut()`, borra el JWT en caché y vuelve a `/`.

## JWT para la API de Go

### Cómo lo obtiene el frontend

`getApiToken()` en `lib/better-auth/client.ts`:

1. Si hay un token en memoria y le quedan más de 30 segundos, lo reutiliza.
2. Si no, llama a `GET /api/auth/token` (requiere la cookie de sesión) y guarda el token nuevo y su `exp`.
3. Si no hay sesión, devuelve `null` y la petición sale sin cabecera `Authorization`.

El interceptor de `lib/api/api.ts` lo usa en cada petición **hecha desde el navegador**. En el servidor el interceptor no hace nada: los Server Components obtienen el token con `getServerApiToken()` (`lib/better-auth/server-token.ts`, que llama a `auth.api.getToken({ headers: await headers() })`) y lo pasan a la función de la API, por ejemplo `getSavedEvents(token)` o `isEventSaved(id, token)`.

### Formato del token

Cabecera:

```json
{ "alg": "EdDSA", "kid": "<id de la clave en auth.jwks>" }
```

Payload:

```json
{
  "sub": "b007fe88-fbcf-4131-9e7e-4b934b8dcf1b",
  "email": "usuario@example.com",
  "name": "Nombre",
  "role": "consumer",
  "iss": "http://localhost:3000",
  "aud": "http://localhost:3000",
  "iat": 1790763574,
  "exp": 1790764474
}
```

### Cómo lo verifica Go

- `auth.NewVerifier` descarga las claves públicas de `AUTH_JWKS_URL` (formato JWKS, `kty: OKP`, `crv: Ed25519`) con `keyfunc`, que las guarda en caché y las vuelve a pedir si llega un `kid` desconocido.
- `Verify` solo acepta `alg = EdDSA` (nunca `none` ni `HS256`) y exige `iss = AUTH_ISSUER`, `aud = AUTH_AUDIENCE` y `exp`.
- `Authenticate` responde `401` si falta la cabecera `Authorization: Bearer <jwt>` o el token no es válido.
- `Provision` crea o actualiza el perfil (`id`, `email`, `name`, `role`) en `public.users` con `INSERT … ON CONFLICT (id) DO UPDATE`. Guarda en memoria el último perfil escrito para cada `sub` y vuelve a hacer el upsert cuando cambian el `email`, el `name` o el `role` del token, así que un cambio de rol se refleja en cuanto llega un token nuevo.
- La autorización usa siempre el `role` **del token**, no el de `public.users`.

## Permisos por rol

Roles: `admin`, `publisher` y `consumer` (por defecto al registrarse). Se comprueban en Go con los middlewares de `middleware/roles.go`, después de `Authenticate` y `Provision`:

- `RequireRole(roles...)`: sin claims → `401`; rol no incluido → `403`.
- `RequireSelfOrRole("id", roles...)`: deja pasar si el `{id}` de la ruta es el `sub` del token o si el rol está incluido; si no, `403`.

| Endpoint | Quién puede |
|----------|-------------|
| `GET /api/events`, `GET /api/events/{id}` | Cualquiera, sin sesión |
| `POST /api/events`, `PUT /api/events/{id}`, `DELETE /api/events/{id}` | `admin` o `publisher` |
| `GET /api/users` | `admin` |
| `GET /api/users/{id}`, `PUT /api/users/{id}` | `admin`, o el propio usuario. Si no es `admin`, el campo `role` del cuerpo se ignora |
| `GET /api/savedEvents`, `GET /api/savedEvents/{eventId}`, `POST /api/savedEvents`, `DELETE /api/savedEvents/{eventId}` | Cualquier usuario con sesión. El usuario sale siempre del token, no de la URL ni del cuerpo |

No hay `DELETE /api/users/{id}` (responde `405`): los usuarios se borran en Better Auth desde `/admin` y el perfil de Go se borra en cascada.

Detalles de `PUT /api/users/{id}`:

- Es una actualización parcial: los campos que vienen en el JSON sobrescriben el perfil y los que faltan se mantienen.
- `id` y `email` pertenecen a Better Auth y nunca se cambian.
- Si lo hace un `admin`, el `role` debe ser `admin`, `consumer` o `publisher` (si no, `400`).
- El cambio de `role` y `name` se escribe también en `auth."user"` (ver [Perfil de Go](#perfil-de-go-publicusers)), pero llega al JWT del usuario afectado con retraso: como mucho 5 minutos de caché de cookie más 15 minutos de vida del JWT en caché. Hasta entonces Go sigue autorizando con el rol anterior.

En el frontend, `/admin` solo es accesible para `admin` (ver [`/admin`](#admin)).

## Puesta en marcha

Desde `frontend/`:

```bash
pnpm install
# Rellenar .env (ver "Variables de entorno")
pnpm dlx auth migrate --config lib/better-auth/server.ts -y   # crea las tablas en el schema auth
pnpm dev
```

Sin `-y`, `auth migrate` pide confirmación de forma interactiva y, si no se responde, no crea nada. El aviso `ERROR [Better Auth]: Database schema mismatch` que aparece antes de migrar solo indica que faltan tablas.

Para ver el SQL sin aplicarlo:

```bash
pnpm dlx auth generate --config lib/better-auth/server.ts --output auth.sql -y
```

Hay que volver a migrar cada vez que se cambie algo que afecte al esquema: campos adicionales, plugins nuevos, etc.

Las tablas de Go dependen de `auth."user"`, así que hay que migrar Better Auth **antes** de crear las tablas de la aplicación o arrancar el servidor de Go (ver [Orden de creación](#orden-de-creación)).

## Tareas habituales

**Crear un usuario sin la interfaz** (con `pnpm dev` en marcha):

```bash
curl -X POST http://localhost:3000/api/auth/sign-up/email \
  -H "Content-Type: application/json" \
  -d '{"email":"usuario@example.com","password":"minimo8caracteres","name":"Nombre"}'
```

**Dar rol de administrador** (el primero hay que darlo así; después se puede hacer desde `/admin`):

```sql
UPDATE auth."user" SET role = 'admin' WHERE email = 'usuario@example.com';
```

El cambio llega a la sesión cuando caduca la caché de cookie (5 minutos) y al JWT cuando caduca el token en caché (15 minutos como máximo). `public.users` se actualiza solo en la siguiente petición a Go con el token nuevo (`Provision`).

**Leer la sesión en un componente cliente:**

```tsx
import { useSession } from "@/lib/better-auth/client";

const { data: session, isPending } = useSession();
session?.user.email; // también: id, name, role…
```

**Leer la sesión en el servidor** (Server Component, Route Handler o Server Action):

```ts
import { headers } from "next/headers";
import { auth } from "@/lib/better-auth/server";

const session = await auth.api.getSession({ headers: await headers() });
```

**Endpoints útiles de Better Auth** (bajo `/api/auth`):

| Método | Ruta | Uso |
|--------|------|-----|
| `POST` | `/sign-up/email` | Registro |
| `POST` | `/sign-in/email` | Login |
| `POST` | `/sign-out` | Cerrar sesión |
| `GET` | `/get-session` | Sesión actual |
| `GET` | `/token` | JWT para la API de Go |
| `GET` | `/jwks` | Claves públicas para verificar los JWT |

## Pendiente

- **Caché de `Provision`:** no se limpia al borrar un usuario. Durante los 15 minutos que le queden al JWT del usuario borrado, Go no vuelve a hacer el upsert y sus peticiones fallan (por ejemplo, guardar un evento da `404` por la clave foránea). Si Go se reinicia en ese rato, el upsert falla con `500`. En ambos casos no se recrea nada.
- **Rol antiguo tras reiniciar Go:** la caché de `Provision` está en memoria. Si Go se reinicia mientras un usuario aún tiene un JWT emitido antes de un cambio de rol, el primer upsert escribe el rol antiguo en `public.users` hasta que llegue un token nuevo. La autorización no se ve afectada (usa el rol del token), pero `/admin` puede mostrar el rol antiguo durante ese rato.
- **`better_auth.sql` sin versionar:** sigue en `db_queries/` (ignorado por git), porque crea un rol con contraseña. Las migraciones de Go sí están versionadas en `backend/migrations/`.

## Otros cambios en el frontend

- **Dependencias nuevas:** `better-auth`, `pg`, `@node-rs/argon2` y `@types/pg` (dev).
- **`pnpm-workspace.yaml`:** `allowBuilds` permite ejecutar los scripts de instalación de `sharp` (optimización de imágenes de Next.js) y `unrs-resolver` (usado por ESLint). pnpm 10+ los bloquea por defecto.
- **Tipos de eventos de formulario:** `FormEvent` está obsoleto en los tipos de React 19; los formularios nuevos usan `SubmitEvent<HTMLFormElement>`. `AdminDashboard` todavía usa `FormEvent`.
