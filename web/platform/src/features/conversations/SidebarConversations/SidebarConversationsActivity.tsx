"use client";

import { createContext, type ReactNode, useContext } from "react";

type SidebarConversationsActivity = {
  isActive: boolean;
  session: number;
};

const SidebarConversationsActivityContext = createContext<SidebarConversationsActivity>({ isActive: true, session: 0 });

type SidebarConversationsActivityProviderProps = {
  children: ReactNode;
  isActive: boolean;
  session: number;
};

export function SidebarConversationsActivityProvider({ children, isActive, session }: SidebarConversationsActivityProviderProps) {
  return (
    <SidebarConversationsActivityContext.Provider value={{ isActive, session }}>
      {children}
    </SidebarConversationsActivityContext.Provider>
  );
}

export function useSidebarConversationsActive() {
  return useContext(SidebarConversationsActivityContext);
}
