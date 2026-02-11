import type { LucideIcon } from "lucide-react";
import { CalendarDays, Database, FileInput, LayoutDashboard, PlusSquare } from "lucide-react";

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
    title: "New Estimate",
    href: "/estimates/new",
    icon: PlusSquare,
    description: "Start a new moving estimate",
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
];
