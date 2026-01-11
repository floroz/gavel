"use client";

import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription,
} from "@/components/ui/form";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { type CreateItemInput, type Item } from "@/shared/api/items";
import { type ActionResult } from "@/shared/types";
import { z } from "zod";

const createItemFormSchema = z.object({
  title: z.string().min(1, "Title is required").max(200, "Title too long"),
  description: z.string().max(5000, "Description too long"),
  startPrice: z.number().min(1, "Start price must be at least 1"),
  durationDays: z.number().min(1, "Duration must be at least 1 day"),
  images: z.array(z.string().url("Invalid image URL")),
  category: z.string(),
});

type CreateItemFormInput = z.infer<typeof createItemFormSchema>;

type CreateItemFormProps = {
  createItemAction: (input: CreateItemInput) => Promise<ActionResult<Item>>;
};

export function CreateItemForm({ createItemAction }: CreateItemFormProps) {
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  const form = useForm<CreateItemFormInput>({
    resolver: zodResolver(createItemFormSchema),
    defaultValues: {
      title: "",
      description: "",
      startPrice: 1,
      durationDays: 7,
      images: [],
      category: "",
    },
  });

  async function onSubmit(data: CreateItemFormInput) {
    startTransition(async () => {
      // Calculate expiration date from duration
      const endAtDate = new Date();
      endAtDate.setDate(endAtDate.getDate() + data.durationDays);

      const result = await createItemAction({
        title: data.title,
        description: data.description,
        startPrice: data.startPrice * 100, // Convert dollars to cents
        endAt: endAtDate.toISOString(),
        images: data.images,
        category: data.category,
      });

      if (result.success) {
        toast.success("Item created successfully!");
        if (result.data) {
          router.push(`/auctions/${result.data.id}`);
        }
      } else {
        toast.error(result.error);
      }
    });
  }

  return (
    <div className="container max-w-2xl py-8">
      <Card>
        <CardHeader>
          <CardTitle>List New Item</CardTitle>
          <CardDescription>
            Create a new auction listing for your item
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
              <FormField
                control={form.control}
                name="title"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Title</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="Enter item title"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Description</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="Describe your item in detail..."
                        className="min-h-32"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="category"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Category (Optional)</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="e.g., Electronics, Collectibles, Art"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="startPrice"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Starting Price ($)</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="0.01"
                        min="0.01"
                        placeholder="0.00"
                        {...field}
                        onChange={(e) => field.onChange(parseFloat(e.target.value))}
                      />
                    </FormControl>
                    <FormDescription>
                      Minimum bid amount in dollars
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="durationDays"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Auction Duration</FormLabel>
                    <Select
                      onValueChange={(value) => field.onChange(Number(value))}
                      value={field.value?.toString()}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="Select duration" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="7">7 days</SelectItem>
                        <SelectItem value="15">15 days</SelectItem>
                        <SelectItem value="30">30 days</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      How long the auction will run
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="images"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Image URLs (Optional)</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="Enter image URLs, one per line"
                        className="min-h-24"
                        value={field.value.join("\n")}
                        onChange={(e) => {
                          const urls = e.target.value
                            .split("\n")
                            .map((url) => url.trim())
                            .filter((url) => url.length > 0);
                          field.onChange(urls);
                        }}
                      />
                    </FormControl>
                    <FormDescription>
                      Paste image URLs, one per line
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="flex gap-4">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => router.back()}
                  disabled={isPending}
                  className="flex-1"
                >
                  Cancel
                </Button>
                <Button type="submit" disabled={isPending} className="flex-1">
                  {isPending ? "Creating..." : "Create Listing"}
                </Button>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
