import { assetPaths } from "@/assets/asset-paths";
import { ru } from "@/i18n/ru";

export type InspirationExample = {
  id: string;
  modelId: string;
  modelName: string;
  quality: string;
  mediaType: "image" | "video";
  mediaPath: string;
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
  "id" | "mediaType" | "mediaPath" | "mediaAlt" | "mediaWidth" | "mediaHeight" | "title"
>;

function uploadedExample(example: UploadedExample): InspirationExample {
  return {
    ...example,
    modelId: example.mediaType === "video" ? "video-example" : "image-example",
    modelName: "NeiroHub",
    quality: example.mediaType === "video" ? "Видео" : "Изображение",
    prompt: "Промпт для этого примера не указан.",
    downloadName: `neirohub-${example.id}.${example.mediaType === "video" ? "mp4" : "png"}`,
    openLabel: `Открыть пример «${example.title}»`,
    canRecreate: false,
  };
}

export const inspirationExamples: readonly InspirationExample[] = [
  {
    id: "paper-crane-cloud",
    modelId: "gpt-image-2",
    modelName: ru.inspiration.modelName,
    quality: "1K",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.paperCraneCloud,
    mediaAlt: ru.inspiration.exampleAlt,
    mediaWidth: 1024,
    mediaHeight: 1536,
    title: ru.inspiration.exampleTitle,
    prompt: ru.inspiration.prompt,
    downloadName: "neirohub-paper-crane-cloud.png",
    openLabel: ru.inspiration.openExample,
    canRecreate: true,
  },
  uploadedExample({
    id: "sleeping-on-cloud",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.sleepingOnCloud,
    mediaAlt: "Девушка отдыхает на белом облаке в комнате с тёплыми охристыми стенами",
    mediaWidth: 956,
    mediaHeight: 1280,
    title: "Сон на облаке",
  }),
  uploadedExample({
    id: "desert-airplane-traveler",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.desertAirplaneTraveler,
    mediaAlt: "Путешественник с чемоданом рядом с винтажным самолётом в пустыне",
    mediaWidth: 1024,
    mediaHeight: 1280,
    title: "Путешественник в пустыне",
  }),
  uploadedExample({
    id: "mango-drink-yellow-fashion",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.mangoDrinkYellowFashion,
    mediaAlt: "Жёлтые туфли и банки фруктового напитка на асфальте",
    mediaWidth: 955,
    mediaHeight: 1280,
    title: "Жёлтая рекламная съёмка",
  }),
  uploadedExample({
    id: "turtle-selfie",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.turtleSelfie,
    mediaAlt: "Девушка делает селфи с четырьмя черепашками-ниндзя в ночном переулке",
    mediaWidth: 956,
    mediaHeight: 1280,
    title: "Селфи с черепашками",
  }),
  uploadedExample({
    id: "fixies-selfie",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.fixiesSelfie,
    mediaAlt: "Девочка делает селфи с двумя анимационными персонажами в мастерской",
    mediaWidth: 1031,
    mediaHeight: 1280,
    title: "Селфи в мастерской",
  }),
  uploadedExample({
    id: "spider-man-selfie",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.spiderManSelfie,
    mediaAlt: "Мужчина и Человек-паук делают селфи на фоне города",
    mediaWidth: 955,
    mediaHeight: 1280,
    title: "Селфи над городом",
  }),
  uploadedExample({
    id: "hair-fresh-shampoo",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.hairFreshShampoo,
    mediaAlt: "Женщина в бигуди говорит по телефону и показывает флакон шампуня",
    mediaWidth: 1744,
    mediaHeight: 2336,
    title: "Реклама шампуня",
  }),
  uploadedExample({
    id: "birthday-photo-strip",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.birthdayPhotoStrip,
    mediaAlt: "Три семейных кадра отца и сына с праздничным тортом",
    mediaWidth: 714,
    mediaHeight: 1280,
    title: "День рождения",
  }),
  uploadedExample({
    id: "desert-storm-traveler",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.desertStormTraveler,
    mediaAlt: "Путешественник с чемоданом и самолётом перед огромной песчаной бурей",
    mediaWidth: 956,
    mediaHeight: 1280,
    title: "Перед песчаной бурей",
  }),
  uploadedExample({
    id: "solene-fridge",
    mediaType: "image",
    mediaPath: assetPaths.images.inspiration.soleneFridge,
    mediaAlt: "Баночки косметики Solene расставлены на полках холодильника",
    mediaWidth: 958,
    mediaHeight: 1280,
    title: "Косметика в холодильнике",
  }),
  uploadedExample({
    id: "video-1",
    mediaType: "video",
    mediaPath: assetPaths.videos.inspiration.videoOne,
    mediaAlt: "Видеопример из подборки вдохновения",
    mediaWidth: 2560,
    mediaHeight: 1440,
    title: "Видеопример 1",
  }),
  uploadedExample({
    id: "video-2",
    mediaType: "video",
    mediaPath: assetPaths.videos.inspiration.videoTwo,
    mediaAlt: "Вертикальный видеопример из подборки вдохновения",
    mediaWidth: 1244,
    mediaHeight: 1664,
    title: "Видеопример 2",
  }),
];

export function selectInspirationExamples(
  modelId: string | null | undefined,
  limit = 3,
  mediaType?: InspirationExample["mediaType"],
  options: { fillFromCollection?: boolean } = {},
): readonly InspirationExample[] {
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
