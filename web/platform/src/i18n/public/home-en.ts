import { brandName } from "@/config/brand";

// English is the first rebuilt homepage. A Russian translation is deliberately
// not synthesized; the /ru home remains empty until its own editorial pass.
export const homeEn = {
  metadata: { title: `${brandName} — The creative platform for your best work`, description: "Create images, video, music and more. Explore AI models and bring your next idea to life in one creative workspace." },
  announcement: "Where ideas take shape", skip: "Skip to content", start: "Start creating", explore: "Explore", why: `Why ${brandName}?`,
  controls: { backToTop: "Back to top", openMenu: "Open navigation", closeMenu: "Close navigation", navigation: "Main navigation", search: "Search or create", searchPlaceholder: "Search tools and AI models", close: "Close", previous: "Previous", next: "Next", showSlide: "Go to slide", pause: "Pause motion", play: "Resume motion", loading: "Loading models…", noResults: "No results. Try another name.", modelError: "Models are temporarily unavailable. Explore the tools below.", toolResults: "Tools", modelResults: "AI models", searchHint: "Search by name, in English or Russian.", comingSoon: "Coming soon" },
  navigation: [
    { label: "Creative Suite", groups: [
      { title: "Create", links: [{ label: "Image Generator", href: "/app/image" }, { label: "Video Generator", href: "/app/models" }, { label: "Music Generator", href: "/app/music" }, { label: "Text to Speech", href: "/app/speech" }] },
      { title: "Explore", links: [{ label: "All AI models", href: "/app/models" }, { label: "AI chat", href: "/app" }, { label: "Inspiration", href: "/app/inspiration" }] },
      { title: "Your workspace", links: [{ label: "My files", href: "/app/files" }, { label: "Conversations", href: "/app/chats" }, { label: "Start creating", href: "/app" }] },
    ] },
    { label: "Inspiration", groups: [{ title: "Find your next idea", links: [{ label: "Visual inspiration", href: "/app/inspiration" }, { label: "Image examples", href: "/ai/image-generator#examples" }, { label: "Creative possibilities", href: "#possibilities" }] }] },
    { label: "Resources", groups: [{ title: "Learn and explore", links: [{ label: "How it works", href: "#workflow" }, { label: "Questions and answers", href: "#questions" }, { label: "Contact support", href: "mailto:support@epifera.com" }] }] },
    { label: "For teams", href: "#teams" }, { label: "Plans", href: "#plans" },
  ],
  login: "Log in", signup: "Sign up",
  hero: { eyebrow: "Your creative tools, all together", title: "The creative platform to direct your best work", description: "AI models for video, image, and audio. Explore ideas, shape your visual world, and move from your first prompt to a result worth sharing.", modelCaption: "Find your creative flow with leading AI model families", capabilities: ["Generate images", "Bring ideas to life", "Create your own music", "Tell your story", "Explore new worlds", "Find the right words"], partners: ["Google", "OpenAI", "FLUX", "KLING", "Suno", "elevenlabs"] },
  suite: { title: "One place to create anything", description: "Pick your starting point. Every tool, every model, every format.", tabsLabel: "Explore the platform", tabs: ["Creative Suite", "Workspace", "MCP & API", "Agents", "Plugins", "Inspiration"],
    heading: "Pro-level production, end to end", intro: "Start with an idea. Explore the shots. Add sound. Try a different direction. Your next creation begins in one workspace.", tour: "Take a tour", categoriesLabel: "Creative Suite tools",
    categories: [
      { name: "Image", icon: "image", description: "Create images from an idea or a reference. Explore a new style and refine your next frame.", href: "/app/image", image: "product-shots.webp", video: "creative-suite-image.webm" },
      { name: "Video", icon: "video", description: "Find the model for your next scene. Turn a text description or an image into moving pictures.", href: "/app/models", image: "preview-1.webp", video: "creative-suite-video.webm" },
      { name: "Audio", icon: "audio", description: "Create music, give your words a voice, and find the sound that belongs to your idea.", href: "/app/music", image: "preview-4.webp" },
      { name: "Text", icon: "text", description: "Write, research, explore ideas, and find a new perspective in a conversation with AI.", href: "/app", image: "card-yourentire-2x.webp" },
      { name: "Inspiration", icon: "spark", description: "Discover examples and visual directions. Pick a starting point and make the next idea your own.", href: "/app/inspiration", image: "brand-campaign.webp" },
    ],
    platforms: [
      { title: "A workspace of your own", description: "Your conversations, creative tools and files, together in a single place. Pick up where your last idea left off.", action: "Open workspace", href: "/app", image: "card-yourentire-2x.webp" },
      { title: "Connect your creative workflow", description: "Explore the tools available today while new ways to connect your work take shape.", action: "Explore tools", href: "#tools", image: "ai-tools.webp", upcoming: true },
      { title: "Make room for your next idea", description: "Begin with a conversation. Shape a brief, explore a direction, and bring the right AI tools into your process.", action: "Start a conversation", href: "/app", image: "card-workflow-2x.webp", upcoming: true },
      { title: "Create where your ideas take you", description: "Explore your models and results in the browser. Additional integrations will appear here when available.", action: "Open workspace", href: "/app", image: "card-alltools-3-2x.webp", upcoming: true },
      { title: "A new starting point for every idea", description: "Discover visual inspiration, explore different directions, and turn a reference into something of your own.", action: "Explore inspiration", href: "/app/inspiration", image: "brand-campaign.webp" },
    ],
  },
  models: { title: "With all the latest models", description: "Explore leading AI model families and find the right starting point for your work.", label: "AI model families", items: [
    { name: "GPT Image", id: "gpt_image_2", image: "gpt-2.webp" }, { name: "Nano Banana", id: "nano_banana_pro", image: "product-shots.webp" }, { name: "Kling", id: "video_kling_v3", image: "preview-2.webp" }, { name: "Veo", id: "video_veo_3_1_fast", image: "preview-1.webp" }, { name: "Seedream", id: "seedream_5_0_lite", image: "advertising.webp" }, { name: "FLUX", id: "flux_2_pro", image: "brand-campaign.webp" }, { name: "Suno", id: "suno_v6", image: "preview-4.webp" }, { name: "GPT Voice", id: "gpt_4o_mini_tts", image: "advertising.webp" },
  ] },
  workflow: { title: "Start simple.\nScale when you’re ready", description: "From a single tool to a complete creative process, at your own pace.", cards: [
    { title: "Every tool, ready to go", description: "Images, video, music and speech. Open what you need, explore the models, and start making.", image: "card-alltools-3-2x.webp", href: "/app/models" },
    { title: "Your entire creative process, one workspace", description: "Follow your ideas from a conversation to a finished result. Compare directions and come back to your best discoveries.", image: "card-yourentire-2x.webp", href: "/app" },
    { title: "One place, all your work", description: "Your conversations and created files stay close at hand, ready for whatever comes next.", image: "card-oneplace-2x.webp", href: "/app/files" },
    { title: "A new workflow starts with one click", description: "Choose your starting point, explore the settings, and let your idea take shape.", image: "card-workflow-2x.webp", href: "/app/inspiration" },
  ] },
  tools: { title: "Pick a tool, start creating", description: "Generate, explore, experiment — your creative workflow starts here.", label: "Creative tools", filtersLabel: "Filter creative tools", filters: ["Featured", "Image", "Video", "Audio", "Text"], all: "All tools", items: [
    { title: "AI Image Generator", description: "Create images from your words and references.", category: "Image", icon: "image", href: "/app/image" },
    { title: "Video Generator", description: "Find a model to bring your next scene to life.", category: "Video", icon: "video", href: "/app/models" },
    { title: "Music Generator", description: "Turn a mood, a story or an idea into music.", category: "Audio", icon: "audio", href: "/app/music" },
    { title: "Text to Speech", description: "Give your words a natural-sounding voice.", category: "Audio", icon: "voice", href: "/app/speech" },
    { title: "AI Chat", description: "A space to write, research, and explore ideas.", category: "Text", icon: "text", href: "/app" },
    { title: "Visual Inspiration", description: "Find a new direction for your next creation.", category: "Image", icon: "spark", href: "/app/inspiration" },
  ] },
  possibilities: { title: "From product shot to viral phenomenon", description: "Visual ideas for a product, a story, or an entire world. Discover what your creative process could look like.", cards: [
    { title: "Advertising", description: "Explore a brief. Find the visual. Shape your campaign.", image: "advertising.webp", href: "/app/image" },
    { title: "Product shots", description: "See your product from a different perspective.", image: "product-shots.webp", href: "/app/image" },
    { title: "Brand campaigns", description: "Explore a visual language people remember.", image: "brand-campaign.webp", href: "/app/image" },
    { title: "Filmmaking", description: "Find the characters, scenes and worlds of your next story.", image: "preview-2.webp", href: "/app/models" },
  ] },
  originals: { title: "Made of imagination", description: "A world of visual possibilities, ready to spark your next idea.", action: "Explore inspiration", slides: [
    { title: "Beyond the ordinary", description: "Discover a different way to see the familiar.", image: "preview-1.webp", href: "/app/inspiration" },
    { title: "Worlds waiting to happen", description: "Start with a frame. See where the story takes you.", image: "preview-2.webp", href: "/app/inspiration" },
    { title: "Find your own rhythm", description: "Your ideas can take more than one form.", image: "preview-4.webp", href: "/app/music" },
  ] },
  teams: { title: "Creative work. A bigger perspective.", description: "Bring your next project into focus with the models and tools that fit your process.", personal: "For your creativity", personalText: "Explore your ideas with image, video, music, speech and text tools in one account.", business: "For your next big project", businessText: "Tell us what you’re working on. We’ll help you understand the options available in the platform.", contact: "Talk to us", benefits: ["AI tools in one place", "A workspace for your ideas", "Your files, close at hand"] },
  plans: { title: "Find your way to create", description: "Start with your idea. Explore the options in your account before choosing a paid generation.", cards: [
    { name: "Explore", badge: "A place to begin", description: "Get to know your creative workspace.", action: "Get started", href: "/login", features: ["Browse available AI models", "Find image, video and audio tools", "Explore visual inspiration", "Start a conversation"] },
    { name: "Create", badge: "Make it your own", description: "Choose a model and bring your next idea to life.", action: "Start creating", href: "/app", features: ["Choose your generation settings", "Check the price before you start", "Keep your results in My files", "Return to your conversations"] },
    { name: "Go further", badge: "More possibilities", description: "Find a creative process that works for you.", action: "Explore all models", href: "/app/models", features: ["Explore different model families", "Work with available references", "Create across formats", "Get help from our support team"] },
  ] },
  faq: { title: "Answers to your top questions", support: "Contact support", items: [
    { question: `What can I create with ${brandName}?`, answer: "Explore tools for images, video, music, speech and text. Available models and settings are shown in your workspace." },
    { question: "How do I get started?", answer: "Sign in, choose a creative tool or model, and describe your idea. Check the available settings and the price before confirming a generation." },
    { question: "Where are my creations saved?", answer: "Your generated media is available in My files, and your conversations remain in your chat history. Generated files are stored for two calendar months unless you delete them earlier." },
    { question: "How much does it cost to create?", answer: "The price depends on the model and settings. The workspace shows the cost before you start a paid task." },
    { question: "How can I get support?", answer: "Write to support@epifera.com. Tell us what happened and which part of the platform you were using." },
  ] },
  closing: `Be ${brandName}`, footer: { description: "A creative workspace for your next idea.", copyright: "All rights reserved.", legalLabel: "Legal", supportEmail: "support@epifera.com", groups: [
    { title: "Create", links: [{ label: "AI Image Generator", href: "/app/image" }, { label: "Video Generator", href: "/app/models" }, { label: "Music Generator", href: "/app/music" }, { label: "Text to Speech", href: "/app/speech" }, { label: "AI Chat", href: "/app" }] },
    { title: "Explore", links: [{ label: "All AI models", href: "/app/models" }, { label: "Inspiration", href: "/app/inspiration" }, { label: "My files", href: "/app/files" }, { label: "Your workspace", href: "/app" }] },
    { title: "Resources", links: [{ label: "Questions and answers", href: "#questions" }, { label: "Contact support", href: "mailto:support@epifera.com" }, { label: "Legal documents", href: "/legal" }] },
  ], legal: [{ label: "User agreement", href: "/legal/user-agreement" }, { label: "Privacy policy", href: "/legal/privacy-policy" }, { label: "Payment and refunds", href: "/legal/paid-access" }] },
} as const;

export type HomeEnglishCopy = typeof homeEn;
