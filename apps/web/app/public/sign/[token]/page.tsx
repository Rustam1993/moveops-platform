import { PublicSignEstimate } from "@/components/estimates/public-sign-estimate";

export default async function PublicSignPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  return <PublicSignEstimate token={token} />;
}
