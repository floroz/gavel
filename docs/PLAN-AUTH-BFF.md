# Authentication Architecture: Backend for Frontend (BFF)

## 1. The Challenge

Our application uses a microservices architecture with a dedicated **Auth Service** (Go/gRPC) and a **Frontend** built with TanStack Start (SSR/React).

We face three critical challenges with the traditional "Direct Browser-to-Microservice" authentication pattern:

1.  **Security**: Storing JWTs in `localStorage` exposes users to XSS attacks.
2.  **Server-Side Rendering (SSR)**: The Node.js SSR server cannot access `localStorage`, causing "flashes of unauthenticated content" (rendering as guest first, then hydration logs the user in).
3.  **Token Management**: Implementing silent refresh logic (handling 401s, refreshing tokens, retrying requests) in the browser is complex and error-prone.

## 2. The Solution: Backend for Frontend (BFF)

We will implement the **Backend for Frontend (BFF)** pattern (also known as the "Token Handler" pattern).

In this architecture, the **Frontend Server (Node.js)** acts as a secure proxy between the Browser and the Microservices.

### Key Concepts

*   **HttpOnly Cookies**: The browser never sees the Access/Refresh tokens. It only holds encrypted/signed HttpOnly cookies containing the tokens.
*   **Server Functions**: The browser calls TanStack Start server functions (e.g., `loginFn`, `placeBidFn`) instead of calling Microservices directly.
*   **Token Mediation**: The Frontend Server reads the cookies, attaches the `Authorization: Bearer <token>` header, and forwards the request to the gRPC services.
*   **Server-Side Refresh**: If a microservice returns `401 Unauthorized`, the Frontend Server transparently calls `AuthService.Refresh`, updates the user's cookies, and retries the original request.

## 3. Implementation Strategy

### A. Components

1.  **Frontend Server (BFF)**: TanStack Start server functions handling the proxy logic.
2.  **Internal Services**: `auth-service`, `bid-service`, `user-stats-service` (Not exposed publicly).
3.  **Frontend Ingress**: The ONLY public entry point.

### B. Architecture Impact

*   **Traffic Flow**:
    *   **Public**: Browser → NGINX Ingress → Frontend Node Pod (BFF)
    *   **Private**: Frontend Node Pod → (K8s Service DNS) → Auth/Bid Service
*   **Ingress Changes**:
    *   **KEEP**: Frontend Ingress (TLS, Load Balancing).
    *   **DELETE**: Ingress resources for `auth-service`, `bid-service`, and `user-stats-service`.
*   **Security Improvements**:
    *   **Zero Trust**: Backend services are no longer exposed to the public internet.
    *   **CORS Elimination**: No CORS configuration needed on backends; all browser requests are same-origin to the Frontend.
    *   **Token Safety**: Tokens never leave the server-side environment.
*   **UX**: Perfect SSR support. The server knows the user identity immediately upon request receipt.

## 4. Communication Architecture

### The Problem

Previously, our `rpc.ts` was designed for direct Browser→Backend communication:

```
Browser ─────ConnectRPC────▶ auth-service (public)
Browser ─────ConnectRPC────▶ bid-service (public)
```

With BFF, backends become private. We need two distinct communication layers:

```
Browser ───Server Functions──▶ BFF (public)
BFF ────────ConnectRPC───────▶ Backend Services (private)
```

### A. Browser → BFF: TanStack Server Functions

**Why Server Functions (not ConnectRPC from browser)**:
- Automatic cookie handling (no manual `credentials: 'include'`)
- Type-safe RPC built into TanStack Start
- Same function callable from SSR and client hydration
- No additional HTTP layer to maintain

**Pattern**:

```typescript
// src/server/api/auth.ts
import { createServerFn } from '@tanstack/react-start'
import { authClient } from './rpc-internal'
import { setAuthCookies, getAuthCookies, clearAuthCookies } from './cookies'

export const loginFn = createServerFn({ method: 'POST' })
  .validator((data: { email: string; password: string }) => data)
  .handler(async ({ data }) => {
    const response = await authClient.login({
      email: data.email,
      password: data.password,
      // Extract from request headers
      ipAddress: getClientIP(),
      userAgent: getUserAgent(),
    })
    
    // Set HttpOnly cookies (tokens never returned to browser)
    setAuthCookies(response.accessToken, response.refreshToken)
    
    return { success: true }
  })
```

### B. BFF → Backend Services: ConnectRPC (Internal)

**Location**: `src/server/rpc-internal.ts`

This is server-only code. The ConnectRPC clients are used exclusively by server functions:

```typescript
// src/server/rpc-internal.ts (SERVER-ONLY)
import { createClient } from '@connectrpc/connect'
import { createGrpcTransport } from '@connectrpc/connect-node'
import { AuthService } from '@proto/auth/v1/auth_service_pb'

// Direct K8s service URLs (cluster-internal)
const AUTH_SERVICE_URL = process.env.AUTH_SERVICE_URL!
const BID_SERVICE_URL = process.env.BID_SERVICE_URL!

const authTransport = createGrpcTransport({
  baseUrl: AUTH_SERVICE_URL,
  httpVersion: '2',
})

export const authClient = createClient(AuthService, authTransport)
export const bidClient = createClient(BidService, bidTransport)
```

### C. Request Types

| Request Type | Flow | Token Handling |
|--------------|------|----------------|
| **SSR Data Fetch** | Route loader → Server Function → Backend | Read cookie, attach Bearer header |
| **Client Mutation** | Browser → Server Function → Backend | Cookie sent automatically, server reads it |
| **Client Query** | Browser → Server Function → Backend | Same as mutation |

### D. Migration from Direct RPC

**Before (Direct Browser → Backend)**:
```typescript
// src/lib/rpc.ts - Browser calls backend directly
const authTransport = createConnectTransport({
  baseUrl: getServiceUrl('auth'), // Was public ingress URL
})
export const authClient = createClient(AuthService, authTransport)

// Component usage
await authClient.register({ email, password, ... })
```

**After (Browser → BFF → Backend)**:
```typescript
// src/server/api/auth.ts - Server function wraps backend call
export const registerFn = createServerFn({ method: 'POST' })
  .validator(registerSchema)
  .handler(async ({ data }) => {
    const response = await authClient.register(data)
    setAuthCookies(response.accessToken, response.refreshToken)
    return { userId: response.userId }
  })

// Component usage
await registerFn({ data: { email, password, ... } })
```

## 5. Auth Flows

### Registration

1. User submits registration form in browser
2. Browser calls `registerFn` server function
3. BFF calls `auth-service.Register` via ConnectRPC
4. BFF calls `auth-service.Login` (or Register returns tokens directly)
5. BFF sets HttpOnly cookies with tokens
6. BFF returns success response (no tokens in body)
7. Browser redirects to authenticated page

### Login

1. User submits login form in browser
2. Browser calls `loginFn` server function
3. BFF calls `auth-service.Login` via ConnectRPC
4. BFF sets HttpOnly cookies with tokens
5. BFF returns success response (no tokens in body)
6. Browser redirects to authenticated page

### Navigate to Protected Route

1. User navigates to a protected route
2. Browser sends request with cookies
3. BFF middleware reads `access_token` cookie
4. If access token is expired but refresh token is valid:
   - BFF calls `auth-service.Refresh`
   - BFF updates cookies with new tokens
5. BFF injects user claims into route context
6. Route loader fetches data using server functions (with token attached)
7. BFF returns SSR-rendered protected page

### Logout

1. User clicks logout
2. Browser calls `logoutFn` server function
3. BFF calls `auth-service.Logout` to revoke refresh token
4. BFF clears all auth cookies
5. Browser redirects to public page

## 6. Decisions & Validation

We have validated the BFF pattern against performance and architectural concerns:

1.  **BFF as Bottleneck**:
    *   **Concern**: Does proxying all traffic through Node.js create a bottleneck?
    *   **Decision**: Accepted. Node.js is efficient at I/O-bound tasks (proxying). If load becomes an issue, we will horizontally scale the Frontend/BFF pods.

2.  **Event Loop Blocking**:
    *   **Concern**: Will combining SSR (CPU-bound) and Proxying (I/O-bound) block the loop?
    *   **Decision**: Accepted trade-off. Proxying yields the event loop. If SSR becomes too heavy, we can split the architecture into "Rendering Pods" and "Gateway/BFF Pods" in the future.

3.  **Role of NGINX**:
    *   **Concern**: Do we still need NGINX if the BFF handles auth?
    *   **Decision**: Yes, but only for the Frontend.
    *   **Change**: We will remove Ingresses for all backend microservices. They will become private, accessible only via the BFF within the cluster. This significantly reduces the attack surface and simplifies network configuration.

4.  **Why Not ConnectRPC from Browser?**:
    *   **Concern**: We already have ConnectRPC clients. Why add server functions?
    *   **Decision**: Server functions provide automatic cookie handling, type safety, and work seamlessly in both SSR and client contexts. ConnectRPC is still used, but only for BFF→Backend communication where it excels (gRPC, streaming, etc.).

## 7. Operational Details

### Cookies

| Cookie | Attributes | TTL | Purpose |
|--------|------------|-----|---------|
| `__Host-access_token` | HttpOnly, Secure, SameSite=Strict, Path=/ | ~15 min | Short-lived auth token |
| `__Host-refresh_token` | HttpOnly, Secure, SameSite=Strict, Path=/ | 7 days | Token rotation (sliding window) |

**Cookie Security Notes**:
- `__Host-` prefix enforces: Secure flag, no Domain attribute, Path must be `/`
- `SameSite=Strict` provides CSRF protection for mutations
- With `SameSite=Strict`, explicit CSRF tokens are not required for POST requests
- Refresh token uses sliding window: each refresh extends the 7-day window
- Reuse detection: if a refresh token is used after rotation, revoke all user tokens

### Auth Request Handling

**Browser Transport**:
- Server functions handle cookie transmission automatically
- No `credentials: 'include'` needed—cookies are read server-side

**Server/SSR Transport**:
- Read `access_token` from cookie
- Attach `Authorization: Bearer <access_token>` to upstream service calls
- If 401 received:
  1. Read `refresh_token` from cookie
  2. Call `auth-service.Refresh`
  3. Update cookies with new tokens
  4. Retry original request once
  5. If still 401, clear cookies and return unauthorized

### SSR Guard/Middleware

The `requireAuth` middleware:
1. Reads cookies from incoming request
2. Validates access token expiry (without calling backend)
3. If expired, attempts refresh using refresh token
4. Injects user claims into route context
5. Updates response cookies if tokens were refreshed
6. Redirects to login if authentication fails

### Network Shape

```
┌─────────────────────────────────────────────────────────────────┐
│                         INTERNET                                 │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    NGINX Ingress                                 │
│                    (TLS Termination)                             │
│                    frontend.example.com                          │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                 KUBERNETES CLUSTER                               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │              Frontend Pod (BFF)                            │  │
│  │              - TanStack Start SSR                          │  │
│  │              - Server Functions                            │  │
│  │              - Cookie Management                           │  │
│  └───────────────────────┬───────────────────────────────────┘  │
│                          │ (K8s Service DNS)                     │
│         ┌────────────────┼────────────────┐                      │
│         ▼                ▼                ▼                      │
│  ┌────────────┐   ┌────────────┐   ┌────────────┐               │
│  │auth-service│   │bid-service │   │user-stats  │               │
│  │  (private) │   │  (private) │   │  (private) │               │
│  └────────────┘   └────────────┘   └────────────┘               │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 8. JWT Validation & Key Management

### Backend JWT Validation

Each backend service needs to validate JWTs sent by the BFF:

1. **Shared Public Key**: Auth-service signs JWTs with a private key. Other services validate with the corresponding public key.
2. **Key Distribution**: Public key is distributed via:
   - Kubernetes Secret mounted to all service pods, OR
   - JWKS endpoint exposed by auth-service (cluster-internal)

### Validation Flow

```
BFF ──Bearer Token──▶ bid-service
                           │
                           ▼
                    Validate JWT signature (public key)
                    Check expiry (exp claim)
                    Extract user claims (sub, email, permissions)
                           │
                           ▼
                    Process request with user context
```

### Key Rotation

- Generate new key pair
- Auth-service starts signing with new key
- Both old and new public keys are valid during transition
- After token TTL passes, remove old public key

## 9. Session Invalidation Strategy

### Scenarios

| Event | Action |
|-------|--------|
| **User logs out** | Revoke refresh token, clear cookies |
| **Password change** | Revoke ALL user's refresh tokens |
| **Admin revokes user** | Revoke ALL tokens + add user to short-lived deny list |
| **Security breach** | Rotate signing keys (invalidates all access tokens) |

### Implementation

**Refresh Token Revocation** (already implemented):
- Refresh tokens stored in DB with `revoked` flag
- On logout/password change: `UPDATE refresh_tokens SET revoked = true WHERE user_id = ?`

**Immediate Access Token Invalidation** (for admin revocation):
- Option A: Short access token TTL (15 min) means natural expiry handles most cases
- Option B: Redis deny list for user IDs (checked on each request, TTL = access token lifetime)

## 10. References

*   **Auth0**: [The Backend for Frontend Pattern](https://auth0.com/blog/backend-for-frontend-pattern-with-auth0-and-dotnet/)
*   **OWASP**: [Local Storage Security](https://cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html#local-storage)
*   **Curity**: [The Token Handler Pattern](https://curity.io/resources/learn/token-handler-pattern/)
*   **TanStack Start**: [Server Functions](https://tanstack.com/start/latest/docs/framework/react/server-functions)
