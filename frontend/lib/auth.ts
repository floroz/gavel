/**
 * Authentication Utilities (SERVER-ONLY)
 *
 * This module provides session management and authentication helpers.
 * It handles token refresh automatically when access tokens expire.
 *
 * Usage:
 * - `getSession()`: Get current session (returns null if unauthenticated)
 * - `requireAuth()`: Get session or redirect to login (protected routes)
 */

import { headers } from "next/headers";
import { authClient } from "./rpc";
import { setAuthCookies, getAuthCookies, clearAuthCookies } from "./cookies";
import { verifyToken, type TokenClaims } from "./jwt";

export interface CurrentUser {
  userId: string;
  email: string;
  fullName: string;
  role: string;
  permissions: string[];
}

export interface Session {
  accessToken: string;
  claims?: TokenClaims;
  userId?: string;
  email?: string;
}

/**
 * Get the current user session
 * Automatically handles token refresh if access token is expired
 * Returns null if user is not authenticated
 */
export async function getSession(): Promise<Session | null> {
  const { accessToken, refreshToken } = await getAuthCookies();

  // No tokens at all - user is not authenticated
  if (!accessToken && !refreshToken) {
    return null;
  }

  // Access token exists - return it
  // Backend will validate the token's signature and expiry
  if (accessToken) {
    return { accessToken };
  }

  // Access token expired, but refresh token exists - try to refresh
  if (refreshToken) {
    try {
      const headersList = await headers();
      const ipAddress = headersList.get("x-forwarded-for") || "127.0.0.1";
      const userAgent = headersList.get("user-agent") || "unknown";

      const response = await authClient.refresh({
        refreshToken,
        ipAddress,
        userAgent,
      });

      // Update cookies with new tokens
      await setAuthCookies(response.accessToken, response.refreshToken);

      return { accessToken: response.accessToken };
    } catch (error: unknown) {
      if (error instanceof Error) {
        console.error("Refresh token error:", error.message);
      } else {
        console.error("Refresh token error:", error);
      }
      // Refresh failed (token revoked, expired, or invalid)
      // Clear cookies and return null
      await clearAuthCookies();
      return null;
    }
  }

  return null;
}

/**
 * Get user information from session
 * This is a helper for pages that want to display user info
 * Returns null if not authenticated
 */
export async function getCurrentUser() {
  const session = await getSession();

  if (!session) {
    return null;
  }

  try {
    // Decode JWT to get user information
    const claims = await verifyToken(session.accessToken);

    return {
      userId: claims.sub,
      email: claims.email,
      fullName: claims.fullName,
      role: claims.role,
      permissions: claims.permissions || [],
    } satisfies CurrentUser;
  } catch {
    // If token verification fails, return null
    // This can happen if the token is expired or invalid
    return null;
  }
}
