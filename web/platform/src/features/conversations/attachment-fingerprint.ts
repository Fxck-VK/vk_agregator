// Fingerprints stay in memory for this message; file contents are unchanged.
export async function attachmentFingerprint(file: File, signal: AbortSignal): Promise<string> {
  signal.throwIfAborted();
  const reader = new FileReader();
  const abort = () => reader.abort();
  signal.addEventListener("abort", abort, { once: true });
  try {
    const bytes = await new Promise<ArrayBuffer>((resolve, reject) => {
      reader.onload = () => resolve(reader.result as ArrayBuffer);
      reader.onerror = () => reject(reader.error);
      reader.onabort = () => reject(new DOMException("Aborted", "AbortError"));
      reader.readAsArrayBuffer(file);
    });
    signal.throwIfAborted();
    const digest = await crypto.subtle.digest("SHA-256", new Uint8Array(bytes));
    signal.throwIfAborted();
    return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
  } finally {
    signal.removeEventListener("abort", abort);
  }
}
