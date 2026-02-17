import { PublicInventoryEditor } from "@/components/estimates/public-inventory-editor";

export default async function PublicInventoryPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  return <PublicInventoryEditor token={token} />;
}
