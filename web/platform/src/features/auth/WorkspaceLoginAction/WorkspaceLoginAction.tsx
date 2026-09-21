"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import Link from "@/i18n/Link";

import { useOptionalWorkspaceLogout } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";

import styles from "./WorkspaceLoginAction.module.css";

type WorkspaceLoginActionProps = {
  placement: "header" | "sidebar";
};

export function WorkspaceLoginAction({ placement }: WorkspaceLoginActionProps) {
  const t = useDictionary();
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
        {t.login.submitLabel}
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
      {t.login.submitLabel}
    </Link>
  );
}
