export const marketingAssets = "/assets/images/marketing/";
export const marketingPartners = ["Google", "OpenAI", "FLUX", "KLING", "Suno", "elevenlabs"] as const;
export const supportEmail = "support@epifera.com";

export const showcase = [
  { icon: "◈", image: "product-shots.webp", href: "/app/image" },
  { icon: "▶", image: "preview-1.webp", href: "/app/models" },
  { icon: "♫", image: "preview-4.webp", href: "/app/music" },
  { icon: "✦", image: "card-yourentire-2x.webp", href: "/app" },
  { icon: "◇", image: "brand-campaign.webp", href: "/app/inspiration" },
] as const;

export const toolCards = [
  { category: 1, image: "gpt-2.webp", href: "/app/image" },
  { category: 2, image: "preview-1.webp", href: "/app/models" },
  { category: 3, image: "preview-4.webp", href: "/app/music" },
  { category: 3, image: "advertising.webp", href: "/app/speech" },
  { category: 4, image: "card-yourentire-2x.webp", href: "/app" },
  { category: 1, image: "product-shots.webp", href: "/app/image" },
] as const;

export const modelCards = [
  { name: "GPT Image", image: "gpt-2.webp" },
  { name: "Nano Banana", image: "product-shots.webp" },
  { name: "Kling", image: "preview-2.webp" },
  { name: "Veo", image: "preview-1.webp" },
  { name: "FLUX", image: "brand-campaign.webp" },
  { name: "Suno", image: "preview-4.webp" },
  { name: "ElevenLabs", image: "advertising.webp" },
] as const;

export const workflowImages = ["card-alltools-3-2x.webp", "card-yourentire-2x.webp", "card-oneplace-2x.webp", "card-workflow-2x.webp"] as const;
export const useCaseImages = ["advertising.webp", "product-shots.webp", "brand-campaign.webp", "preview-2.webp"] as const;
export const originalImages = ["preview-1.webp", "preview-2.webp", "preview-4.webp"] as const;
export const footerDestinations = ["/app/image", "/app/models", "/app/music", "/app/speech", "/app", "/app/models", "/app/inspiration", "/app/files", "/login", "mailto:support@epifera.com", "mailto:support@epifera.com"] as const;
