import { ShieldAlert } from "lucide-react";

import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function NotAuthorizedState({ message }: { message?: string }) {
  return (
    <Card className="border-destructive/40">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <ShieldAlert className="h-5 w-5 text-destructive" />
          Not authorized
        </CardTitle>
        <CardDescription>{message ?? "You do not have permission to view this content."}</CardDescription>
      </CardHeader>
    </Card>
  );
}
