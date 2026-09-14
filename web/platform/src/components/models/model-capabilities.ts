export type InputCapability = {
  support: "supported" | "unsupported" | "unknown";
  extensions: string[] | null;
  max_count: number | null;
};
export type CapabilityProfile = {
  text?: { images: InputCapability; videos: InputCapability; files: InputCapability };
  image?: { images: InputCapability; aspect_ratios: string[] | null; resolutions: string[] | null; quality_modes: string[] | null; speed_modes: string[] | null; max_output_count: number | null; max_combined_images: number | null };
  video?: { images: InputCapability; videos: InputCapability; allowed_image_counts: number[] | null; duration: { mode: string; min_seconds: number | null; max_seconds: number | null; allowed_seconds: number[] | null; by_resolution?: Record<string, number[]>; max_by_orientation?: Record<string, number> }; resolutions: string[] | null; quality_modes: string[] | null; aspect_ratios: string[] | null; audio: { mode: string; selectable: boolean }; start_frame: string; end_frame: string };
  audio?: { audio: InputCapability; videos: InputCapability; output: string };
  notes?: string[];
};
export type ModelCapabilities = { schema_version: number; api: CapabilityProfile; application: CapabilityProfile };
