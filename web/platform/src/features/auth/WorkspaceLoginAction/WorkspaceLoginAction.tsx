import Link from "next/link";

import { useOptionalWorkspaceLogout } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import { ru } from "@/i18n/ru";

import styles from "./WorkspaceLoginAction.module.css";

type WorkspaceLoginActionProps = {
  placement: "header" | "sidebar";
};

export function WorkspaceLoginAction({ placement }: WorkspaceLoginActionProps) {
  const workspaceLogout = useOptionalWorkspaceLogout();
  const className = `${styles.action} ${styles[placement]}`;

  if (workspaceLogout && workspaceLogout.phase !== "authenticated") {
    return (
      <button
        className={className}
        data-placement={placement}
        onClick={workspaceLogout.requestLogin}
        type="button"
      >
        {ru.login.submitLabel}
      </button>
    );
  }

  return (
    <Link
      className={className}
      data-placement={placement}
      href="/login"
      prefetch
    >
      {ru.login.submitLabel}
    </Link>
  );
}
