import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function WorkspacePlaceholder({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <Card className="border-border/70">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="rounded-lg border border-dashed border-border/80 bg-muted/10 p-6 text-sm text-muted-foreground">
          Coming soon in Phase 2/3. The estimate workspace route and shell are ready.
        </div>
      </CardContent>
    </Card>
  );
}
