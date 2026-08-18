import Link from "next/link";

import { ru } from "@/i18n/ru";

import styles from "./WorkspaceLoginAction.module.css";

type WorkspaceLoginActionProps = {
  placement: "header" | "sidebar";
};

export function WorkspaceLoginAction({ placement }: WorkspaceLoginActionProps) {
  return (
    <Link
      className={`${styles.action} ${styles[placement]}`}
      data-placement={placement}
      href="/login"
      prefetch
    >
      {ru.login.submitLabel}
    </Link>
  );
}
