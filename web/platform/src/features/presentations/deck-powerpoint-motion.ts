import type { DeckAppearance } from "./deck-appearance";

export function powerpointTransition(transition: DeckAppearance["transition"] | undefined) {
  if (transition === "fade") return '<p:transition spd="fast"><p:fade/></p:transition>';
  if (transition === "push") return '<p:transition spd="fast"><p:push dir="l"/></p:transition>';
  return "";
}

function revealParagraph(shapeId: number, paragraph: number) {
  // The root and main sequence use IDs 1 and 2. Each click owns four time nodes.
  const id = 3 + paragraph * 4;
  const startImmediately = '<p:stCondLst><p:cond delay="0"/></p:stCondLst>';
  return [
    `<p:par><p:cTn id="${id}" fill="hold"><p:stCondLst><p:cond delay="indefinite"/></p:stCondLst><p:childTnLst>`,
    `<p:par><p:cTn id="${id + 1}" fill="hold">${startImmediately}<p:childTnLst>`,
    `<p:par><p:cTn id="${id + 2}" presetID="1" presetClass="entr" presetSubtype="0" fill="hold" grpId="0" nodeType="clickEffect">`,
    `${startImmediately}<p:childTnLst><p:set><p:cBhvr>`,
    `<p:cTn id="${id + 3}" dur="1" fill="hold">${startImmediately}</p:cTn>`,
    `<p:tgtEl><p:spTgt spid="${shapeId}"><p:txEl><p:pRg st="${paragraph}" end="${paragraph}"/></p:txEl></p:spTgt></p:tgtEl>`,
    '<p:attrNameLst><p:attrName>style.visibility</p:attrName></p:attrNameLst>',
    '</p:cBhvr><p:to><p:strVal val="visible"/></p:to></p:set>',
    '</p:childTnLst></p:cTn></p:par>',
    '</p:childTnLst></p:cTn></p:par>',
    '</p:childTnLst></p:cTn></p:par>',
  ].join("");
}

// The PowerPoint Appear effect targets a paragraph range in one editable text box.
export function powerpointParagraphTiming(shapeId: number, count: number) {
  if (count === 0) return "";
  const steps = Array.from({ length: count }, (_, paragraph) => revealParagraph(shapeId, paragraph));
  return [
    '<p:timing><p:tnLst><p:par><p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot"><p:childTnLst>',
    '<p:seq concurrent="1" nextAc="seek"><p:cTn id="2" dur="indefinite" nodeType="mainSeq"><p:childTnLst>',
    ...steps,
    '</p:childTnLst></p:cTn>',
    '<p:prevCondLst><p:cond evt="onPrev" delay="0"><p:tgtEl><p:sldTgt/></p:tgtEl></p:cond></p:prevCondLst>',
    '<p:nextCondLst><p:cond evt="onNext" delay="0"><p:tgtEl><p:sldTgt/></p:tgtEl></p:cond></p:nextCondLst>',
    '</p:seq></p:childTnLst></p:cTn></p:par></p:tnLst>',
    `<p:bldLst><p:bldP spid="${shapeId}" grpId="0" build="p"/></p:bldLst></p:timing>`,
  ].join("");
}
