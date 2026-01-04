/**
 * Auth Server Actions
 *
 * All actions set/clear HttpOnly cookies and never expose tokens to the browser.
 */

"use server";

import { headers, cookies } from "next/headers";
import { redirect } from "next/navigation";
import { ConnectError } from "@connectrpc/connect";
import { authClient } from "@/lib/rpc";
import { setAuthCookies, clearAuthCookies } from "@/lib/cookies";
import {
  loginInputSchema,
  registerInputSchema,
  type LoginInput,
  type RegisterInput,
} from "@/shared/api/auth";
import { type ActionResult } from "@/shared/types";

/**
 * Login Action
 * Called from the login form
 */
export async function loginAction(
  input: LoginInput,
): Promise<ActionResult<{ userId: string }>> {
  // Validate input
  const parsed = loginInputSchema.safeParse(input);

  if (!parsed.success) {
    return {
      success: false,
      error: "Invalid email or password format",
    };
  }

  try {
    // Get request metadata
    const headersList = await headers();
    const ipAddress = headersList.get("x-forwarded-for") ?? "unknown";
    const userAgent = headersList.get("user-agent") ?? "unknown";

    // Call auth service
    const response = await authClient.login({
      email: parsed.data.email,
      password: parsed.data.password,
      ipAddress,
      userAgent,
    });

    // Set HttpOnly cookies (tokens never returned to browser)
    await setAuthCookies(response.accessToken, response.refreshToken);
  } catch (error) {
    // gRPC errors are thrown by ConnectRPC
    if (error instanceof ConnectError) {
      return {
        success: false,
        error: error.message,
      };
    }

    // Fallback for other errors
    return {
      success: false,
      error: "Login failed. Please try again.",
    };
  }

  // Redirect to dashboard (must be outside try/catch)
  redirect("/dashboard");
}

/**
 * Register Action
 * Called from the registration form
 */
export async function registerAction(
  input: RegisterInput,
): Promise<ActionResult<{ userId: string }>> {
  // Validate input
  const parsed = registerInputSchema.safeParse(input);

  if (!parsed.success) {
    return {
      success: false,
      error: "Invalid registration data",
    };
  }

  try {
    // Get request metadata
    const headersList = await headers();
    const ipAddress = headersList.get("x-forwarded-for") ?? "unknown";
    const userAgent = headersList.get("user-agent") ?? "unknown";

    // Call auth service to register
    await authClient.register({
      email: parsed.data.email,
      password: parsed.data.password,
      fullName: parsed.data.fullName,
      countryCode: parsed.data.countryCode,
      phoneNumber: parsed.data.phoneNumber,
    });

    // Auto-login after registration
    const loginResponse = await authClient.login({
      email: parsed.data.email,
      password: parsed.data.password,
      ipAddress,
      userAgent,
    });

    // Set HttpOnly cookies
    await setAuthCookies(loginResponse.accessToken, loginResponse.refreshToken);
  } catch (error) {
    if (error instanceof ConnectError) {
      return {
        success: false,
        error: error.message,
      };
    }

    return {
      success: false,
      error: "Registration failed. Email may already be in use.",
    };
  }

  // Redirect to dashboard (must be outside try/catch)
  redirect("/dashboard");
}

/**
 * Logout Action
 * Revokes refresh token on backend and clears cookies
 */
export async function logoutAction(): Promise<ActionResult> {
  try {
    // Get refresh token to revoke it
    const cookieStore = await cookies();
    const refreshToken = cookieStore.get("refresh_token")?.value;

    // Revoke refresh token on backend (if exists)
    if (refreshToken) {
      try {
        await authClient.logout({ refreshToken });
      } catch {
        // If revoke fails, still clear local cookies
        // This ensures user can logout even if backend is down
      }
    }

    // Clear cookies
    await clearAuthCookies();
  } catch {
    // TODO: log to observability tool
    return {
      success: false,
      error: "Logout failed",
    };
  }

  // Redirect to home (must be outside try/catch)
  redirect("/");
}
