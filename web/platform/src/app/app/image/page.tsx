import { ImageGenerationPanel } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel";
import { ImageJobHistory } from "@/features/image-generation/ImageJobHistory/ImageJobHistory";

export default function ImagePage() {
  return (
    <>
      <ImageGenerationPanel />
      <ImageJobHistory />
    </>
  );
}
