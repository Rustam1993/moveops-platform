# UI Guidelines

## Purpose
This document defines the Phase 1 web UI structure and patterns for a consistent, modern dashboard experience.

## Layout structure
- App shell is implemented in `apps/web/components/layout/app-shell.tsx`.
- Desktop layout:
  - fixed left sidebar for primary navigation
  - sticky top header for global actions
- Mobile layout:
  - sidebar opens in a sheet/drawer from hamburger button
  - header keeps search and account actions accessible

## Navigation conventions
Primary routes in sidebar and command palette:
- `/` Dashboard
- `/estimates/new` New Estimate
- `/calendar` Calendar
- `/storage` Storage
- `/import` Import / Export

Conventions:
- page-level heading always includes title + short description
- placeholder pages must still provide a polished empty state and a primary CTA

## Component usage patterns (shadcn style)
- UI primitives live in `apps/web/components/ui/*`.
- Layout composition lives in `apps/web/components/layout/*`.
- Prefer composition over page-specific ad hoc styles.
- Reuse these baseline components for consistency:
  - `Button`, `Card`, `Input`, `Label`
  - `DropdownMenu`, `Sheet`, `Dialog`, `Command`
  - `Skeleton` for loading states

## Theming and dark mode
- Theme provider is configured in root layout via `next-themes`.
- Tailwind uses class-based dark mode (`darkMode: ["class"]`).
- Design tokens are defined in `apps/web/app/globals.css` as CSS variables.
- Components should consume semantic token classes (`bg-card`, `text-muted-foreground`, `border-border`) rather than hardcoded colors.

## Command palette
- Implemented as a keyboard-first navigation and quick-action surface.
- Triggered by:
  - Cmd+K (macOS)
  - Ctrl+K (Windows/Linux)
  - header search button
- Groups:
  - Navigation
  - Quick Search (placeholder action)
  - Actions (logout)

## Accessibility baseline
- Interactive controls include visible focus styles.
- Dialogs/sheets support escape close and focus management.
- Menu and command interactions are keyboard navigable.
