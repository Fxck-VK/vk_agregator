import { useEffect, useState } from "react";
import { artifactMediaUrl } from "../api/client";

/** Authenticated artifact blob URL for <img>/<video>. */
export function useArtifactMediaUrl(artifactId: string | undefined): string | null {
  const [src, setSrc] = useState<string | null>(null);

  useEffect(() => {
    if (!artifactId) {
      setSrc(null);
      return;
    }
    let cancelled = false;
    let objectUrl: string | null = null;
    setSrc(null);
    void artifactMediaUrl(artifactId).then((url) => {
      if (cancelled) {
        if (url?.startsWith("blob:")) URL.revokeObjectURL(url);
        return;
      }
      objectUrl = url;
      setSrc(url);
    });
    return () => {
      cancelled = true;
      if (objectUrl?.startsWith("blob:")) URL.revokeObjectURL(objectUrl);
    };
  }, [artifactId]);

  return src;
}
