import type { IconProps } from "../IconProps";

export function CheckIcon(props: Readonly<IconProps>) {
  return (
    <svg
      {...props}
      aria-hidden="true"
      data-icon="check"
      fill="none"
      focusable="false"
      viewBox="0 0 24 24"
    >
      <path
        d="m5 12.5 4.25 4.25L19 7"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.9"
      />
    </svg>
  );
}
