/**
 * Next.js Proxy (Lightweight Auth Check)
 *
 * This proxy performs a FAST check before pages load:
 * - If no auth cookies exist → redirect to /login immediately
 * - If cookies exist → let the request through (RSC validates tokens)
 *
 * We intentionally keep this lightweight:
 * - No token validation (avoid network calls in proxy)
 * - No token refresh (handled by getSession in RSCs)
 * - Just a quick cookie presence check
 */

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PROTECTED_ROUTES = ["/dashboard", "/auctions"];
const AUTH_ROUTES = ["/login", "/register"];

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Check if this is a protected route
  const isProtectedRoute = PROTECTED_ROUTES.some((route) =>
    pathname.startsWith(route),
  );

  // Check if this is an auth route (login/register)
  const isAuthRoute = AUTH_ROUTES.some((route) => pathname.startsWith(route));

  const accessToken = request.cookies.get("access_token")?.value;
  const refreshToken = request.cookies.get("refresh_token")?.value;
  const hasAnyToken = accessToken || refreshToken;

  // Protected route without tokens → redirect to login
  if (isProtectedRoute && !hasAnyToken) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  // Auth route with valid session → redirect to dashboard
  if (isAuthRoute && hasAnyToken) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  // Allow request to proceed
  return NextResponse.next();
}

export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization)
     * - favicon.ico, sitemap.xml, robots.txt
     * - public files (images, etc.)
     */
    "/((?!_next/static|_next/image|favicon.ico|sitemap.xml|robots.txt|.*\\.png$|.*\\.jpg$|.*\\.svg$).*)",
  ],
};
