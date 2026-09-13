import Image from "next/image";
import type { ButtonHTMLAttributes } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { Tooltip } from "@/components/ui/Tooltip/Tooltip";

import styles from "./ChatSubmitButton.module.css";

type ChatSubmitButtonProps = {
  disabled: boolean;
  label: string;
  type?: ButtonHTMLAttributes<HTMLButtonElement>["type"];
};

export function ChatSubmitButton({
  disabled,
  label,
  type = "submit",
}: ChatSubmitButtonProps) {
  return (
    <Tooltip label={label}>
      <button
        aria-label={label}
        className={styles.button}
        data-ui="chat-submit-button"
        disabled={disabled}
        type={type}
      >
        <Image
          alt=""
          aria-hidden="true"
          height={24}
          src={assetPaths.icons.ui.sendMessage}
          width={24}
        />
      </button>
    </Tooltip>
  );
}
