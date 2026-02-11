"use client";

import { Menu, Search } from "lucide-react";
import { useState } from "react";

import { CommandPalette } from "@/components/layout/command-palette";
import { SidebarNav } from "@/components/layout/sidebar-nav";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { UserMenu } from "@/components/layout/user-menu";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { type SessionPayload } from "@/lib/session";

export function AppShell({ children, session }: { children: React.ReactNode; session: SessionPayload | null }) {
  const [openCommand, setOpenCommand] = useState(false);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <CommandPalette open={openCommand} setOpen={setOpenCommand} />
      <div className="mx-auto flex w-full max-w-[1600px]">
        <aside className="sticky top-0 hidden h-screen w-72 flex-col border-r border-border/70 bg-card/40 p-4 backdrop-blur lg:flex">
          <div className="mb-4 px-3 py-2">
            <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">MoveOps</p>
            <h2 className="text-lg font-semibold">Operations Portal</h2>
          </div>
          <SidebarNav />
        </aside>

        <div className="flex min-h-screen flex-1 flex-col">
          <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-border/70 bg-background/90 px-4 backdrop-blur md:px-6">
            <Sheet>
              <SheetTrigger asChild>
                <Button variant="outline" size="icon" className="lg:hidden" aria-label="Open navigation">
                  <Menu className="h-4 w-4" />
                </Button>
              </SheetTrigger>
              <SheetContent>
                <div className="mb-6 mt-4 px-1">
                  <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">MoveOps</p>
                  <h2 className="text-lg font-semibold">Operations Portal</h2>
                </div>
                <SidebarNav />
              </SheetContent>
            </Sheet>

            <Button
              variant="outline"
              size="icon"
              className="sm:hidden"
              onClick={() => setOpenCommand(true)}
              aria-label="Open search"
            >
              <Search className="h-4 w-4" />
            </Button>

            <div className="hidden items-center gap-2 sm:flex">
              <Button variant="outline" className="w-[320px] justify-start text-muted-foreground" onClick={() => setOpenCommand(true)}>
                <Search className="mr-2 h-4 w-4" />
                Search
                <span className="ml-auto rounded border border-border px-1.5 py-0.5 text-[10px] tracking-wider">⌘K</span>
              </Button>
            </div>

            <div className="ml-auto flex items-center gap-2">
              <ThemeToggle />
              <Separator orientation="vertical" className="h-6" />
              <UserMenu session={session} />
            </div>
          </header>

          <main className="flex-1 p-4 md:p-6">{children}</main>
        </div>
      </div>
    </div>
  );
}
