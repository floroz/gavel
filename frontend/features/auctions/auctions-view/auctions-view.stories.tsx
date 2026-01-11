import type { Meta, StoryObj } from "@storybook/react";
import { AuctionsView } from "./auctions-view";
import type { Item } from "@/shared/api";

const meta = {
  title: "Features/Auctions/AuctionsView",
  component: AuctionsView,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof AuctionsView>;

export default meta;
type Story = StoryObj<typeof meta>;

const mockItems: Item[] = [
  {
    id: "item-1",
    title: "Vintage Camera",
    description: "A beautiful vintage camera from the 1960s",
    category: "Electronics",
    startPrice: 5000,
    currentHighestBid: 7500,
    sellerId: "seller-1",
    status: 0, // ACTIVE
    endAt: new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString(),
    images: ["https://images.unsplash.com/photo-1611532736597-de2d4265fba3?w=500"],
    createdAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: "item-2",
    title: "Classic Leather Watch",
    description: "Swiss-made leather watch",
    category: "Accessories",
    startPrice: 3000,
    currentHighestBid: 4500,
    sellerId: "seller-2",
    status: 0,
    endAt: new Date(Date.now() + 1 * 24 * 60 * 60 * 1000).toISOString(),
    images: ["https://images.unsplash.com/photo-1523170335258-f5ed11844a49?w=500"],
    createdAt: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000).toISOString(),
  },
  {
    id: "item-3",
    title: "Antique Oil Painting",
    description: "19th century landscape painting",
    category: "Art",
    startPrice: 10000,
    currentHighestBid: 0,
    sellerId: "seller-3",
    status: 0,
    endAt: new Date(Date.now() + 5 * 24 * 60 * 60 * 1000).toISOString(),
    images: ["https://images.unsplash.com/photo-1578926314433-a11be7c30c76?w=500"],
    createdAt: new Date().toISOString(),
  },
  {
    id: "item-4",
    title: "Collectible Vinyl Record",
    description: "First edition vinyl album",
    category: "Music",
    startPrice: 2000,
    currentHighestBid: 3200,
    sellerId: "seller-1",
    status: 0,
    endAt: new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString(),
    images: ["https://images.unsplash.com/photo-1470225620780-dba8ba36b745?w=500"],
    createdAt: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
  },
];

export const Default: Story = {
  args: {
    items: mockItems,
    currentUser: {
      userId: "user-1",
      email: "john@example.com",
    },
  },
};

export const WithAuthenticatedUser: Story = {
  args: {
    items: mockItems,
    currentUser: {
      userId: "user-1",
      email: "buyer@example.com",
    },
  },
};

export const WithUnauthenticatedUser: Story = {
  args: {
    items: mockItems,
    currentUser: null,
  },
};

export const EmptyState: Story = {
  args: {
    items: [],
    currentUser: {
      userId: "user-1",
      email: "john@example.com",
    },
  },
};
