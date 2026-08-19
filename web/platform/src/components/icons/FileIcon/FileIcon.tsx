import type { IconProps } from "../IconProps";

export function FileIcon(props: Readonly<IconProps>) {
  return (
    <svg {...props} aria-hidden="true" data-icon="file" fill="none" focusable="false" viewBox="0 0 24 24">
      <path
        d="M7.1 3.8H14.2C14.73 3.8 15.24 4.01 15.61 4.39L18.61 7.39C18.99 7.76 19.2 8.27 19.2 8.8V16.9C19.2 18.7 17.74 20.15 15.95 20.15H7.1C5.31 20.15 3.85 18.7 3.85 16.9V7.05C3.85 5.26 5.31 3.8 7.1 3.8Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path
        d="M14.4 4V7.15C14.4 8.09 15.16 8.85 16.1 8.85H19.05"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path d="M7.8 11.15H15.3" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
      <path d="M7.8 14.15H14.1" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
      <path d="M7.8 17.15H11.7" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </svg>
  );
}
