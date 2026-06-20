import { useState } from "react";
import { downloadFromBlobUrl, neirohubArtifactFilename } from "../utils/artifactDownload";

type MediaResultPreviewProps = {
  kind: "image" | "video";
  src: string;
  prompt: string;
  jobId: string;
  onMediaError?: () => void;
};

const MEDIA_WARNING =
  "Сохраните файл сейчас — превью в приложении временное и может стать недоступным.";

export function MediaResultPreview({
  kind,
  src,
  prompt,
  jobId,
  onMediaError,
}: MediaResultPreviewProps) {
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);

  async function downloadMedia() {
    if (downloading) return;
    setDownloading(true);
    setDownloadError(null);
    try {
      const fallbackExt = kind === "video" ? "mp4" : "png";
      const filename = neirohubArtifactFilename(prompt, jobId, undefined, fallbackExt);
      await downloadFromBlobUrl(src, filename);
    } catch {
      setDownloadError("Не удалось скачать файл");
    } finally {
      setDownloading(false);
    }
  }

  const downloadLabel = kind === "video" ? "Скачать видео" : "Скачать изображение";

  return (
    <div className="media-result">
      <div className="media-result__frame">
        {kind === "image" ? (
          <img
            className="vk-preview__media media-result__media"
            src={src}
            alt="Готовый результат"
            onError={onMediaError}
          />
        ) : (
          <video
            className="vk-preview__media media-result__media"
            src={src}
            controls
            onError={onMediaError}
          />
        )}
        <button
          type="button"
          className="media-result__download"
          aria-label={downloadLabel}
          title={downloadLabel}
          disabled={downloading}
          onClick={() => void downloadMedia()}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M12 3v12m0 0 4-4m-4 4-4-4M4 19v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </div>
      <p className="media-result__warning" role="note">
        {MEDIA_WARNING}
      </p>
      {downloadError && <p className="media-result__error">{downloadError}</p>}
    </div>
  );
}
