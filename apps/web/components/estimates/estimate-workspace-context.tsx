"use client";

import { createContext, useContext } from "react";

import type { Estimate } from "@/lib/phase2-api";

type EstimateWorkspaceContextValue = {
  estimate: Estimate;
  setEstimate: (next: Estimate) => void;
};

const EstimateWorkspaceContext = createContext<EstimateWorkspaceContextValue | null>(null);

export function EstimateWorkspaceProvider({
  value,
  children,
}: {
  value: EstimateWorkspaceContextValue;
  children: React.ReactNode;
}) {
  return <EstimateWorkspaceContext.Provider value={value}>{children}</EstimateWorkspaceContext.Provider>;
}

export function useEstimateWorkspace() {
  const context = useContext(EstimateWorkspaceContext);
  if (!context) {
    throw new Error("useEstimateWorkspace must be used inside EstimateWorkspaceProvider");
  }
  return context;
}
