"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { LogOut, Search } from "lucide-react";
import { toast } from "sonner";

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { navItems } from "@/lib/navigation";
import { logout } from "@/lib/session";

type Props = {
  open: boolean;
  setOpen: (open: boolean) => void;
};

export function CommandPalette({ open, setOpen }: Props) {
  const router = useRouter();

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(!open);
      }
    };
    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, [open, setOpen]);

  async function onLogout() {
    try {
      await logout();
      toast.success("Signed out");
      setOpen(false);
      router.push("/login");
      router.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Logout failed");
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="overflow-hidden p-0">
        <DialogTitle className="sr-only">Command Palette</DialogTitle>
        <Command>
          <CommandInput placeholder="Search jobs/customers or jump to a page..." />
          <CommandList>
            <CommandEmpty>No results found.</CommandEmpty>
            <CommandGroup heading="Navigation">
              {navItems.map((item) => {
                const Icon = item.icon;
                return (
                  <CommandItem
                    key={item.href}
                    onSelect={() => {
                      setOpen(false);
                      router.push(item.href);
                    }}
                  >
                    <Icon className="h-4 w-4" />
                    <span>{item.title}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup heading="Quick Search">
              <CommandItem
                onSelect={() => {
                  setOpen(false);
                  toast.info("Search will be available in the next phase.");
                }}
              >
                <Search className="h-4 w-4" />
                <span>Search jobs/customers...</span>
                <CommandShortcut>Soon</CommandShortcut>
              </CommandItem>
            </CommandGroup>
            <CommandSeparator />
            <CommandGroup heading="Actions">
              <CommandItem onSelect={onLogout}>
                <LogOut className="h-4 w-4" />
                <span>Logout</span>
              </CommandItem>
            </CommandGroup>
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  );
}
