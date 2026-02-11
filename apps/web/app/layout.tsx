import type { Metadata } from "next";

import { ThemeProvider } from "@/components/providers/theme-provider";
import { AppToaster } from "@/components/ui/toaster";
import "./globals.css";

export const metadata: Metadata = {
  title: "MoveOps",
  description: "MoveOps platform foundation",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="font-sans antialiased">
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
          {children}
          <AppToaster />
        </ThemeProvider>
      </body>
    </html>
  );
}
