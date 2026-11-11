import { modelWorkspaceHref } from './ModelCard/model-card-content';

// Public landing families select real catalogue IDs without changing admission.
const models = new Set(['gpt_image_2', 'nano_banana_pro', 'video_kling_v3',
  'video_veo_3_1_fast', 'seedream_5_0_lite', 'flux_2_pro', 'suno_v6', 'gpt_4o_mini_tts']);

export function modelEntryTarget(id: string): string | null {
  if (!models.has(id)) return null;
  if (id === 'gpt_4o_mini_tts') return `/app/speech?model=${id}`;
  return modelWorkspaceHref({ id, category: id === 'suno_v6' ? 'audio' : undefined });
}

export function modelEntryHref(id: string): string {
  return `/app/launch?model=${encodeURIComponent(id)}`;
}
