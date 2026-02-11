"use client";

import { Toaster } from "sonner";
import { useTheme } from "next-themes";

export function AppToaster() {
  const { theme = "system" } = useTheme();
  return <Toaster position="top-right" richColors theme={theme as "light" | "dark" | "system"} />;
}
