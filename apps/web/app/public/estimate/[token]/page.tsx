import { PublicEstimateViewer } from "@/components/estimates/public-estimate-viewer";

export default async function PublicEstimatePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  return <PublicEstimateViewer token={token} />;
}
