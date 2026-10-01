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
4. Go verifica el JWT con las claves públicas de `/api/auth/jwks` y obtiene el usuario del claim `sub`.

Así Next.js es el único que tiene credenciales de la base de datos de auth, y Go solo depende del contrato del JWT. Si en el futuro la autenticación se mueve a Go, basta con que Go emita tokens con el mismo formato.

## Ficheros

| Fichero | Descripción |
|---------|-------------|
| `lib/better-auth/server.ts` | Configuración de Better Auth (solo servidor). Exporta `auth` y el tipo `Session`. |
| `lib/better-auth/client.ts` | Cliente para React: `signIn`, `signUp`, `signOut`, `useSession`, y `getApiToken()` / `clearApiToken()` para el JWT. |
| `app/api/auth/[...all]/route.ts` | Expone todos los endpoints de Better Auth en `/api/auth/*`. |
| `lib/api/api.ts` | Cliente axios de la API de Go. Añade el JWT a cada petición hecha desde el navegador. |
| `app/login/page.tsx` | Página de inicio de sesión (correo y contraseña). |
| `app/signup/page.tsx` | Página de registro (nombre, correo, contraseña y confirmación). |
| `components/Navbar.tsx` | Muestra la inicial del correo del usuario y el menú para cerrar sesión, o un icono que lleva a `/login`. |
| `i18n/locales/{es,en}.json` | Textos de `login.*`, `signup.*`, `nav.login` y `nav.logout`. |

> `lib/auth.ts` (helpers de `localStorage`) y `lib/api/auth.ts` (login contra Go) son del sistema anterior y solo los usa todavía `/admin`. Ver [Pendiente](#pendiente).

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
- La tabla `users` del backend de Go es independiente y no tiene clave foránea hacia `auth."user"`. La sincronización está descrita en [Pendiente](#pendiente).

Setup inicial de la base de datos (ya hecho, como superusuario):

```sql
CREATE SCHEMA auth;
CREATE ROLE folixenda_auth LOGIN PASSWORD '...';
GRANT USAGE, CREATE ON SCHEMA auth TO folixenda_auth;
```

## Variables de entorno

En `frontend/.env` (ignorado por git):

| Variable | Ejemplo | Descripción |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | URL de la API de Go. Es pública (llega al navegador). |
| `BETTER_AUTH_SECRET` | `openssl rand -base64 32` | Secreto para firmar cookies y cifrar las claves privadas de `jwks`. Si cambia, se invalidan las sesiones. |
| `BETTER_AUTH_URL` | `http://localhost:3000` | URL base de la app. También es el `iss` y el `aud` de los JWT. |
| `DATABASE_URL` | `postgres://folixenda_auth:<password>@localhost:5432/folixenda` | Conexión a Postgres con el rol `folixenda_auth`. |

Solo las variables con prefijo `NEXT_PUBLIC_` se envían al navegador. **Nunca** añadas ese prefijo a `DATABASE_URL` ni a `BETTER_AUTH_SECRET`.

## Páginas y componentes

### `/login`

- Formulario de correo y contraseña que llama a `signIn.email()`.
- Si las credenciales son correctas, redirige a `/` y refresca la página para que se renderice con la sesión.
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

## JWT para la API de Go

### Cómo lo obtiene el frontend

`getApiToken()` en `lib/better-auth/client.ts`:

1. Si hay un token en memoria y le quedan más de 30 segundos, lo reutiliza.
2. Si no, llama a `GET /api/auth/token` (requiere la cookie de sesión) y guarda el token nuevo y su `exp`.
3. Si no hay sesión, devuelve `null` y la petición sale sin cabecera `Authorization`.

El interceptor de `lib/api/api.ts` lo usa en cada petición **hecha desde el navegador**. En Server Components no se envía todavía (ver [Pendiente](#pendiente)).

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

### Cómo debe verificarlo Go

- Claves públicas: `GET http://localhost:3000/api/auth/jwks` (formato JWKS, `kty: OKP`, `crv: Ed25519`). Guardarlas en caché y volver a pedirlas si llega un `kid` desconocido.
- Comprobar `alg = EdDSA`, `iss`, `aud` y `exp`.
- Identificar al usuario por `sub` y autorizar por `role`.

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

## Tareas habituales

**Crear un usuario sin la interfaz** (con `pnpm dev` en marcha):

```bash
curl -X POST http://localhost:3000/api/auth/sign-up/email \
  -H "Content-Type: application/json" \
  -d '{"email":"usuario@example.com","password":"minimo8caracteres","name":"Nombre"}'
```

**Dar rol de administrador:**

```sql
UPDATE auth."user" SET role = 'admin' WHERE email = 'usuario@example.com';
```

El cambio se refleja en el JWT en cuanto caduque el token en caché (15 minutos como máximo) y en la sesión cuando caduque la caché de cookie (5 minutos).

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

**Frontend:**

- `/admin` todavía usa el login antiguo contra Go (`lib/api/auth.ts`) y guarda el usuario en `localStorage` (`lib/auth.ts`). Hay que pasarlo a `useSession()` y comprobar `role === "admin"`; después se pueden borrar esos dos ficheros.
- Las páginas que llaman a la API desde el servidor (por ejemplo `/my-events`) no envían el JWT. Se obtiene con `auth.api.getToken({ headers: await headers() })`.
- `/my-events` usa el usuario fijo `ConsumerUser` de `data/users.ts`. Debe usar el usuario de la sesión, o mejor, rutas `/api/me/...` en las que Go saque el usuario del token.
- No hay protección de rutas: las páginas que requieren sesión deberían redirigir a `/login`.

**Backend (sincronización con Better Auth):**

1. Adaptar la tabla `users` de Go como tabla de perfil: `id` = UUID de Better Auth (`sub`), sin `password` y con `username` opcional.
2. Middleware que verifique el JWT con el JWKS y guarde `sub`, `email` y `role` en el `context`.
3. Crear o actualizar el perfil la primera vez que llega cada usuario (`INSERT … ON CONFLICT (id) DO UPDATE`).
4. Sacar el usuario del token en lugar de la URL (`/api/me/saved-events` en vez de `/api/user/{userId}/savedEvents`).
5. Eliminar `POST /api/auth/login` y el hashing con bcrypt de Go.
6. Decidir qué hacer con el perfil de Go cuando se borra un usuario en Better Auth (hook `databaseHooks.user.delete.after` o limpieza periódica).

## Otros cambios en el frontend

- **Dependencias nuevas:** `better-auth`, `pg`, `@node-rs/argon2` y `@types/pg` (dev).
- **`pnpm-workspace.yaml`:** `allowBuilds` permite ejecutar los scripts de instalación de `sharp` (optimización de imágenes de Next.js) y `unrs-resolver` (usado por ESLint). pnpm 10+ los bloquea por defecto.
- **Tipos de eventos de formulario:** `FormEvent` está obsoleto en los tipos de React 19; los formularios nuevos usan `SubmitEvent<HTMLFormElement>`.
