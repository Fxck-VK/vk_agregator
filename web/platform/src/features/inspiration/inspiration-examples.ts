import { getTranslator, type Translator } from "@/i18n/messages";

import { assetPaths } from "@/assets/asset-paths";
import { getDictionary } from "@/i18n/dictionary";
import type { Locale } from "@/i18n/locales";

export type InspirationExample = {
  id: string;
  modelId: string;
  modelName: string;
  quality: string;
  mediaType: "image" | "video";
  mediaPath: string;
  posterPath?: string;
  mediaAlt: string;
  mediaWidth: number;
  mediaHeight: number;
  title: string;
  prompt: string;
  downloadName: string;
  openLabel: string;
  canRecreate: boolean;
};

type UploadedExample = Pick<
  InspirationExample,
  "id" | "mediaType" | "mediaPath" | "posterPath" | "mediaAlt" | "mediaWidth" | "mediaHeight" | "title"
>;

function uploadedExample(example: UploadedExample, msg: Translator = getTranslator("ru")): InspirationExample {
  return {
    ...example,
    modelId: example.mediaType === "video" ? "video-example" : "image-example",
    modelName: "NeiroHub",
    quality: example.mediaType === "video" ? msg("inspirationExamples.video") : msg("inspirationExamples.image"),
    prompt: msg("inspirationExamples.noPromptIsProvidedForThisExample"),
    downloadName: `neirohub-${example.id}.${example.mediaType === "video" ? "mp4" : "png"}`,
    openLabel: msg("inspirationExamples.openTheValueExample", { value1: example.title }),
    canRecreate: false,
  };
}

export function getInspirationExamples(locale: Locale = "ru"): readonly InspirationExample[] {
  const msg = getTranslator(locale);
  const t = getDictionary(locale);
  return [
    {
      id: "paper-crane-cloud",
      modelId: "gpt_image_2",
      modelName: t.inspiration.modelName,
      quality: "1K",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.paperCraneCloud,
      mediaAlt: t.inspiration.exampleAlt,
      mediaWidth: 1024,
      mediaHeight: 1536,
      title: t.inspiration.exampleTitle,
      prompt: t.inspiration.prompt,
      downloadName: "neirohub-paper-crane-cloud.png",
      openLabel: t.inspiration.openExample,
      canRecreate: true,
    },
    uploadedExample({
      id: "sleeping-on-cloud",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.sleepingOnCloud,
      mediaAlt: msg("inspirationExamples.aWomanRestsOnAWhiteCloud"),
      mediaWidth: 956,
      mediaHeight: 1280,
      title: msg("inspirationExamples.sleepingOnACloud"),
    }, msg),
    uploadedExample({
      id: "desert-airplane-traveler",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.desertAirplaneTraveler,
      mediaAlt: msg("inspirationExamples.aTravelerWithASuitcaseBesideA"),
      mediaWidth: 1024,
      mediaHeight: 1280,
      title: msg("inspirationExamples.desertTraveler"),
    }, msg),
    uploadedExample({
      id: "mango-drink-yellow-fashion",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.mangoDrinkYellowFashion,
      mediaAlt: msg("inspirationExamples.yellowShoesAndFruitDrinkCansOn"),
      mediaWidth: 955,
      mediaHeight: 1280,
      title: msg("inspirationExamples.yellowAdvertisingShoot"),
    }, msg),
    uploadedExample({
      id: "turtle-selfie",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.turtleSelfie,
      mediaAlt: msg("inspirationExamples.aWomanTakesASelfieWithFour"),
      mediaWidth: 956,
      mediaHeight: 1280,
      title: msg("inspirationExamples.aSelfieWithTurtles"),
    }, msg),
    uploadedExample({
      id: "fixies-selfie",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.fixiesSelfie,
      mediaAlt: msg("inspirationExamples.aGirlTakesASelfieWithTwo"),
      mediaWidth: 1031,
      mediaHeight: 1280,
      title: msg("inspirationExamples.aSelfieInTheWorkshop"),
    }, msg),
    uploadedExample({
      id: "spider-man-selfie",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.spiderManSelfie,
      mediaAlt: msg("inspirationExamples.aManAndSpiderManTakeA"),
      mediaWidth: 955,
      mediaHeight: 1280,
      title: msg("inspirationExamples.aSelfieAboveTheCity"),
    }, msg),
    uploadedExample({
      id: "hair-fresh-shampoo",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.hairFreshShampoo,
      mediaAlt: msg("inspirationExamples.aWomanInHairRollersTalksOn"),
      mediaWidth: 1744,
      mediaHeight: 2336,
      title: msg("inspirationExamples.shampooAdvertisement"),
    }, msg),
    uploadedExample({
      id: "birthday-photo-strip",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.birthdayPhotoStrip,
      mediaAlt: msg("inspirationExamples.threeFamilyPhotosOfAFatherAnd"),
      mediaWidth: 714,
      mediaHeight: 1280,
      title: msg("inspirationExamples.birthday"),
    }, msg),
    uploadedExample({
      id: "desert-storm-traveler",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.desertStormTraveler,
      mediaAlt: msg("inspirationExamples.aTravelerWithASuitcaseAndAirplane"),
      mediaWidth: 956,
      mediaHeight: 1280,
      title: msg("inspirationExamples.beforeTheSandstorm"),
    }, msg),
    uploadedExample({
      id: "solene-fridge",
      mediaType: "image",
      mediaPath: assetPaths.images.inspiration.soleneFridge,
      mediaAlt: msg("inspirationExamples.soleneCosmeticsJarsArrangedOnRefrigeratorShelves"),
      mediaWidth: 958,
      mediaHeight: 1280,
      title: msg("inspirationExamples.cosmeticsInTheRefrigerator"),
    }, msg),
    uploadedExample({
      id: "video-1",
      mediaType: "video",
      mediaPath: assetPaths.videos.inspiration.videoOne,
      posterPath: assetPaths.images.inspiration.videoOnePoster,
      mediaAlt: msg("inspirationExamples.aVideoExampleFromTheInspirationCollection"),
      mediaWidth: 2560,
      mediaHeight: 1440,
      title: msg("inspirationExamples.videoExample1"),
    }, msg),
    uploadedExample({
      id: "video-2",
      mediaType: "video",
      mediaPath: assetPaths.videos.inspiration.videoTwo,
      posterPath: assetPaths.images.inspiration.videoTwoPoster,
      mediaAlt: msg("inspirationExamples.aVerticalVideoExampleFromTheInspiration"),
      mediaWidth: 1244,
      mediaHeight: 1664,
      title: msg("inspirationExamples.videoExample2"),
    }, msg),
  ];
}

export const inspirationExamples = getInspirationExamples();

export function selectInspirationExamples(
  modelId: string | null | undefined,
  limit = 3,
  mediaType?: InspirationExample["mediaType"],
  options: { fillFromCollection?: boolean } = {},
  locale: Locale = "ru",
): readonly InspirationExample[] {
  const inspirationExamples = getInspirationExamples(locale);
  const eligibleExamples = mediaType
    ? inspirationExamples.filter((example) => example.mediaType === mediaType)
    : inspirationExamples;
  const modelExamples = modelId
    ? eligibleExamples.filter((example) => example.modelId === modelId)
    : [];
  const source = options.fillFromCollection
    ? [...modelExamples, ...eligibleExamples.filter((example) => !modelExamples.includes(example))]
    : modelExamples.length > 0 ? modelExamples : eligibleExamples;

  return source.slice(0, limit);
}
