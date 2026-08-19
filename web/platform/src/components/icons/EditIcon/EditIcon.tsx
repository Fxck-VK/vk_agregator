import type { IconProps } from "../IconProps";

export function EditIcon(props: Readonly<IconProps>) {
  return (
    <svg {...props} aria-hidden="true" data-icon="edit" fill="none" focusable="false" viewBox="0 0 24 24">
      <path
        d="M7.2 4.5H14.6C15.15 4.5 15.67 4.72 16.06 5.11L18.39 7.44C18.78 7.83 19 8.35 19 8.9V16.8C19 18.57 17.57 20 15.8 20H7.2C5.43 20 4 18.57 4 16.8V7.7C4 5.93 5.43 4.5 7.2 4.5Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path
        d="M15 4.7V7.1C15 8.04 15.76 8.8 16.7 8.8H19"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path
        d="M8 15.8L8.64 13.47C8.72 13.16 8.88 12.88 9.11 12.65L14.42 7.34C15.05 6.71 16.07 6.71 16.7 7.34C17.33 7.97 17.33 8.99 16.7 9.62L11.39 14.93C11.16 15.16 10.88 15.32 10.57 15.4L8 15.8Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path d="M13.4 8.35L15.7 10.65" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
      <path d="M7.9 9.1H10.3" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </svg>
  );
}
