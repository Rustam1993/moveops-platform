import { redirect } from "next/navigation";

export default async function EstimateRootRedirect({
  params,
}: {
  params: Promise<{ estimateId: string }>;
}) {
  const { estimateId } = await params;
  redirect(`/estimates/${estimateId}/entry`);
}
