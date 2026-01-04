import { z } from "zod";

/**
 * Place Bid Input Schema
 */
export const placeBidInputSchema = z.object({
  itemId: z.string().min(1, "Item ID is required"),
  amount: z.number().min(1, "Bid amount must be positive"),
});

export type PlaceBidInput = z.infer<typeof placeBidInputSchema>;
