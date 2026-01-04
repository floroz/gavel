import { getSession } from "@/lib/auth";
import { statsClient } from "@/lib/rpc";

export interface UserStats {
  userId: string;
  totalBids: bigint;
  totalAmount: bigint;
  lastUpdatedAt: string;
}

/**
 * Get User Stats
 * This is a helper function for Server Components
 * Use this in RSCs to fetch user statistics
 */
export async function getUserStats(): Promise<UserStats | null> {
  // Get authenticated session
  const session = await getSession();

  if (!session) {
    return null;
  }

  try {
    const response = await statsClient.getUserStats(
      {},
      {
        headers: {
          Authorization: `Bearer ${session.accessToken}`,
        },
      },
    );

    if (!response.stats) {
      return null;
    }

    return {
      userId: response.stats.userId,
      totalBids: response.stats.totalBids,
      totalAmount: response.stats.totalAmount,
      lastUpdatedAt: response.stats.lastUpdatedAt,
    };
  } catch (error) {
    console.error("Failed to fetch user stats:", error);
    return null;
  }
}
