import Link from "next/link";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import type { Item } from "@/shared/api/items";
import { ItemStatus } from "@/proto/bids/v1/bid_service_pb";
import { formatTimeRemainingCompact } from "@/lib/date";

type ItemCardProps = {
  item: Item;
  currentUserId?: string;
};

export function ItemCard({ item, currentUserId }: ItemCardProps) {
  const isActive = item.status === ItemStatus.ACTIVE;
  const endDate = new Date(item.endAt);
  const now = new Date();
  const isEnded = endDate < now;
  const isOwnItem = currentUserId === item.sellerId;

  // Format price for display (convert from cents to dollars)
  const formatPrice = (cents: number) => {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(cents / 100);
  };

  const statusBadge = () => {
    if (item.status === ItemStatus.CANCELLED) {
      return <span className="text-xs text-destructive font-medium">Cancelled</span>;
    }
    if (!isActive || isEnded) {
      return <span className="text-xs text-muted-foreground font-medium">Ended</span>;
    }
    return <span className="text-xs text-green-600 font-medium">Active</span>;
  };

  return (
    <Card className="overflow-hidden hover:shadow-md transition-shadow">
      <Link href={`/auctions/${item.id}`}>
        {item.images.length > 0 ? (
          <div className="aspect-video bg-muted relative overflow-hidden">
            <img
              src={item.images[0]}
              alt={item.title}
              className="w-full h-full object-cover"
            />
          </div>
        ) : (
          <div className="aspect-video bg-muted flex items-center justify-center">
            <span className="text-muted-foreground text-sm">No image</span>
          </div>
        )}
      </Link>

      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <Link href={`/auctions/${item.id}`} className="flex-1 min-w-0">
            <CardTitle className="line-clamp-1 hover:underline">
              {item.title}
            </CardTitle>
          </Link>
          {statusBadge()}
        </div>
        {item.category && (
          <p className="text-xs text-muted-foreground">{item.category}</p>
        )}
      </CardHeader>

      <CardContent className="space-y-2">
        <div className="flex justify-between items-center">
          <span className="text-sm text-muted-foreground">Current bid</span>
          <span className="font-semibold">
            {item.currentHighestBid > 0
              ? formatPrice(item.currentHighestBid)
              : formatPrice(item.startPrice)}
          </span>
        </div>
        {isActive && !isEnded && (
          <div className="flex justify-between items-center">
            <span className="text-sm text-muted-foreground">Time left</span>
            <span className="text-sm font-medium">{formatTimeRemainingCompact(item.endAt, isActive)}</span>
          </div>
        )}
      </CardContent>

      <CardFooter>
        <Button asChild className="w-full" variant={isOwnItem && isActive && !isEnded ? "outline" : "default"}>
          <Link href={isOwnItem ? `/dashboard/listings` : `/auctions/${item.id}`}>
            {isOwnItem && isActive && !isEnded
              ? "Go to your listing"
              : isActive && !isEnded
                ? "Place Bid"
                : "View Details"}
          </Link>
        </Button>
      </CardFooter>
    </Card>
  );
}
