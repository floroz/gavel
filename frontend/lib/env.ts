/**
 * Environment Configuration
 *
 * This module uses @t3-oss/env-nextjs for type-safe environment variables with:
 * - Runtime validation with Zod schemas
 * - Sensible defaults for local development
 * - Type safety and autocomplete
 *
 * Local Development Defaults:
 * - AUTH_SERVICE_URL: http://localhost:8080
 * - BID_SERVICE_URL: http://localhost:8081
 * - USER_STATS_SERVICE_URL: http://localhost:8082
 *
 * Production (K8s):
 * - Set via Helm chart environment variables
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
