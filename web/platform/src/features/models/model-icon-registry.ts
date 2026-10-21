import { assetPaths } from "@/assets/asset-paths";

// Explicit public IDs only: unknown/new models keep supplied artwork or the
// theme fallback. Family/developer marks and provenance: public/assets/images/models/README.md.
export const modelIconRegistry: Readonly<Record<string, string>> = Object.freeze({
  // The site's branded chat; its legacy ID does not identify the ChatGPT product.
  chatgpt: assetPaths.images.models.brand,

  // OpenAI: text, image and speech models.
  gpt_image_2_5_flare: assetPaths.images.models.openai,
  gpt_image_2_5_sunburst: assetPaths.images.models.openai,
  gpt_image_2: assetPaths.images.models.openai,
  gpt_5: assetPaths.images.models.openai,
  gpt_5_1: assetPaths.images.models.openai,
  gpt_5_2: assetPaths.images.models.openai,
  gpt_5_3_codex: assetPaths.images.models.openai,
  gpt_5_4: assetPaths.images.models.openai,
  gpt_5_5: assetPaths.images.models.openai,
  gpt_5_6_terra: assetPaths.images.models.openai,
  gpt_5_chat_latest: assetPaths.images.models.openai,
  gpt_5_mini: assetPaths.images.models.openai,
  gpt_6_astra: assetPaths.images.models.openai,
  gpt_6_sol: assetPaths.images.models.openai,
  gpt_6_luna: assetPaths.images.models.openai,
  gpt_4o_mini_tts: assetPaths.images.models.openai,
  whisper_1: assetPaths.images.models.openai,

  // ByteDance: Seedream and Seedance.
  seedream_5_0_lite: assetPaths.images.models.bytedance,
  seedream_5_0_pro: assetPaths.images.models.bytedance,
  seedream_5_0_flash: assetPaths.images.models.bytedance,
  seedream_4_5: assetPaths.images.models.bytedance,
  seedance_2_0: assetPaths.images.models.bytedance,
  seedance_2_0_mini: assetPaths.images.models.bytedance,
  video_seedance_2_0_fast: assetPaths.images.models.bytedance,
  video_seedance_2_5: assetPaths.images.models.bytedance,

  midjourney_v7: assetPaths.images.models.midjourney,

  // Black Forest Labs.
  flux_2_pro: assetPaths.images.models.flux,
  flux_2_max: assetPaths.images.models.flux,
  flux_2_flex: assetPaths.images.models.flux,
  flux_3_video: assetPaths.images.models.flux,

  // Google developer mark for image, video and music; Gemini's own family mark below.
  nano_banana: assetPaths.images.models.google,
  nano_banana_2: assetPaths.images.models.google,
  nano_banana_pro: assetPaths.images.models.google,
  imagen_4_0: assetPaths.images.models.google,
  video_veo_3_1_fast: assetPaths.images.models.google,
  video_veo_3_1_quality: assetPaths.images.models.google,
  video_veo_3_1_lite: assetPaths.images.models.google,
  lyria_3_5: assetPaths.images.models.google,

  gemini_3_1_pro: assetPaths.images.models.gemini,
  gemini_3_7_flash: assetPaths.images.models.gemini,
  gemini_3_6_flash: assetPaths.images.models.gemini,
  gemini_3_5_flash: assetPaths.images.models.gemini,
  gemini_3_1_pro_preview: assetPaths.images.models.gemini,
  gemini_3_pro_preview: assetPaths.images.models.gemini,
  gemini_3_pro_preview_thinking: assetPaths.images.models.gemini,
  gemini_3_flash_preview: assetPaths.images.models.gemini,
  gemini_2_5_pro: assetPaths.images.models.gemini,
  gemini_2_5_flash: assetPaths.images.models.gemini,
  gemini_2_5_flash_lite: assetPaths.images.models.gemini,
  gemini_omni_flash_preview: assetPaths.images.models.gemini,
  video_gemini_omni_1_1_flash_ext: assetPaths.images.models.gemini,

  // Anthropic.
  claude_opus_4_7: assetPaths.images.models.claude,
  claude_opus_4_8: assetPaths.images.models.claude,
  claude_opus_5: assetPaths.images.models.claude,
  claude_fable_5_1: assetPaths.images.models.claude,
  claude_fable_5: assetPaths.images.models.claude,
  claude_opus_4_6: assetPaths.images.models.claude,
  claude_sonnet_4_6: assetPaths.images.models.claude,
  claude_opus_4_5_20251101: assetPaths.images.models.claude,
  claude_opus_5_5: assetPaths.images.models.claude,
  claude_haiku_4_5_20251001: assetPaths.images.models.claude,

  qwen_image_3: assetPaths.images.models.qwen,
  qwen_image_3_pro: assetPaths.images.models.qwen,
  qwen_3_8_max: assetPaths.images.models.qwen,
  qwen_3_7_flash: assetPaths.images.models.qwen,

  grok_image_1_5: assetPaths.images.models.grok,
  grok_image_2_0: assetPaths.images.models.grok,
  grok_imagine_1_5_video: assetPaths.images.models.grok,
  grok_4_5: assetPaths.images.models.grok,
  grok_4_6: assetPaths.images.models.grok,
  grok_4_7: assetPaths.images.models.grok,

  deepseek_v4_pro: assetPaths.images.models.deepseek,
  deepseek_v4_flash: assetPaths.images.models.deepseek,
  deepseek_v3_2: assetPaths.images.models.deepseek,
  deepseek_v3_2_exp: assetPaths.images.models.deepseek,
  deepseek_r1_250528: assetPaths.images.models.deepseek,
  deepseek_v3_0324: assetPaths.images.models.deepseek,

  kimi_k3: assetPaths.images.models.kimi,

  video_kling_v3: assetPaths.images.models.kling,
  video_kling_3_0_turbo: assetPaths.images.models.kling,
  video_kling_o3_standard: assetPaths.images.models.kling,
  kling_2_6: assetPaths.images.models.kling,
  kling_video_o1: assetPaths.images.models.kling,

  video_minimax_h3: assetPaths.images.models.minimax,
  minimax_h3_max: assetPaths.images.models.minimax,
  video_runway_gen4_5: assetPaths.images.models.runway,
  happyhorse_1_0: assetPaths.images.models.happyhorse,
  happyhorse_1_1: assetPaths.images.models.happyhorse,

  // Skywork developer mark for SkyReels.
  skyreels_v4_fast: assetPaths.images.models.skywork,
  skyreels_v4_std: assetPaths.images.models.skywork,

  suno_v6: assetPaths.images.models.suno,
  suno_v6_wild: assetPaths.images.models.suno,
  suno_v6_mini: assetPaths.images.models.suno,

  // Alibaba Cloud developer mark for Wan and Tongyi's Z-Image.
  wan_3_0: assetPaths.images.models.alibaba,
  wan_3_0_prime: assetPaths.images.models.alibaba,
  wan_2_7: assetPaths.images.models.alibaba,
  z_image_turbo: assetPaths.images.models.alibaba,

  pixverse_v6: assetPaths.images.models.pixverse,
  vidu_q3_pro: assetPaths.images.models.vidu,
  vidu_q3: assetPaths.images.models.vidu,
  vidu_q3_mix: assetPaths.images.models.vidu,
  vidu_q3_turbo: assetPaths.images.models.vidu,
});

export function getModelIconPath(modelID: string): string | undefined {
  return Object.hasOwn(modelIconRegistry, modelID) ? modelIconRegistry[modelID] : undefined;
}
