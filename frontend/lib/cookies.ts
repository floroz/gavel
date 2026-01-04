/**
 * Cookie Management Utilities (SERVER-ONLY)
 *
 * This module provides secure cookie management for authentication tokens.
 * All cookies use HttpOnly, Secure (in production), and SameSite=Strict.
 *
 * Security features:
 * - HttpOnly: Prevents JavaScript access (XSS protection)
 * - Secure: HTTPS-only in production (MITM protection)
 * - SameSite=Strict: Prevents CSRF attacks
 * - Path=/: Accessible across entire application
 */

import { cookies } from "next/headers";
import { isProduction } from "./env";

const COOKIE_OPTIONS = {
  httpOnly: true,
  secure: isProduction,
  sameSite: "strict" as const,
  path: "/",
} as const;

/**
 * Set authentication cookies after successful login/register
 */
export async function setAuthCookies(
  accessToken: string,
  refreshToken: string,
) {
  const cookieStore = await cookies();

  cookieStore.set("access_token", accessToken, {
    ...COOKIE_OPTIONS,
    maxAge: 15 * 60, // 15 minutes
  });

  cookieStore.set("refresh_token", refreshToken, {
    ...COOKIE_OPTIONS,
    maxAge: 7 * 24 * 60 * 60, // 7 days
  });
}

/**
 * Get authentication cookies
 * Returns undefined for missing cookies
 */
export async function getAuthCookies() {
  const cookieStore = await cookies();
  return {
    accessToken: cookieStore.get("access_token")?.value,
    refreshToken: cookieStore.get("refresh_token")?.value,
  };
}

/**
 * Clear all authentication cookies (logout)
 */
export async function clearAuthCookies() {
  const cookieStore = await cookies();
  cookieStore.delete("access_token");
  cookieStore.delete("refresh_token");
}
