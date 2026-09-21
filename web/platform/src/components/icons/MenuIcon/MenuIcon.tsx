import type { IconProps } from "../IconProps";

export function MenuIcon(props: Readonly<IconProps>) {
  return (
    <svg
      {...props}
      aria-hidden="true"
      data-icon="menu"
      fill="none"
      focusable="false"
      viewBox="0 0 24 24"
    >
      <path
        d="M4 6h16M4 12h16M4 18h16"
        stroke="currentColor"
        strokeLinecap="round"
        strokeWidth="1.5"
      />
    </svg>
  );
}
