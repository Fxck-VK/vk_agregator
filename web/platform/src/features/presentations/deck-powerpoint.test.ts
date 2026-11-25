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

const appearanceSlides = [
  { title: "Обложка", points: ["Введение"] },
  { title: "Конструкция & сборка", points: ["Эскиз", "Модель", "Прототип"] },
];

it("exports the chosen frame as the editable picture's exact slide coordinates", async () => {
  const picture = Uint8Array.of(255, 216, 1, 255, 217);
  const parts = await readPowerPointParts(powerpoint([
    appearanceSlides[0],
    { ...appearanceSlides[1], imageFrame: { x: 2500, y: 2000, width: 4000, height: 3000 } },
  ], "Учебный", [undefined, picture]));
  const slide = new TextDecoder().decode(parts.get("ppt/slides/slide2.xml"));
  expect(slide).toMatch(/<p:pic>.*<a:off x="3048000" y="1371600"\/><a:ext cx="4876800" cy="2057400"\/>.*<\/p:pic>/u);
  expect(parts.get("ppt/media/slide2.jpg")).toEqual(picture);
});

it("applies the selected typeface, colors and native fade without animating the text", async () => {
  const presentation = powerpoint(appearanceSlides, "Учебный", [], {
    font: "georgia", background: "dark", animation: "off", transition: "fade",
    colors: { background: "#123abc", text: "#fedcba" },
  });
  const parts = await readPowerPointParts(presentation);
  const decoder = new TextDecoder();
  const slide = decoder.decode(parts.get("ppt/slides/slide2.xml"));
  expect(slide).toContain('<p:bg><p:bgPr><a:solidFill><a:srgbClr val="123ABC"/>');
  expect(slide.match(/<a:srgbClr val="FEDCBA"\/>/gu)).toHaveLength(5);
  expect(slide.match(/<a:latin typeface="Georgia"\/>/gu)).toHaveLength(5);
  expect(slide).toMatch(/<\/p:clrMapOvr><p:transition[^>]*><p:fade\/><\/p:transition><\/p:sld>$/u);
  expect(slide).not.toContain("<p:timing>");
});

it("writes one native PowerPoint click to reveal each editable bullet in slide order", async () => {
  const presentation = powerpoint(appearanceSlides, "Учебный", [], {
    font: "arial", background: "original", animation: "on", transition: "push", colors: {},
  });
  const parts = await readPowerPointParts(presentation);
  const decoder = new TextDecoder();
  const slide = decoder.decode(parts.get("ppt/slides/slide2.xml"));
  expect(slide).toMatch(/<\/p:clrMapOvr><p:transition[^>]*><p:push dir="l"\/><\/p:transition><p:timing>/u);
  expect(slide).not.toContain("advTm=");
  expect(slide.match(/nodeType="clickEffect"/gu)).toHaveLength(3);
  expect(slide.match(/delay="indefinite"/gu)).toHaveLength(3);
  expect([...slide.matchAll(/<p:spTgt spid="(\d+)"><p:txEl><p:pRg st="(\d+)" end="(\d+)"\/>/gu)].map(match => match.slice(1))).toEqual([
    ["4", "0", "0"], ["4", "1", "1"], ["4", "2", "2"],
  ]);
  expect(slide.match(/<p:strVal val="visible"\/>/gu)).toHaveLength(3);
  expect(slide).toContain('<p:bldP spid="4" grpId="0" build="p"');
  const timingIDs = [...slide.matchAll(/<p:cTn id="(\d+)"/gu)].map(match => match[1]);
  expect(new Set(timingIDs).size).toBe(timingIDs.length);
  expect(slide).toContain('<p:cNvPr id="4"');
  expect(slide).toContain("<a:t>Конструкция &amp; сборка</a:t>");
  expect(slide).toContain("<a:t>•  Эскиз</a:t>");
  expect(slide).toContain("<a:t>•  Прототип</a:t>");
});

it("keeps omission backwards-compatible and can reveal bullets with no slide transition", async () => {
  const decoder = new TextDecoder();
  const legacy = await readPowerPointParts(powerpoint(appearanceSlides, "Учебный"));
  const slide = decoder.decode(legacy.get("ppt/slides/slide1.xml"));
  expect(slide).toContain('<a:srgbClr val="322A56"/>');
  expect(slide).toContain('<a:srgbClr val="E4DAFF"/>');
  expect(slide).toContain('<a:latin typeface="Aptos"/>');
  expect(slide).not.toMatch(/<p:(timing|transition)\b/u);

  const parts = await readPowerPointParts(powerpoint(appearanceSlides, "Учебный", [], {
    font: "geist", background: "original", animation: "on", transition: "none", colors: {},
  }));
  const animated = decoder.decode(parts.get("ppt/slides/slide1.xml"));
  expect(animated).toMatch(/<\/p:clrMapOvr><p:timing>/u);
  expect(animated).not.toContain("<p:transition");
  expect(animated.match(/nodeType="clickEffect"/gu)).toHaveLength(1);
});
