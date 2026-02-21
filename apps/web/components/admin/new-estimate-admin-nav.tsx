"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { cn } from "@/lib/utils";

const links = [
  { href: "/admin/new-estimate/catalog", label: "Catalog" },
  { href: "/admin/new-estimate/pricing", label: "Pricing Defaults" },
  { href: "/admin/new-estimate/email-templates", label: "Email Templates" },
  { href: "/admin/new-estimate/documents", label: "Document Branding" },
  { href: "/admin/new-estimate/metrics", label: "Metrics" },
  { href: "/admin/audit-logs", label: "Audit Logs" },
] as const;

export function NewEstimateAdminNav() {
  const pathname = usePathname();

  return (
    <div className="overflow-x-auto">
      <nav className="flex min-w-max gap-1 rounded-lg border border-border/70 bg-card/50 p-1">
        {links.map((link) => {
          const active = pathname === link.href || (link.href !== "/admin/audit-logs" && pathname.startsWith(link.href));
          return (
            <Link
              key={link.href}
              href={link.href}
              className={cn(
                "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-primary text-primary-foreground shadow-sm"
                  : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
              )}
            >
              {link.label}
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
