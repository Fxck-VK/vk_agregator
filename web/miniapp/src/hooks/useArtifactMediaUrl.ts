import { useEffect, useState } from "react";
import { artifactMediaUrl } from "../api/client";

/** Authenticated artifact URL for <img>/<video> (browser tags cannot send headers). */
export function useArtifactMediaUrl(artifactId: string | undefined): string | null {
  const [src, setSrc] = useState<string | null>(null);

  useEffect(() => {
    if (!artifactId) {
      setSrc(null);
      return;
    }
    let cancelled = false;
    void artifactMediaUrl(artifactId).then((url) => {
      if (!cancelled) setSrc(url);
    });
    return () => {
      cancelled = true;
    };
  }, [artifactId]);

  return src;
}
