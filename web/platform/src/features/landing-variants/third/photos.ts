import { assetPaths } from '@/assets/asset-paths';

/** Reuse the contributor's actual photo files; no copies or changes to either source. */
export type HomePhoto = Exclude<keyof typeof assetPaths.images.home, 'ui'>;
export function homePhotoSource(photo: HomePhoto): string {
  // The contributor's visible gallery uses this frame instead of the rejected portrait.
  return photo === 'originalPortrait' ? assetPaths.images.home.suiteVideo : assetPaths.images.home[photo];
}

export const suitePhotos = ['suiteImage', 'suiteVideo', 'suiteAudio', 'suiteText', 'suiteInspiration'] as const satisfies readonly HomePhoto[];
export const platformPhotos = ['platformWorkspace', 'platformMcp', 'platformAgents', 'platformPlugins', 'platformInspiration'] as const satisfies readonly HomePhoto[];

/** Actual dimensions of the existing source files, including square/portrait frames. */
export const suitePhotoSizes = {
  suiteImage: { width: 1920, height: 1072 },
  suiteVideo: { width: 1920, height: 1072 },
  suiteAudio: { width: 1920, height: 1920 },
  suiteText: { width: 1856, height: 2304 },
  suiteInspiration: { width: 1920, height: 1920 },
  platformWorkspace: { width: 1920, height: 1072 },
  platformMcp: { width: 1920, height: 1072 },
  platformAgents: { width: 1920, height: 1072 },
  platformPlugins: { width: 1920, height: 1072 },
  platformInspiration: { width: 1920, height: 1072 },
} as const satisfies Record<(typeof suitePhotos | typeof platformPhotos)[number], { width: number; height: number }>;
export const modelPhotos = {
  gpt_image_2: 'modelGptImage', nano_banana_pro: 'modelNanoBanana', video_kling_v3: 'modelKling',
  video_veo_3_1_fast: 'modelVeo', seedream_5_0_lite: 'modelSeedream', flux_2_pro: 'modelFlux',
  suno_v6: 'modelSuno', gpt_4o_mini_tts: 'modelGptVoice',
} as const satisfies Record<string, HomePhoto>;
export const possibilityPhotos = ['possibilityAdvertising', 'possibilityProduct', 'possibilityBrand', 'possibilityFilm'] as const satisfies readonly HomePhoto[];
export const originalPhotos = ['originalWorld', 'originalPortrait', 'originalMotion'] as const satisfies readonly HomePhoto[];
