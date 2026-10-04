/** A fresh document prevents a prefetched guest layout surviving cookie sign-in. */
export function replaceSignInDocument(target: string): void {
  window.location.replace(target);
}
