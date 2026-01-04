/**
 * Bid Server Actions
 *
 * These Server Actions handle bid-related operations:
 * - Place Bid
 *
 * All actions require authentication and attach the Bearer token to backend requests.
 * The backend extracts user ID from the JWT token - we don't pass it in the request.
 */

"use server";

import { getSession } from "@/lib/auth";
import { bidClient } from "@/lib/rpc";
import { PlaceBidInput, placeBidInputSchema } from "@/shared/api";
import { type ActionResult } from "@/shared/types";

/**
 * Place Bid Action
 * Requires authentication
 */
export async function placeBidAction(input: PlaceBidInput): Promise<
  ActionResult<{
    bidId: string;
    amount: bigint;
    createdAt: string;
  }>
> {
  // Validate input
  const parsed = placeBidInputSchema.safeParse(input);

  if (!parsed.success) {
    return {
      success: false,
      error: "Invalid bid data",
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
    // Backend extracts userId from JWT token
    const response = await bidClient.placeBid(
      {
        itemId: parsed.data.itemId,
        amount: BigInt(parsed.data.amount),
      },
      {
        headers: {
          Authorization: `Bearer ${session.accessToken}`,
        },
      },
    );

    if (!response.bid) {
      return {
        success: false,
        error: "Failed to place bid. Please try again.",
      };
    }

    return {
      success: true,
      data: {
        bidId: response.bid.id,
        amount: response.bid.amount,
        createdAt: response.bid.createdAt,
      },
    };
  } catch {
    return {
      success: false,
      error: "Failed to place bid. Please try again.",
    };
  }
}
