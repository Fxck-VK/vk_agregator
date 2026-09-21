import type { WebApiPath } from "./path";

export type BrowserRequestInit = RequestInit & {
  onUploadProgress?: (percentage: number | null) => void;
};
export type DevelopmentTransport = (path: WebApiPath, init?: BrowserRequestInit) => Promise<Response> | undefined;

let transport: DevelopmentTransport | undefined;

// Installed only by the server-gated local preview UI. Production can never
// activate this seam, even if a caller attempts to register a transport.
export function registerDevelopmentTransport(next: DevelopmentTransport) {
  if (process.env.NODE_ENV !== "development") return () => {};
  transport = next;
  return () => { if (transport === next) transport = undefined; };
}

export function developmentResponse(path: WebApiPath, init?: BrowserRequestInit) {
  if (process.env.NODE_ENV === "development") return transport?.(path, init);
}
