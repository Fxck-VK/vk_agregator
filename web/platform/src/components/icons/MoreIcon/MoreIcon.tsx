import type { IconProps } from "../IconProps";

export function MoreIcon(props: Readonly<IconProps>) {
  return (
    <svg
      {...props}
      aria-hidden="true"
      data-icon="more"
      fill="currentColor"
      focusable="false"
      viewBox="0 0 24 24"
    >
      <circle cx="5" cy="12" r="1.5" />
      <circle cx="12" cy="12" r="1.5" />
      <circle cx="19" cy="12" r="1.5" />
    </svg>
  );
}
