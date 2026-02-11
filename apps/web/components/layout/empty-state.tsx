import { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function EmptyState({
  title,
  description,
  ctaLabel,
  icon,
}: {
  title: string;
  description: string;
  ctaLabel: string;
  icon?: ReactNode;
}) {
  return (
    <Card className="border-dashed">
      <CardHeader>
        <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/50">{icon}</div>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button>{ctaLabel}</Button>
      </CardContent>
    </Card>
  );
}
