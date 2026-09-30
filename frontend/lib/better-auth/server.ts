import { betterAuth } from "better-auth";
import { jwt } from "better-auth/plugins";
import { nextCookies } from "better-auth/next-js";
import { Pool } from "pg";
import { hash, verify } from "@node-rs/argon2";

// Argon2id with OWASP-recommended parameters. Hashes are standard PHC strings
// ($argon2id$v=19$m=19456,t=2,p=1$...) so they can be verified from Go later.
const ARGON2_OPTIONS = { memoryCost: 19456, timeCost: 2, parallelism: 1 };

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
        input: false, // users can't choose their own role when signing up
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
