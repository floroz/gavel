/**
 * Environment Configuration
 *
 * This module uses @t3-oss/env-nextjs for type-safe environment variables with:
 * - Runtime validation with Zod schemas
 * - Type safety and autocomplete
 * - No defaults in code - all values must be explicitly set
 *
 * Environment Sources:
 * - Local Development: Load from .env.local file (copy from .env.example)
 * - Kubernetes: Injected via Helm chart values (deploy/charts/frontend/values.yaml)
 *
 * Next.js automatically loads .env files in this order:
 * 1. .env.local (highest priority, git-ignored)
 * 2. .env.production / .env.development
 * 3. .env
 */

import { createEnv } from "@t3-oss/env-nextjs";
import { z } from "zod";

export const env = createEnv({
  /**
   * Server-side environment variables
   * Only accessible on the server (Server Components, Server Actions, API Routes)
   */
  server: {
    NODE_ENV: z
      .enum(["development", "production", "test"])
      .default("development"),
    AUTH_SERVICE_URL: z.url(),
    BID_SERVICE_URL: z.url(),
    USER_STATS_SERVICE_URL: z.url(),
    JWT_PUBLIC_KEY_PATH: z.string(),
    JWT_ISSUER: z.string(),
    SECURE_COOKIE: z.string(),
  },

  /**
   * Client-side environment variables
   * Must be prefixed with NEXT_PUBLIC_
   */
  client: {},

  /**
   * Runtime environment variables mapping
   * This tells t3-env which process.env variables to use
   */
  runtimeEnv: {
    NODE_ENV: process.env.NODE_ENV,
    AUTH_SERVICE_URL: process.env.AUTH_SERVICE_URL,
    BID_SERVICE_URL: process.env.BID_SERVICE_URL,
    USER_STATS_SERVICE_URL: process.env.USER_STATS_SERVICE_URL,
    JWT_PUBLIC_KEY_PATH: process.env.JWT_PUBLIC_KEY_PATH,
    JWT_ISSUER: process.env.JWT_ISSUER,
    SECURE_COOKIE: process.env.SECURE_COOKIE,
  },

  /**
   * Skip validation during build time
   * Useful for Docker builds where env vars are set at runtime
   */
  skipValidation: !!process.env.SKIP_ENV_VALIDATION,

  /**
   * Makes it so that empty strings are treated as undefined.
   * `SOME_VAR: z.string()` and `SOME_VAR=''` will throw an error.
   */
  emptyStringAsUndefined: true,
});

/**
 * Convenience exports for common checks
 */
export const isDevelopment = env.NODE_ENV === "development";
export const isProduction = env.NODE_ENV === "production";
export const isTest = env.NODE_ENV === "test";
