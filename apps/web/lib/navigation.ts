import type { LucideIcon } from "lucide-react";
import {
  Briefcase,
  CalendarDays,
  ClipboardList,
  Database,
  FileInput,
  FileText,
  LayoutDashboard,
  PlusSquare,
  Settings2,
} from "lucide-react";

export type NavItem = {
  title: string;
  href: string;
  icon: LucideIcon;
  description: string;
};

export const navItems: NavItem[] = [
  {
    title: "Dashboard",
    href: "/",
    icon: LayoutDashboard,
    description: "Overview of your operations workspace",
  },
  {
    title: "Estimates",
    href: "/estimates",
    icon: FileText,
    description: "View and manage saved estimates",
  },
  {
    title: "New Estimate",
    href: "/estimates/new",
    icon: PlusSquare,
    description: "Start a new moving estimate",
  },
  {
    title: "Jobs",
    href: "/jobs",
    icon: Briefcase,
    description: "View and manage converted jobs",
  },
  {
    title: "Calendar",
    href: "/calendar",
    icon: CalendarDays,
    description: "Track upcoming jobs and schedules",
  },
  {
    title: "Storage",
    href: "/storage",
    icon: Database,
    description: "Manage storage records and status",
  },
  {
    title: "Import / Export",
    href: "/import",
    icon: FileInput,
    description: "Migrate and export tenant data",
  },
  {
    title: "New Estimate Admin",
    href: "/admin/new-estimate/catalog",
    icon: Settings2,
    description: "Tenant settings for catalog, pricing, templates, and metrics",
  },
  {
    title: "Audit Logs",
    href: "/admin/audit-logs",
    icon: ClipboardList,
    description: "Review tenant-scoped audit activity",
  },
];
