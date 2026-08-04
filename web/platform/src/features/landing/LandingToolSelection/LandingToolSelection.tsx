"use client";

import { createContext, type ReactNode, useContext, useState } from "react";

import { landingTools } from "../landing-content";
import type { LandingTool } from "../landing-contracts";

type LandingToolSelection = {
  selectedTool: LandingTool;
  selectTool: (toolId: string) => void;
};

const defaultTool = landingTools[0];
const LandingToolSelectionContext = createContext<LandingToolSelection>({
  selectedTool: defaultTool,
  selectTool: () => undefined,
});

export function LandingToolSelectionProvider({ children }: { children: ReactNode }) {
  const [selectedToolId, setSelectedToolId] = useState(defaultTool.id);
  const selectedTool = landingTools.find(({ id }) => id === selectedToolId) ?? defaultTool;

  const selectTool = (toolId: string) => {
    if (landingTools.some(({ id }) => id === toolId)) setSelectedToolId(toolId);
  };

  return (
    <LandingToolSelectionContext.Provider value={{ selectedTool, selectTool }}>
      {children}
    </LandingToolSelectionContext.Provider>
  );
}

export function useLandingToolSelection(): LandingToolSelection {
  return useContext(LandingToolSelectionContext);
}
