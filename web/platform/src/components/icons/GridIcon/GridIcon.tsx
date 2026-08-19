import type { IconProps } from "../IconProps";

export function GridIcon(props: Readonly<IconProps>) {
  return (
    <svg {...props} aria-hidden="true" data-icon="grid" fill="none" focusable="false" viewBox="0 0 24 24">
      <rect height="6.3" rx="2.2" stroke="currentColor" strokeWidth="1.7" width="6.3" x="4.1" y="4.1" />
      <rect height="6.3" rx="2.8" stroke="currentColor" strokeWidth="1.7" width="6.3" x="13.6" y="4.1" />
      <rect height="6.3" rx="2.8" stroke="currentColor" strokeWidth="1.7" width="6.3" x="4.1" y="13.6" />
      <path
        d="M16.75 13.75H18.35C19.21 13.75 19.9 14.44 19.9 15.3V18.35C19.9 19.21 19.21 19.9 18.35 19.9H15.3C14.44 19.9 13.75 19.21 13.75 18.35V16.75"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path d="M16.05 16.05L19.15 19.15" opacity="0.9" stroke="currentColor" strokeLinecap="round" strokeWidth="1.35" />
    </svg>
  );
}
