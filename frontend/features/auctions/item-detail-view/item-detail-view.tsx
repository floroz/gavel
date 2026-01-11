"use client";

import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { placeBidAction } from "@/actions/bids";
import { placeBidInputSchema, type PlaceBidInput } from "@/shared/api/bids";
import type { Item } from "@/shared/api/items";
import type { Bid } from "@/lib/items";
import { ItemStatus } from "@/proto/bids/v1/bid_service_pb";
import { formatTimeRemaining } from "@/lib/date";


type ItemDetailViewProps = {
  item: Item;
  bids: Bid[];
  isAuthenticated: boolean;
  currentUserId?: string;
  placeBidAction: typeof placeBidAction;
};

export function ItemDetailView({
  item,
  bids,
  isAuthenticated,
  currentUserId,
  placeBidAction,
}: ItemDetailViewProps) {
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  const isActive = item.status === ItemStatus.ACTIVE;
  const endDate = new Date(item.endAt);
  const now = new Date();
  const isEnded = endDate < now;
  const canBid = isActive && !isEnded && isAuthenticated;
  const isOwnItem = currentUserId === item.sellerId;

  const form = useForm<PlaceBidInput>({
    resolver: zodResolver(placeBidInputSchema),
    defaultValues: {
      itemId: item.id,
      amount: item.currentHighestBid > 0 ? item.currentHighestBid / 100 + 1 : item.startPrice / 100,
    },
  });

  const formatPrice = (cents: number) => {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(cents / 100);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  async function onSubmit(data: PlaceBidInput) {
    if (isOwnItem) {
      toast.error("You cannot bid on your own item");
      return;
    }

    startTransition(async () => {
      const result = await placeBidAction({
        itemId: data.itemId,
        amount: data.amount * 100, // Convert dollars to cents
      });

      if (result.success) {
        toast.success("Bid placed successfully!");
        form.reset({
          itemId: item.id,
          amount: (result.data?.amount ? Number(result.data.amount) : 0) / 100 + 1,
        });
        router.refresh();
      } else {
        toast.error(result.error);
      }
    });
  }

  const statusBadge = () => {
    if (item.status === ItemStatus.CANCELLED) {
      return <span className="text-sm text-destructive font-medium">Cancelled</span>;
    }
    if (!isActive || isEnded) {
      return <span className="text-sm text-muted-foreground font-medium">Ended</span>;
    }
    return <span className="text-sm text-green-600 font-medium">Active</span>;
  };

  return (
    <div className="container py-8">
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Left Column - Images and Details */}
        <div className="space-y-6">
          {item.images.length > 0 ? (
            <div className="aspect-square bg-muted rounded-lg overflow-hidden">
              <img
                src={item.images[0]}
                alt={item.title}
                className="w-full h-full object-cover"
              />
            </div>
          ) : (
            <div className="aspect-square bg-muted rounded-lg flex items-center justify-center">
              <span className="text-muted-foreground">No image available</span>
            </div>
          )}

          {item.images.length > 1 && (
            <div className="grid grid-cols-4 gap-4">
              {item.images.slice(1, 5).map((image, idx) => (
                <div key={idx} className="aspect-square bg-muted rounded-lg overflow-hidden">
                  <img
                    src={image}
                    alt={`${item.title} ${idx + 2}`}
                    className="w-full h-full object-cover"
                  />
                </div>
              ))}
            </div>
          )}

          <Card>
            <CardHeader>
              <CardTitle>Item Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <h3 className="font-semibold">Description</h3>
                <p className="text-muted-foreground whitespace-pre-wrap">
                  {item.description || "No description provided"}
                </p>
              </div>
              {item.category && (
                <div className="space-y-2">
                  <h3 className="font-semibold">Category</h3>
                  <p className="text-muted-foreground">{item.category}</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Right Column - Bidding */}
        <div className="space-y-6">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <h1 className="text-3xl font-bold">{item.title}</h1>
              {statusBadge()}
            </div>
            <p className="text-muted-foreground">{formatTimeRemaining(item.endAt, isActive)}</p>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Current Price</CardTitle>
              <CardDescription>
                {item.currentHighestBid > 0 ? "Highest bid" : "Starting price"}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-4xl font-bold">
                {formatPrice(item.currentHighestBid > 0 ? item.currentHighestBid : item.startPrice)}
              </p>
            </CardContent>
          </Card>

          {canBid && !isOwnItem && (
            <Card>
              <CardHeader>
                <CardTitle>Place a Bid</CardTitle>
                <CardDescription>
                  Enter your bid amount (minimum: {formatPrice((item.currentHighestBid > 0 ? item.currentHighestBid : item.startPrice) + 100)})
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Form {...form}>
                  <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                    <FormField
                      control={form.control}
                      name="amount"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Bid Amount ($)</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              step="0.01"
                              min={((item.currentHighestBid > 0 ? item.currentHighestBid : item.startPrice) + 100) / 100}
                              placeholder="Enter bid amount"
                              {...field}
                              onChange={(e) => field.onChange(parseFloat(e.target.value))}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <Button type="submit" className="w-full" disabled={isPending}>
                      {isPending ? "Placing bid..." : "Place Bid"}
                    </Button>
                  </form>
                </Form>
              </CardContent>
            </Card>
          )}

          {isOwnItem && isActive && (
            <Card>
              <CardContent className="pt-6">
                <p className="text-muted-foreground text-center">
                  This is your listing. You cannot bid on your own item.
                </p>
              </CardContent>
            </Card>
          )}

          {!isAuthenticated && !isOwnItem && isActive && !isEnded && (
            <Card>
              <CardContent className="pt-6">
                <p className="text-muted-foreground text-center mb-4">
                  Please sign in to place a bid
                </p>
                <Button asChild className="w-full">
                  <a href={`/login?redirect=/auctions/${item.id}`}>Sign In</a>
                </Button>
              </CardContent>
            </Card>
          )}

          {/* Bid History */}
          <Card>
            <CardHeader>
              <CardTitle>Bid History</CardTitle>
              <CardDescription>
                {bids.length} {bids.length === 1 ? "bid" : "bids"} placed
              </CardDescription>
            </CardHeader>
            <CardContent>
              {bids.length === 0 ? (
                <p className="text-muted-foreground text-center py-4">No bids yet</p>
              ) : (
                <div className="space-y-3">
                  {bids.map((bid) => (
                    <div key={bid.id} className="flex justify-between items-center border-b pb-3 last:border-0">
                      <div>
                        <p className="font-semibold">{formatPrice(bid.amount)}</p>
                        <p className="text-sm text-muted-foreground">{formatDate(bid.createdAt)}</p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
