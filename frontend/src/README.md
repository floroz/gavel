# BFF Layer Architecture

This directory contains the Backend for Frontend (BFF) implementation for the auction system.

## Directory Structure

```
src/
├── server/               # Server-only code (BFF layer)
│   ├── handlers/        # TanStack Router server function handlers
│   ├── services/        # gRPC client wrappers for backend services
│   ├── middleware/      # Server middleware (context, auth, etc.)
│   └── rpc-internal.ts  # gRPC transport configuration
│
├── shared/              # Shared between browser and server
│   ├── api/            # API contracts (Zod schemas)
│   └── types/          # Non-schema shared types
│
├── components/          # React components (browser)
├── routes/             # TanStack Router routes
├── hooks/              # React hooks (browser)
└── lib/                # Browser utilities

proto/                   # Generated protobuf types (server-only imports)
```

## Layer Boundaries

### 1. Browser Layer (`components/`, `routes/`, `hooks/`, `lib/`)
- **Purpose**: User interface and client-side logic
- **Can import from**: `shared/`
- **Cannot import from**: `server/`, `proto/`
- **Communication**: Calls server functions via TanStack Router

### 2. Shared Layer (`shared/`)
- **Purpose**: Type definitions and schemas shared between browser and BFF
- **Can be imported by**: Browser and server code
- **Contains**: 
  - Zod schemas for API contracts
  - TypeScript types derived from schemas
  - Common utilities that work in both environments

### 3. Server Layer (`server/`)
- **Purpose**: BFF logic, auth mediation, backend communication
- **Can import from**: `shared/`, `proto/`
- **Cannot be imported by**: Browser code
- **Contains**:
  - Server functions (API endpoints)
  - gRPC client wrappers
  - Middleware (auth, context enrichment)

### 4. Protobuf Layer (`proto/`)
- **Purpose**: Generated types for backend service communication
- **Can import from**: Server code only
- **Represents**: Backend-to-backend contracts (not browser contracts)

## Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                        Browser                               │
│  Components → Server Functions (uses shared/ schemas)       │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           │ (HTTP, cookies auto-sent)
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                         BFF (Node.js)                        │
│  Server Functions → Service Wrappers                         │
│    - Validate input (shared/ schemas)                        │
│    - Enrich with server data (IP, User-Agent, tokens)       │
│    - Map to proto types                                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           │ (gRPC with Bearer token)
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   Backend Services                           │
│  auth-service, bid-service, user-stats-service (private)    │
└─────────────────────────────────────────────────────────────┘
```

## Type System

### Browser ↔ BFF: Zod Schemas (`shared/api/`)

```typescript
// shared/api/auth.ts
export const loginInputSchema = z.object({
  email: z.string().email(),
  password: z.string().min(8),
})
export type LoginInput = z.infer<typeof loginInputSchema>
```

**Why**: 
- Runtime validation of untrusted input
- Browser only needs to know about user-providable fields
- Decouples browser from backend implementation

### BFF ↔ Backend: Protobuf Types (`proto/`)

```typescript
// proto/auth/v1/auth_service_pb.ts (generated)
export type LoginRequest = {
  email: string
  password: string
  ipAddress: string  // Server-side enrichment
  userAgent: string  // Server-side enrichment
}
```

**Why**:
- Backend needs additional metadata (IP, User-Agent)
- Type safety for gRPC communication
- Single source of truth for backend contracts

## Example: Login Flow

### 1. Browser Component
```typescript
import { loginInputSchema, type LoginInput } from '@/shared/api/auth'

// Form only collects user-providable fields
const handleLogin = async (data: LoginInput) => {
  await loginFn({ data })
}
```

### 2. Server Function (Handler)
```typescript
// server/handlers/auth.ts
import { loginInputSchema } from '@/shared/api/auth'
import { authService } from '@/server/services/auth-service'

export const loginFn = createServerFn({ method: 'POST' })
  .validator(loginInputSchema)
  .handler(async ({ data, request }) => {
    // Enrich with server-side data
    const response = await authService.login({
      email: data.email,
      password: data.password,
      ipAddress: getClientIP(request),
      userAgent: request.headers.get('user-agent') || '',
    })
    
    // Set cookies (tokens never sent to browser)
    setAuthCookies(response.accessToken, response.refreshToken)
    
    return { userId: response.userId }
  })
```

### 3. Service Wrapper
```typescript
// server/services/auth-service.ts
import { createClient } from '@connectrpc/connect'
import { AuthService } from '@/proto/auth/v1/auth_service_pb'

export const authClient = createClient(AuthService, createAuthTransport())

export const authService = {
  async login(params: {
    email: string
    password: string
    ipAddress: string
    userAgent: string
  }) {
    // Direct mapping to proto types
    return await authClient.login(params)
  }
}
```

## Key Principles

1. **Separation of Concerns**: Each layer has a clear responsibility
2. **Type Safety**: End-to-end type safety with appropriate boundaries
3. **Security**: Tokens never exposed to browser, server-side enrichment
4. **DX**: Clear import rules prevent accidental coupling
5. **Maintainability**: Easy to evolve browser and backend APIs independently

## Import Rules (Enforced by Build Config)

✅ **Allowed**:
- Browser → `shared/`
- Server → `shared/` + `proto/`

❌ **Forbidden**:
- Browser → `server/`
- Browser → `proto/`
- `shared/` → `server/`
- `shared/` → `proto/`

## Next Steps

See [PLAN-AUTH-BFF.md](../../../docs/PLAN-AUTH-BFF.md) for implementation milestones.

Current Status: **Milestone 1 Complete** ✅
- Folder structure created
- Auth API schemas defined
- Service wrappers implemented
- Transport configuration established

