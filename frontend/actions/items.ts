/**
 * Item Server Actions
 *
 * These Server Actions handle item-related mutations:
 * - Create Item
 * - Update Item
 * - Cancel Item
 *
 * All actions require authentication and attach the Bearer token to backend requests.
 * The backend extracts user ID (seller ID) from the JWT token.
 */

"use server";

import { getSession } from "@/lib/auth";
import { bidClient } from "@/lib/rpc";
import {
  type CreateItemInput,
  createItemInputSchema,
  type UpdateItemInput,
  updateItemInputSchema,
  type CancelItemInput,
  cancelItemInputSchema,
  protoItemToItem,
  type Item,
} from "@/shared/api";
import { type ActionResult } from "@/shared/types";
import { revalidatePath } from "next/cache";

/**
 * Create Item Action
 * Requires authentication
 */
export async function createItemAction(
  input: CreateItemInput,
): Promise<ActionResult<Item>> {
  // Validate input
  const parsed = createItemInputSchema.safeParse(input);

  if (!parsed.success) {
    return {
      success: false,
      error: "Invalid item data",
    };
  }

  // Get authenticated session
  const session = await getSession();

  if (!session) {
    return {
      success: false,
      error: "Unauthorized",
    };
  }

  try {
    // Backend extracts sellerId from JWT token
    const response = await bidClient.createItem(
      {
        title: parsed.data.title,
        description: parsed.data.description,
        startPrice: BigInt(parsed.data.startPrice),
        endAt: parsed.data.endAt,
        images: parsed.data.images,
        category: parsed.data.category,
      },
      {
        headers: {
          Authorization: `Bearer ${session.accessToken}`,
        },
      },
    );

    if (!response.item) {
      return {
        success: false,
        error: "Failed to create item. Please try again.",
      };
    }

    // Revalidate relevant pages
    revalidatePath("/auctions");
    revalidatePath("/dashboard/listings");

    return {
      success: true,
      data: protoItemToItem(response.item),
    };
  } catch (error) {
    console.error("Create item error:", error);
    return {
      success: false,
      error: "Failed to create item. Please try again.",
    };
  }
}

/**
 * Update Item Action
 * Requires authentication and ownership
 */
export async function updateItemAction(
  input: UpdateItemInput,
): Promise<ActionResult<Item>> {
  // Validate input
  const parsed = updateItemInputSchema.safeParse(input);

  if (!parsed.success) {
    return {
      success: false,
      error: "Invalid item data",
    };
  }

  // Get authenticated session
  const session = await getSession();

  if (!session) {
    return {
      success: false,
      error: "Unauthorized",
    };
  }

  try {
    const response = await bidClient.updateItem(
      {
        id: parsed.data.id,
        title: parsed.data.title,
        description: parsed.data.description,
        images: parsed.data.images || [],
        category: parsed.data.category,
      },
      {
        headers: {
          Authorization: `Bearer ${session.accessToken}`,
        },
      },
    );

    if (!response.item) {
      return {
        success: false,
        error: "Failed to update item. Please try again.",
      };
    }

    // Revalidate relevant pages
    revalidatePath("/auctions");
    revalidatePath(`/auctions/${parsed.data.id}`);
    revalidatePath("/dashboard/listings");

    return {
      success: true,
      data: protoItemToItem(response.item),
    };
  } catch (error) {
    console.error("Update item error:", error);
    return {
      success: false,
      error: "Failed to update item. You may not have permission to edit this item.",
    };
  }
}

/**
 * Cancel Item Action
 * Requires authentication and ownership
 * Can only cancel if no bids have been placed
 */
export async function cancelItemAction(
  input: CancelItemInput,
): Promise<ActionResult<Item>> {
  // Validate input
  const parsed = cancelItemInputSchema.safeParse(input);

  if (!parsed.success) {
    return {
      success: false,
      error: "Invalid item ID",
    };
  }

  // Get authenticated session
  const session = await getSession();

  if (!session) {
    return {
      success: false,
      error: "Unauthorized",
    };
  }

  try {
    const response = await bidClient.cancelItem(
      {
        id: parsed.data.id,
      },
      {
        headers: {
          Authorization: `Bearer ${session.accessToken}`,
        },
      },
    );

    if (!response.item) {
      return {
        success: false,
        error: "Failed to cancel item. Please try again.",
      };
    }

    // Revalidate relevant pages
    revalidatePath("/auctions");
    revalidatePath(`/auctions/${parsed.data.id}`);
    revalidatePath("/dashboard/listings");

    return {
      success: true,
      data: protoItemToItem(response.item),
    };
  } catch (error) {
    console.error("Cancel item error:", error);
    return {
      success: false,
      error:
        "Failed to cancel item. You may not have permission or bids may have already been placed.",
    };
  }
}
