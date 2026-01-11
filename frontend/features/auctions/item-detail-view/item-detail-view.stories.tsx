import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ItemDetailView } from "./item-detail-view";
import { ItemStatus } from "@/proto/bids/v1/bid_service_pb";
import type { Item } from "@/shared/api/items";
import type { Bid } from "@/lib/items";
import { PlaceBidInput } from "@/shared/api";
import { type ActionResult } from "@/shared/types";

const meta = {
  title: "Features/Auctions/ItemDetailView",
  component: ItemDetailView,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    placeBidAction: async (item: PlaceBidInput) => ({
        success: true,
        data: {
            bidId: "bid-123",
            amount: BigInt(item.amount),
            createdAt: new Date().toISOString(),
        }
    } as ActionResult<{
        bidId: string;
        amount: bigint;
        createdAt: string;
      }>)
  }
} satisfies Meta<typeof ItemDetailView>;

export default meta;
type Story = StoryObj<typeof meta>;

const mockItem: Item = {
  id: "item-1",
  title: "Vintage Camera",
  description: "A beautiful vintage camera from the 1960s in excellent condition.",
  category: "Electronics",
  startPrice: 5000, // $50.00
  currentHighestBid: 7500, // $75.00
  sellerId: "seller-1",
  status: ItemStatus.ACTIVE,
  endAt: new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString(), // 2 days from now
  images: [
    "https://images.unsplash.com/photo-1611532736597-de2d4265fba3?w=500",
    "https://images.unsplash.com/photo-1526170375885-4d8ecf77b99f?w=500",
  ],
  createdAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
  updatedAt: new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(),
};

const mockBids: Bid[] = [
  {
    id: "bid-3",
    itemId: "item-1",
    userId: "user-3",
    amount: 7500,
    createdAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: "bid-2",
    itemId: "item-1",
    userId: "user-2",
    amount: 6500,
    createdAt: new Date(Date.now() - 4 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: "bid-1",
    itemId: "item-1",
    userId: "user-1",
    amount: 5500,
    createdAt: new Date(Date.now() - 6 * 60 * 60 * 1000).toISOString(),
  },
];

export const ActiveAuction: Story = {
  args: {
    item: mockItem,
    bids: mockBids,
    isAuthenticated: true,
    currentUserId: "user-4",
  },
};

export const ActiveAuctionUnauthenticated: Story = {
  args: {
    item: mockItem,
    bids: mockBids,
    isAuthenticated: false,
    currentUserId: undefined,
  },
};

export const OwnListing: Story = {
  args: {
    item: mockItem,
    bids: mockBids,
    isAuthenticated: true,
    currentUserId: "seller-1",
  },
};

export const EndedAuction: Story = {
  args: {
    item: {
      ...mockItem,
      status: ItemStatus.ACTIVE,
      endAt: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(), // 1 hour ago
    },
    bids: mockBids,
    isAuthenticated: true,
    currentUserId: "user-4",
  },
};

export const CancelledAuction: Story = {
  args: {
    item: {
      ...mockItem,
      status: ItemStatus.CANCELLED,
    },
    bids: mockBids,
    isAuthenticated: true,
    currentUserId: "user-4",
  },
};

export const NoBids: Story = {
  args: {
    item: {
      ...mockItem,
      currentHighestBid: 0,
    },
    bids: [],
    isAuthenticated: true,
    currentUserId: "user-4",
  },
};

export const NoImages: Story = {
  args: {
    item: {
      ...mockItem,
      images: [],
    },
    bids: mockBids,
    isAuthenticated: true,
    currentUserId: "user-4",
  },
};
