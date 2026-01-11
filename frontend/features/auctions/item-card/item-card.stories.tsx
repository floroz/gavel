import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ItemCard } from "./item-card";
import type { Item } from "@/shared/api/items";
import { ItemStatus } from "@/proto/bids/v1/bid_service_pb";

const meta = {
  title: "Features/Auctions/ItemCard",
  component: ItemCard,
  parameters: {
    layout: "centered",
  },
  decorators: [
    (Story) => (
      <div className="w-[400px]">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ItemCard>;

export default meta;
type Story = StoryObj<typeof meta>;

const mockActiveItem: Item = {
  id: "item-1",
  title: "Vintage Camera",
  description: "A beautiful vintage camera from the 1960s",
  category: "Electronics",
  startPrice: 5000,
  currentHighestBid: 7500,
  sellerId: "seller-1",
  status: ItemStatus.ACTIVE,
  endAt: new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString(),
  images: ["https://images.unsplash.com/photo-1611532736597-de2d4265fba3?w=500"],
  createdAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
  updatedAt: new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(),
};

export const ActiveAuction: Story = {
  args: {
    item: mockActiveItem,
    currentUserId: "user-1",
  },
};

export const ActiveAuctionWithBids: Story = {
  args: {
    item: {
      ...mockActiveItem,
      currentHighestBid: 8500,
    },
    currentUserId: "user-1",
  },
};

export const ActiveAuctionNoBids: Story = {
  args: {
    item: {
      ...mockActiveItem,
      currentHighestBid: 0,
    },
    currentUserId: "user-1",
  },
};

export const OwnListing: Story = {
  args: {
    item: mockActiveItem,
    currentUserId: "seller-1",
  },
};

export const EndedAuction: Story = {
  args: {
    item: {
      ...mockActiveItem,
      endAt: new Date(Date.now() - 1 * 60 * 60 * 1000).toISOString(),
    },
    currentUserId: "user-1",
  },
};

export const CancelledAuction: Story = {
  args: {
    item: {
      ...mockActiveItem,
      status: ItemStatus.CANCELLED,
    },
    currentUserId: "user-1",
  },
};

export const NoImage: Story = {
  args: {
    item: {
      ...mockActiveItem,
      images: [],
    },
    currentUserId: "user-1",
  },
};

export const LongTitle: Story = {
  args: {
    item: {
      ...mockActiveItem,
      title: "Extremely Rare and Beautiful Vintage Camera from the 1960s with Original Leather Case",
    },
    currentUserId: "user-1",
  },
};

export const EndingSoon: Story = {
  args: {
    item: {
      ...mockActiveItem,
      endAt: new Date(Date.now() + 45 * 60 * 1000).toISOString(), // 45 minutes
    },
    currentUserId: "user-1",
  },
};

export const UnauthenticatedUser: Story = {
  args: {
    item: mockActiveItem,
    currentUserId: undefined,
  },
};
