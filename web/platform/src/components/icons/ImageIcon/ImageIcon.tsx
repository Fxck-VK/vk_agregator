import type { IconProps } from "../IconProps";

export function ImageIcon(props: Readonly<IconProps>) {
  return (
    <svg {...props} aria-hidden="true" data-icon="image" fill="none" focusable="false" viewBox="0 0 24 24">
      <rect height="15.6" rx="3.4" stroke="currentColor" strokeWidth="1.7" width="16.2" x="3.9" y="4.2" />
      <circle cx="15.9" cy="8.6" r="1.95" stroke="currentColor" strokeWidth="1.7" />
      <path
        d="M5.2 16.5L8.4 13.5C9.02 12.92 9.98 12.93 10.58 13.52L11.47 14.4C12.03 14.96 12.91 15.01 13.54 14.52L14.77 13.56C15.42 13.05 16.34 13.11 16.92 13.69L18.8 15.57"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path d="M5.25 16.55L6.55 17.85H17.6" opacity="0.9" stroke="currentColor" strokeLinecap="round" strokeWidth="1.35" />
    </svg>
  );
}
