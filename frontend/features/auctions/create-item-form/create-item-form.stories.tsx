import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { CreateItemForm } from "./create-item-form";
import type { CreateItemInput, Item } from "@/shared/api/items";
import type { ActionResult } from "@/shared/types";

const meta = {
  title: "Features/Auctions/CreateItemForm",
  component: CreateItemForm,
  parameters: {
    layout: "fullscreen",
    nextjs: {
      appDirectory: true,
    },
  },
  args: {
    createItemAction: async (input: CreateItemInput): Promise<ActionResult<Item>> => {
      console.log("Create item action called with:", input);
      // Simulate successful creation
      return {
        success: true,
        data: {
          id: "mock-item-id",
          title: input.title,
          description: input.description,
          category: input.category,
          startPrice: input.startPrice,
          currentHighestBid: 0,
          sellerId: "mock-seller-id",
          status: 0, // ACTIVE
          endAt: input.endAt,
          images: input.images,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      };
    },
  },
} satisfies Meta<typeof CreateItemForm>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithFailure: Story = {
  args: {
    createItemAction: async (input: CreateItemInput): Promise<ActionResult<Item>> => {
      console.log("Create item action called with:", input);
      // Simulate failure
      return {
        success: false,
        error: "Failed to create item. Please try again.",
      };
    },
  },
};

export const WithMockRouter: Story = {
  parameters: {
    nextjs: {
      navigation: {
        pathname: "/auctions/new",
      },
    },
  },
};
