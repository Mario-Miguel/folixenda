import { betterAuth } from "better-auth";
import { jwt } from "better-auth/plugins";
import { nextCookies } from "better-auth/next-js";
import { Pool } from "pg";
import { hash, verify } from "@node-rs/argon2";
import type { GenericEndpointContext } from "better-auth";

// Argon2id with OWASP-recommended parameters. Hashes are standard PHC strings
// ($argon2id$v=19$m=19456,t=2,p=1$...) so they can be verified from Go later.
const ARGON2_OPTIONS = { memoryCost: 19456, timeCost: 2, parallelism: 1 };

// Roles a user can end up with by signing up. "admin" is never one of them:
// admins are promoted later from /admin or with SQL.
type SignupRole = "consumer" | "publisher";

// The sign-up form sends `accountType` next to name/email/password. Better Auth's
// /sign-up/email body schema accepts extra keys and ignores the ones that aren't
// user fields, so we read it here instead of letting the client set `role`.
function signupRole(ctx: GenericEndpointContext | null): SignupRole {
  if (ctx?.path !== "/sign-up/email") return "consumer";
  const body: unknown = ctx.body;
  const accountType =
    typeof body === "object" && body !== null ? (body as Record<string, unknown>).accountType : undefined;
  // Future: publishing will be paid. A publisher could be created here as usual and
  // only unlocked (e.g. a `publisherStatus` field) after checkout succeeds.
  return accountType === "publisher" ? "publisher" : "consumer";
}

export const auth = betterAuth({
  // Auth tables live in their own "auth" schema, apart from the app's tables
  database: new Pool({
    connectionString: process.env.DATABASE_URL,
    options: "-c search_path=auth",
  }),
  advanced: {
    database: {
      generateId: "uuid",
    },
  },
  emailAndPassword: {
    enabled: true,
    password: {
      hash: (password) => hash(password, ARGON2_OPTIONS),
      verify: ({ hash, password }) => verify(hash, password),
    },
  },
  user: {
    additionalFields: {
      role: {
        type: "string",
        defaultValue: "consumer",
        input: false, // clients can never set it: see databaseHooks.user.create.before
      },
    },
    // POST /api/auth/delete-user: requires the password (or a fresh session).
    // In Postgres, deleting auth."user" cascades to the Go profile (public.users),
    // saved events, publisher profile, notification preferences and the publisher's events.
    deleteUser: {
      enabled: true,
      // Future: cancel the publisher's paid subscription here (beforeDelete) once payments exist
    },
  },
  databaseHooks: {
    user: {
      create: {
        // Runs for every user Better Auth creates. Only /sign-up/email can ask for
        // "publisher"; anything else (or no request context at all) is a consumer.
        before: async (user, ctx) => ({ data: { ...user, role: signupRole(ctx) } }),
      },
    },
  },
  session: {
    expiresIn: 60 * 60 * 24 * 7, // 7 days
    updateAge: 60 * 60 * 24, // refresh expiry once a day
    cookieCache: {
      enabled: true,
      maxAge: 5 * 60,
      strategy: "jwt",
    },
  },
  plugins: [
    // Issues JWTs for the Go API (GET /api/auth/token) and publishes the
    // public keys to verify them at GET /api/auth/jwks
    jwt({
      sessionCookieCache: true,
      jwt: {
        expirationTime: "15m",
        definePayload: ({ user }) => ({
          email: user.email,
          name: user.name,
          role: user.role,
        }),
      },
    }),
    nextCookies(), // must be the last plugin
  ],
});

export type Session = typeof auth.$Infer.Session;
