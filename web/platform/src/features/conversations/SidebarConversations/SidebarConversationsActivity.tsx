"use client";

import { createContext, type ReactNode, useContext } from "react";

type SidebarConversationsActivity = {
  isActive: boolean;
  onPendingPanelChange?: (conversationId: string, isPending: boolean, session: number) => void;
  session: number;
};

const SidebarConversationsActivityContext = createContext<SidebarConversationsActivity>({ isActive: true, session: 0 });

type SidebarConversationsActivityProviderProps = {
  children: ReactNode;
  isActive: boolean;
  onPendingPanelChange?: (conversationId: string, isPending: boolean, session: number) => void;
  session: number;
};

export function SidebarConversationsActivityProvider({ children, isActive, onPendingPanelChange, session }: SidebarConversationsActivityProviderProps) {
  return (
    <SidebarConversationsActivityContext.Provider value={{ isActive, onPendingPanelChange, session }}>
      {children}
    </SidebarConversationsActivityContext.Provider>
  );
}

export function useSidebarConversationsActive() {
  return useContext(SidebarConversationsActivityContext);
}
