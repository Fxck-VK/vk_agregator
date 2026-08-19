import type { IconProps } from "../IconProps";

export function CopyIcon(props: Readonly<IconProps>) {
  return (
    <svg
      {...props}
      aria-hidden="true"
      data-icon="copy"
      fill="none"
      focusable="false"
      viewBox="0 0 24 24"
    >
      <rect height="13" rx="2.5" stroke="currentColor" strokeWidth="1.8" width="11" x="8" y="8" />
      <path
        d="M16 8V6.5A2.5 2.5 0 0 0 13.5 4h-7A2.5 2.5 0 0 0 4 6.5v7A2.5 2.5 0 0 0 6.5 16H8"
        stroke="currentColor"
        strokeLinecap="round"
        strokeWidth="1.8"
      />
    </svg>
  );
}
