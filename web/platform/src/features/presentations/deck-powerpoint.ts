import type { DeckSlide } from "./deck-content";
import { presentationDeckCopy, presentationStyles } from "@/i18n/presentation-deck";

const doc = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
const drawing = "http://schemas.openxmlformats.org/drawingml/2006/main";
const presentation = "http://schemas.openxmlformats.org/presentationml/2006/main";
const namespaces = `xmlns:a="${drawing}" xmlns:r="${doc}" xmlns:p="${presentation}"`;
const group = '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>';
const colors = 'accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" bg1="lt1" bg2="lt2" tx1="dk1" tx2="dk2" hlink="hlink" folHlink="folHlink"';
const xml = (text: string) => text.replace(/[^\u0009\u000a\u000d\u0020-\uD7FF\uE000-\uFFFD\u{10000}-\u{10FFFF}]/gu, "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");

function relationships(entries: [string, string][]) {
  return `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">${entries.map(([kind, target], i) => `<Relationship Id="rId${i + 1}" Type="${doc}/${kind}" Target="${target}"/>`).join("")}</Relationships>`;
}
function textShape(id: number, x: number, y: number, width: number, height: number, lines: string[], size: number, color: string, bold = false) {
  const paragraphs = lines.map(line => `<a:p><a:r><a:rPr lang="ru-RU" sz="${size}" b="${bold ? 1 : 0}"><a:solidFill><a:srgbClr val="${color}"/></a:solidFill><a:latin typeface="Aptos"/></a:rPr><a:t>${xml(line)}</a:t></a:r></a:p>`).join("");
  return `<p:sp><p:nvSpPr><p:cNvPr id="${id}" name="${presentationDeckCopy.powerpoint.textShapeName(id)}"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="${x}" y="${y}"/><a:ext cx="${width}" cy="${height}"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr wrap="square"><a:normAutofit/></a:bodyPr><a:lstStyle/>${paragraphs}</p:txBody></p:sp>`;
}

// Store the small UTF-8 XML parts as uncompressed ZIP records (ECMA-376 / OPC).
function packageParts(parts: Map<string, string | Uint8Array<ArrayBuffer>>) {
  const encoder = new TextEncoder();
  const files: Uint8Array<ArrayBuffer>[] = [], directory: Uint8Array<ArrayBuffer>[] = [];
  let offset = 0, centralSize = 0;
  for (const [path, content] of parts) {
    const name = encoder.encode(path);
    const bytes = typeof content === "string" ? encoder.encode(content) : content;
    let crc = -1;
    for (const byte of bytes) {
      crc ^= byte;
      for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0);
    }
    const local = new Uint8Array(30 + name.length);
    const header = new DataView(local.buffer);
    header.setUint32(0, 0x04034b50, true);
    header.setUint16(4, 20, true);
    header.setUint16(12, 33, true);
    header.setUint32(14, ~crc, true);
    header.setUint32(18, bytes.length, true);
    header.setUint32(22, bytes.length, true);
    header.setUint16(26, name.length, true);
    local.set(name, 30);
    files.push(local, bytes);
    const central = new Uint8Array(46 + name.length);
    const index = new DataView(central.buffer);
    index.setUint32(0, 0x02014b50, true);
    index.setUint16(4, 20, true);
    central.set(local.subarray(4, 28), 6);
    index.setUint32(42, offset, true);
    central.set(name, 46);
    directory.push(central);
    centralSize += central.length;
    offset += local.length + bytes.length;
  }
  const end = new Uint8Array(22), footer = new DataView(end.buffer);
  footer.setUint32(0, 0x06054b50, true);
  footer.setUint16(8, parts.size, true);
  footer.setUint16(10, parts.size, true);
  footer.setUint32(12, centralSize, true);
  footer.setUint32(16, offset, true);
  return new Blob([...files, ...directory, end], { type: "application/vnd.openxmlformats-officedocument.presentationml.presentation" });
}

export function powerpoint(
  slides: DeckSlide[],
  style: string,
  photos: ReadonlyArray<Uint8Array<ArrayBuffer> | undefined> = [],
) {
  const parts = new Map<string, string | Uint8Array<ArrayBuffer>>();
  const entries: [string, string][] = [["slideMaster", "slideMasters/slideMaster1.xml"]];
  const types = new Map<string, string>();
  function part(path: string, kind: string, content: string) {
    parts.set(path, content);
    types.set(path, `application/vnd.openxmlformats-officedocument.${kind}+xml`);
  }
  parts.set("_rels/.rels", relationships([["officeDocument", "ppt/presentation.xml"]]));
  let ids = "";
  slides.forEach((slide, i) => {
    const n = i + 1;
    entries.push(["slide", `slides/slide${n}.xml`]);
    ids += `<p:sldId id="${255 + n}" r:id="rId${n + 1}"/>`;
    const cover = i === 0;
    const bg = cover ? (style === presentationStyles.business ? "173C3A" : "322A56") : "F8F7F2";
    const ink = cover ? "FFFFFF" : "201E29";
    const photo = photos[i];
    const width = photo ? 5350000 : 10800000;
    const folio = textShape(2, 650000, 360000, width, 480000, [presentationDeckCopy.powerpoint.folio(style, n, slides.length)], 1600, cover ? "E4DAFF" : "575463");
    const title = textShape(3, 650000, 1250000, width, 1800000, [slide.title], slide.title.length > 70 ? 3200 : 4200, ink, true);
    const body = textShape(4, 650000, 3180000, width, 3120000, slide.points.map(p => slide.points.length > 1 ? `•  ${p}` : p), 2300, ink);
    const rels: [string, string][] = [["slideLayout", "../slideLayouts/slideLayout1.xml"]];
    let picture = "";
    if (photo) {
      const filename = cover ? "cover.jpg" : `slide${n}.jpg`;
      const name = cover ? presentationDeckCopy.powerpoint.coverShapeName : presentationDeckCopy.powerpoint.slideImageShapeName;
      parts.set(`ppt/media/${filename}`, photo);
      rels.push(["image", `../media/${filename}`]);
      picture = `<p:pic><p:nvPicPr><p:cNvPr id="5" name="${name}"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="6461760" y="0"/><a:ext cx="5730240" cy="6858000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`;
    }
    part(`ppt/slides/slide${n}.xml`, "presentationml.slide", `<p:sld ${namespaces}><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="${bg}"/></a:solidFill><a:effectLst/></p:bgPr></p:bg><p:spTree>${group}${picture}${folio}${title}${body}</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`);
    parts.set(`ppt/slides/_rels/slide${n}.xml.rels`, relationships(rels));
  });
  part("ppt/presentation.xml", "presentationml.presentation.main", `<p:presentation ${namespaces}><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst>${ids}</p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`);
  parts.set("ppt/_rels/presentation.xml.rels", relationships(entries));
  part("ppt/slideMasters/slideMaster1.xml", "presentationml.slideMaster", `<p:sldMaster ${namespaces}><p:cSld><p:spTree>${group}</p:spTree></p:cSld><p:clrMap ${colors}/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`);
  parts.set("ppt/slideMasters/_rels/slideMaster1.xml.rels", relationships([["slideLayout", "../slideLayouts/slideLayout1.xml"], ["theme", "../theme/theme1.xml"]]));
  part("ppt/slideLayouts/slideLayout1.xml", "presentationml.slideLayout", `<p:sldLayout ${namespaces} type="blank" preserve="1"><p:cSld name="${presentationDeckCopy.powerpoint.emptyLayoutName}"><p:spTree>${group}</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`);
  parts.set("ppt/slideLayouts/_rels/slideLayout1.xml.rels", relationships([["slideMaster", "../slideMasters/slideMaster1.xml"]]));
  const palette = { dk1: "201E29", lt1: "FFFFFF", dk2: "322A56", lt2: "F8F7F2", accent1: "AD97EA", accent2: "91CFB8", accent3: "DFAE83", accent4: "728FC8", accent5: "BD8CAB", accent6: "F4DF91", hlink: "435FA8", folHlink: "79599C" };
  const scheme = Object.entries(palette).map(([k, v]) => `<a:${k}><a:srgbClr val="${v}"/></a:${k}>`).join("");
  const font = '<a:latin typeface="Aptos"/><a:ea typeface=""/><a:cs typeface=""/>';
  const fill = '<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>';
  const lines = [6350, 12700, 19050].map(n => `<a:ln w="${n}">${fill}</a:ln>`).join("");
  part("ppt/theme/theme1.xml", "theme", `<a:theme xmlns:a="${drawing}" name="Epifera"><a:themeElements><a:clrScheme name="Epifera">${scheme}</a:clrScheme><a:fontScheme name="Aptos"><a:majorFont>${font}</a:majorFont><a:minorFont>${font}</a:minorFont></a:fontScheme><a:fmtScheme name="Epifera"><a:fillStyleLst>${fill.repeat(3)}</a:fillStyleLst><a:lnStyleLst>${lines}</a:lnStyleLst><a:effectStyleLst>${"<a:effectStyle><a:effectLst/></a:effectStyle>".repeat(3)}</a:effectStyleLst><a:bgFillStyleLst>${fill.repeat(3)}</a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`);
  const overrides = [...types].map(([path, type]) => `<Override PartName="/${path}" ContentType="${type}"/>`).join("");
  parts.set("[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="jpg" ContentType="image/jpeg"/>${overrides}</Types>`);
  return packageParts(parts);
}
