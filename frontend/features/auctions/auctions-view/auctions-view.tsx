
import { ItemCard } from "../item-card";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Item } from "@/shared/api";
import { CurrentUser } from "@/lib/auth";

type Props = {
  items: Item[];
  currentUser: CurrentUser | null;
}

export async function AuctionsView({ items, currentUser }: Props) {
 
  return (
    <div className="container py-8">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold">Auctions</h1>
          <p className="text-muted-foreground mt-2">
            Browse active auction items
          </p>
        </div>
        <Button asChild>
          <Link href="/auctions/new">Sell Item</Link>
        </Button>
      </div>

      {items.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-muted-foreground">No auction items available yet.</p>
          <Button asChild className="mt-4">
            <Link href="/auctions/new">List your first item</Link>
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
          {items.map((item) => (
            <ItemCard key={item.id} item={item} currentUserId={currentUser?.userId} />
          ))}
        </div>
      )}
    </div>
  );
}
