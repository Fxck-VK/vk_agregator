"use client";

import { useRouter } from "next/navigation";
import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { clearPendingConversationPrompts } from "@/features/conversations/pending-conversation-prompt";
import { clearPendingConversationBootstraps } from "@/features/conversations/pending-conversation-bootstrap";
import { clearPendingConversationTitleSyncs } from "@/features/conversations/pending-conversation-title-sync";
import { ru } from "@/i18n/ru";

import { requestWorkspaceLogout } from "./workspace-logout-request";
import styles from "./WorkspaceLogoutBoundary.module.css";

type WorkspaceLogoutPhase = "authenticated" | "pending" | "confirmed" | "failed";

type WorkspaceLogoutController = {
  logout: () => void;
  phase: WorkspaceLogoutPhase;
  requestLogin: () => void;
  retry: () => void;
};

type WorkspaceLogoutMessage = {
  type: "logout-confirmed" | "logout-failed" | "logout-started";
};

type WorkspaceLogoutBoundaryProps = {
  children: ReactNode;
  guest: ReactNode;
};

const WorkspaceLogoutContext = createContext<WorkspaceLogoutController | undefined>(undefined);
const workspaceLogoutChannelName = "neirohub.workspace-session";

function isWorkspaceLogoutMessage(value: unknown): value is WorkspaceLogoutMessage {
  if (typeof value !== "object" || value === null || !("type" in value)) {
    return false;
  }
  return value.type === "logout-started" || value.type === "logout-confirmed" || value.type === "logout-failed";
}

function clearPrivateBrowserState() {
  clearPendingConversationPrompts();
  clearPendingConversationBootstraps();
  clearPendingConversationTitleSyncs();
}

export function WorkspaceLogoutBoundary({ children, guest }: WorkspaceLogoutBoundaryProps) {
  const router = useRouter();
  const [phase, setPhase] = useState<WorkspaceLogoutPhase>("authenticated");
  const phaseRef = useRef<WorkspaceLogoutPhase>("authenticated");
  const requestGenerationRef = useRef(0);
  const requestInFlightRef = useRef(false);
  const loginRequestedRef = useRef(false);
  const channelRef = useRef<BroadcastChannel | null>(null);

  const transition = useCallback((nextPhase: WorkspaceLogoutPhase) => {
    phaseRef.current = nextPhase;
    setPhase(nextPhase);
  }, []);

  const publish = useCallback((message: WorkspaceLogoutMessage) => {
    try {
      channelRef.current?.postMessage(message);
    } catch {
      // Cross-tab coordination is optional and must not block local logout.
    }
  }, []);

  const navigateAfterConfirmation = useCallback(() => {
    router.replace(loginRequestedRef.current ? "/login" : "/app");
    router.refresh();
  }, [router]);

  const runLogout = useCallback(() => {
    if (requestInFlightRef.current) {
      return;
    }

    requestInFlightRef.current = true;
    const requestGeneration = requestGenerationRef.current + 1;
    requestGenerationRef.current = requestGeneration;
    clearPrivateBrowserState();
    transition("pending");
    publish({ type: "logout-started" });

    void requestWorkspaceLogout().then(() => {
      if (requestGenerationRef.current !== requestGeneration) {
        return;
      }
      requestInFlightRef.current = false;
      transition("confirmed");
      publish({ type: "logout-confirmed" });
      navigateAfterConfirmation();
    }).catch(() => {
      if (requestGenerationRef.current !== requestGeneration) {
        return;
      }
      requestInFlightRef.current = false;
      transition("failed");
      publish({ type: "logout-failed" });
    });
  }, [navigateAfterConfirmation, publish, transition]);

  const requestLogin = useCallback(() => {
    loginRequestedRef.current = true;
    if (phaseRef.current === "confirmed") {
      navigateAfterConfirmation();
      return;
    }
    if (phaseRef.current === "failed") {
      runLogout();
    }
  }, [navigateAfterConfirmation, runLogout]);

  useEffect(() => {
    if (typeof BroadcastChannel === "undefined") {
      return;
    }

    let channel: BroadcastChannel;
    try {
      channel = new BroadcastChannel(workspaceLogoutChannelName);
      channelRef.current = channel;
    } catch {
      return;
    }

    channel.onmessage = (event: MessageEvent<unknown>) => {
      if (!isWorkspaceLogoutMessage(event.data)) {
        return;
      }

      requestGenerationRef.current += 1;
      requestInFlightRef.current = false;
      if (event.data.type === "logout-started") {
        clearPrivateBrowserState();
        transition("pending");
        return;
      }
      if (event.data.type === "logout-failed") {
        transition("failed");
        return;
      }
      transition("confirmed");
      navigateAfterConfirmation();
    };

    return () => {
      channel.close();
      if (channelRef.current === channel) {
        channelRef.current = null;
      }
    };
  }, [navigateAfterConfirmation, transition]);

  const controller = useMemo<WorkspaceLogoutController>(() => ({
    logout: runLogout,
    phase,
    requestLogin,
    retry: runLogout,
  }), [phase, requestLogin, runLogout]);

  return (
    <WorkspaceLogoutContext.Provider value={controller}>
      {phase === "authenticated" ? children : guest}
      {phase === "pending" ? (
        <div aria-hidden="true" className={styles.transitionVeil} data-testid="workspace-logout-transition" />
      ) : null}
      {phase === "failed" ? (
        <div aria-live="polite" className={styles.failureNotice} role="status">
          <span>{ru.account.logoutServerFailure}</span>
          <button className={styles.retryButton} onClick={runLogout} type="button">
            {ru.account.logoutRetryLabel}
          </button>
        </div>
      ) : null}
    </WorkspaceLogoutContext.Provider>
  );
}

export function useOptionalWorkspaceLogout(): WorkspaceLogoutController | undefined {
  return useContext(WorkspaceLogoutContext);
}

export function useWorkspaceLogout(): WorkspaceLogoutController {
  const controller = useOptionalWorkspaceLogout();
  if (controller === undefined) {
    throw new Error("useWorkspaceLogout must be used within WorkspaceLogoutBoundary.");
  }
  return controller;
}
