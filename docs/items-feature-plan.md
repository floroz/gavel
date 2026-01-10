# Items Feature Implementation Plan

## Summary
Add full items management to the auction application, extending the existing bid-service with CRUD operations for auction items, plus a complete frontend UI.

## Decisions Made
| Decision | Choice |
|----------|--------|
| Service architecture | Extend existing bid-service |
| Additional fields | Images (URL array) + Category |
| Permissions | Any authenticated user can create |
| Lifecycle states | active, ended, cancelled |
| UI views | Full set (list, detail, create, my listings) |
| Images | URL references only (no file upload) |

---

## Phase 1: Database Schema Update

**Modify:** `services/bid-service/migrations/00001_initial_schema.sql`

Update the existing items table definition to include new fields:

```sql
CREATE TYPE item_status AS ENUM ('active', 'ended', 'cancelled');

CREATE TABLE items (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    start_price BIGINT NOT NULL CHECK (start_price >= 0),
    current_highest_bid BIGINT DEFAULT 0 CHECK (current_highest_bid >= 0),
    end_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    -- New fields
    images TEXT[] DEFAULT '{}',
    category TEXT,
    seller_id UUID NOT NULL,
    status item_status NOT NULL DEFAULT 'active'
);

CREATE INDEX idx_items_seller_id ON items(seller_id);
CREATE INDEX idx_items_status ON items(status);
CREATE INDEX idx_items_status_end_at ON items(status, end_at) WHERE status = 'active';
```

*Note: User will reset the database after schema changes.*

### Phase 1 Validation
- [ ] Migration runs successfully
- [ ] All new columns exist with correct types
- [ ] Indexes created

---

## Phase 2: Proto Updates

**Modify:** `api/proto/bids/v1/bid_service.proto`

Add new RPC methods:
- `CreateItem` - Create auction item (auth required)
- `GetItem` - Get item by ID (public)
- `ListItems` - List active items with pagination (public)
- `ListSellerItems` - List seller's items (auth required)
- `UpdateItem` - Update item fields (owner only)
- `CancelItem` - Cancel auction (owner only, no bids)
- `GetItemBids` - Get bids for an item (public)

### Phase 2 Validation
- [ ] Proto compiles without errors
- [ ] Go code generated successfully (`buf generate`)
- [ ] TypeScript types generated for frontend

---

## Phase 3: Backend Implementation

### Domain Layer
**Create:** `services/bid-service/internal/domain/items/`
- `models.go` - Item struct with new fields, ItemStatus enum, helper methods
- `ports.go` - ItemRepository interface
- `service.go` - ItemService with business rules:
  - Validate start price > 0
  - Validate end time in future
  - Only owner can update/cancel
  - Can only cancel if no bids
  - Seller cannot bid on own item

### Repository Layer
**Modify:** `services/bid-service/internal/adapters/database/item_repository.go`
- Add CreateItem, UpdateItem, UpdateStatus, ListActiveItems, ListItemsBySellerID

### API Layer
**Modify:** `services/bid-service/internal/adapters/api/handler.go`
- Add handlers for all new RPC methods
- Wire up ItemService in main.go

### Phase 3 Validation
- [ ] Unit tests for Item domain methods (IsActive, CanBeCancelled, IsOwnedBy)
- [ ] Integration tests for all API endpoints (CreateItem, GetItem, ListItems, etc.)
- [ ] Test: seller cannot bid on own item
- [ ] Test: cannot cancel item with bids
- [ ] Test: item status transitions work correctly

---

## Phase 4: Frontend Implementation

### Data Fetching Pattern
- **Mutations (Server Actions):** createItem, updateItem, cancelItem
- **Reads in RSC (preferred):** getItem, listItems, listSellerItems, getItemBids
- **Reads for client components:** Next.js API routes if needed (e.g., for polling/refetching)

### Server Actions (mutations only)
**Create:** `frontend/actions/items.ts`
- createItemAction, updateItemAction, cancelItemAction

### Data Fetching (RSC)
**Create:** `frontend/lib/items.ts`
- getItem(id) - fetch single item
- listItems(opts) - fetch active items with pagination
- listSellerItems(opts) - fetch seller's items
- getItemBids(itemId) - fetch bids for an item

### API Schemas
**Create:** `frontend/shared/api/items.ts`
- Item type, CreateItemInput, UpdateItemInput with Zod validation

### Pages
| Route | Component | Description |
|-------|-----------|-------------|
| `/auctions` | AuctionsView | Grid of active auction items |
| `/auctions/[id]` | ItemDetailView | Item details + bid form + bid history |
| `/auctions/new` | CreateItemForm | Form to create new auction |
| `/dashboard/listings` | SellerListingsView | Seller's items with cancel option |

### Components
- `ItemCard` - Card component for listings
- `ItemDetailView` - Full item view with bidding
- `CreateItemForm` - React Hook Form for item creation
- `SellerListingsView` - Dashboard for seller's items

### Navigation Updates
**Modify:** `frontend/components/navbar.tsx`
- Add "Sell" link → `/auctions/new`
- Add "My Listings" link → `/dashboard/listings`

### Phase 4 Validation
- [ ] Manual testing: browse auctions page loads items
- [ ] Manual testing: item detail page shows correct data + bid form
- [ ] Manual testing: create item form submits successfully
- [ ] Manual testing: my listings page shows user's items
- [ ] Manual testing: cancel item works (when no bids)
- [ ] Verify: unauthenticated users redirected from protected pages
- [ ] Verify: placing bid refreshes item detail correctly

---

## Key Files to Modify/Create

### Backend (Go)
1. `services/bid-service/migrations/00001_initial_schema.sql` - MODIFY
2. `api/proto/bids/v1/bid_service.proto` - MODIFY
3. `services/bid-service/internal/domain/items/models.go` - MODIFY (extend Item)
4. `services/bid-service/internal/domain/items/ports.go` - NEW
5. `services/bid-service/internal/domain/items/service.go` - NEW
6. `services/bid-service/internal/adapters/database/item_repository.go` - MODIFY
7. `services/bid-service/internal/adapters/api/handler.go` - MODIFY
8. `services/bid-service/cmd/api/main.go` - MODIFY (wire ItemService)
9. `services/bid-service/internal/domain/bids/service.go` - MODIFY (add seller check)

### Frontend (TypeScript)
1. `frontend/shared/api/items.ts` - NEW (types & schemas)
2. `frontend/lib/items.ts` - NEW (RSC data fetching)
3. `frontend/actions/items.ts` - NEW (mutations only)
4. `frontend/features/auctions/item-card.tsx` - NEW
5. `frontend/features/auctions/auctions-view.tsx` - MODIFY
6. `frontend/features/auctions/item-detail-view.tsx` - NEW
7. `frontend/features/auctions/create-item-form.tsx` - NEW
8. `frontend/features/dashboard/seller-listings-view.tsx` - NEW
9. `frontend/app/auctions/[id]/page.tsx` - NEW
10. `frontend/app/auctions/new/page.tsx` - NEW
11. `frontend/app/dashboard/listings/page.tsx` - NEW
12. `frontend/components/navbar.tsx` - MODIFY

---

## Business Rules Summary

1. **Item Creation**: Any authenticated user, must have title, start_price > 0, end_at in future
2. **Item Status**: active (default) → ended (time expires) or cancelled (by seller)
3. **Cancellation**: Only if status=active AND no bids placed
4. **Editing**: Owner only, limited to title/description/images/category
5. **Bidding**: Must be authenticated, cannot bid on own item, item must be active
