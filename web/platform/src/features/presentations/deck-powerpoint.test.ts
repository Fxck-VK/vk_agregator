// @vitest-environment node

import { expect, it } from "vitest";
import { powerpoint } from "./deck-powerpoint";

async function readPowerPointParts(file: Blob) {
  const bytes = await file.arrayBuffer();
  const view = new DataView(bytes);
  const decoder = new TextDecoder();
  const parts = new Map<string, Uint8Array<ArrayBuffer>>();
  let offset = 0;
  while (offset + 30 <= view.byteLength && view.getUint32(offset, true) === 0x04034b50) {
    const size = view.getUint32(offset + 18, true);
    const nameLength = view.getUint16(offset + 26, true);
    const extraLength = view.getUint16(offset + 28, true);
    const name = decoder.decode(new Uint8Array(bytes, offset + 30, nameLength));
    const data = offset + 30 + nameLength + extraLength;
    parts.set(name, new Uint8Array(bytes, data, size));
    offset = data + size;
  }
  return parts;
}

it("exports distinct pictures with the correct slides and preserves editable text", async () => {
  const coverPhoto = Uint8Array.of(255, 216, 1, 255, 217);
  const detailPhoto = Uint8Array.of(255, 216, 2, 255, 217);
  const file = powerpoint([
    { title: "Обложка", points: ["Введение"] },
    { title: "Без изображения", points: ["Описание"] },
    { title: "Детали & примеры", points: ["Геометрия"] },
  ], "Учебный", [coverPhoto, undefined, detailPhoto]);
  const parts = await readPowerPointParts(file);
  const decoder = new TextDecoder();
  const part = (path: string) => decoder.decode(parts.get(path));

  function linkedPicture(slideNumber: number) {
    const slide = part(`ppt/slides/slide${slideNumber}.xml`);
    const rels = part(`ppt/slides/_rels/slide${slideNumber}.xml.rels`);
    const target = rels.match(/Id="rId2"[^>]*\/image" Target="\.\.\/(media\/[^"]+\.jpg)"/u)?.[1];
    expect(slide).toMatch(/<p:pic>.*r:embed="rId2".*<\/p:pic>/u);
    expect(slide).toContain('<a:off x="6461760" y="0"/><a:ext cx="5730240" cy="6858000"/>');
    return target ? parts.get(`ppt/${target}`) : undefined;
  }

  expect(linkedPicture(3)).toEqual(detailPhoto);
  expect(linkedPicture(1)).toEqual(coverPhoto);
  expect(part("ppt/slides/slide2.xml")).not.toContain("<p:pic>");
  expect(part("ppt/slides/_rels/slide2.xml.rels")).not.toContain("/image");
  expect(part("ppt/slides/slide2.xml")).toContain('<a:ext cx="10800000"');
  expect(part("ppt/slides/slide3.xml")).toMatch(/<p:sp>.*<a:t>Детали &amp; примеры<\/a:t>.*<\/p:sp>/u);
});
