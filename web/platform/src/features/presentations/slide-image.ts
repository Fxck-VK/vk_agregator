const jpegPrefix = "data:image/jpeg;base64,";
export const slideImageDataLimit = 64 * 1024;
export const slideImagePattern = /^data:image\/jpeg;base64,[A-Za-z0-9+/]+={0,2}$/;
export const slideImageTypes = ["image/jpeg", "image/png", "image/webp"];
export const slideImageUploadLimit = 8 * 1024 * 1024;

// Reserve space for the data URL header and Base64 padding in the tab draft.
const jpegByteLimit = Math.floor((slideImageDataLimit - jpegPrefix.length) / 4) * 3;

export async function renderSlideImage(source: string): Promise<Blob> {
  const image = new Image();
  image.src = source;
  await image.decode();
  if (image.naturalWidth < 1 || image.naturalHeight < 1) throw new Error("invalid slide image");

  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) throw new Error("slide image processing unavailable");

  for (const [width, height, quality] of [[650, 778, .85], [650, 778, .68], [440, 527, .72], [320, 383, .62]]) {
    canvas.width = width;
    canvas.height = height;
    const scale = Math.max(width / image.naturalWidth, height / image.naturalHeight);
    const scaledWidth = image.naturalWidth * scale;
    const scaledHeight = image.naturalHeight * scale;
    context.drawImage(image, (width - scaledWidth) * .55, (height - scaledHeight) * .67, scaledWidth, scaledHeight);
    const jpeg = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, "image/jpeg", quality));
    if (jpeg && jpeg.size > 0 && jpeg.size <= jpegByteLimit) return jpeg;
  }

  throw new Error("slide image could not be processed");
}

export async function prepareSlideImage(image: Blob): Promise<string> {
  const source = URL.createObjectURL(image);
  try {
    const jpeg = await renderSlideImage(source);
    return await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        if (typeof reader.result === "string") resolve(reader.result);
        else reject(new Error("slide image could not be read"));
      };
      reader.onerror = () => reject(new Error("slide image could not be read"));
      reader.readAsDataURL(jpeg);
    });
  } finally {
    URL.revokeObjectURL(source);
  }
}

export function slideImageBytes(image: string): Uint8Array<ArrayBuffer> {
  return Uint8Array.from(atob(image.slice(jpegPrefix.length)), char => char.charCodeAt(0));
}
