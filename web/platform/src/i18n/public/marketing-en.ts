import { brandName } from "@/config/brand";

export type MarketingGuide = {
  readonly title: string; readonly description: string; readonly steps: readonly string[];
  readonly href?: string; readonly action?: string; readonly planned?: boolean;
};
const imageSteps = ["Choose an image model and check its supported inputs.", "Describe your visual direction and add a reference if the model supports it.", "Review the settings and cost before creating, then compare the result with your brief."] as const;
export const marketingEn = {
  metadata: { title: `AI Marketing Tools for Creative Teams | ${brandName}`, description: "Explore AI tools for marketing campaigns, social content and brand concepts. Find a creative direction with Epifera’s image, video and audio tools." },
  skip: "Skip to content", announcement: "Where ideas take shape", close: "Close", comingSoon: "Coming soon", source: "Magnific reference artwork", preview: "Workflow guide", planned: "This dedicated workflow is being prepared. Explore the tools currently available in your Epifera workspace.", explore: "Explore available tools",
  hero: { title: "AI marketing tools to create assets and test ideas", description: "Explore AI tools for digital marketers, brand teams, and copywriters who want to develop ideas, compare visual directions, and create their next campaign.", modelsLabel: "Explore models for your next creative direction", models: [{label:"Google",id:"nano_banana_pro"},{label:"OpenAI",id:"gpt_image_2"},{label:"FLUX",id:"flux_2_pro"},{label:"Kling",id:"video_kling_v3"},{label:"Suno",id:"suno_v6"}] },
  workflows: {
    title: "The easiest way to get from idea to done", description: "Set a goal, choose a creative direction, and explore it in your workspace. Move from a written brief to image concepts, video ideas and audio with the tools that fit your project.", filters: "Workflow categories", categories: ["All","Branding","Advertising","Fashion"], viewAll: "View all",
    examples: [
      { category: "Branding", title: "Restyle illustration", description: "Explore another illustration style for an existing visual idea.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { category: "Advertising", title: "Outdoor placement mockup", description: "Develop a visual concept for a campaign in an outdoor setting. Treat the result as a concept rather than a measured production mockup.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { category: "Fashion", title: "Apply design to t-shirt mockup", description: "Explore how a graphic might look on apparel with a model that supports image references. Prepare final print artwork separately.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
    ],
  },
  needs: {
    title: "Smart AI tools for fast-moving marketing teams", label: "Marketing tasks", items: [
      { id: "campaigns", label: "Campaigns", title: "Design full-funnel campaigns at speed", description: "Deadlines don’t wait. Explore campaign concepts, ad variations and promotional visuals in one workspace. Choose the image, video or audio tool for each stage, and keep the brief at the center of the process." },
      { id: "social", label: "Social content", title: "Create social content at the pace of your calendar", description: "Build a pool of visual ideas for your next post or campaign. Try different compositions, moods and formats, then review each result against the platform and audience you have in mind." },
      { id: "branding", label: "Branding", title: "Keep your brand identity in view across channels", description: "Include the colors, mood, references and visual rules that define your brand. Use them to guide new concepts, and review every result before it becomes part of a campaign." },
      { id: "audiences", label: "Audiences", title: "Adapt your content for multiple audiences", description: "Explore a different setting, message or visual direction for a new audience. Develop variations with the available tools and review the language, brand details and final output for each market." },
    ],
  },
  guides: {
    title: "Create campaigns with AI, step by step", description: "Find a starting point for product visuals, ad variations and social content. These guides explain how to explore the idea with Epifera’s available tools.", label: "Marketing guides", previous: "Previous", next: "Next",
    items: [
      { title: "Explore image upscaling", description: "A dedicated image-upscaling workflow is planned. This guide does not promise a specific output resolution.", steps: ["Choose an image you are entitled to use.","Keep the original available for comparison.","Use the workspace’s supported image tools while dedicated upscaling is being prepared."], planned: true },
      { title: "Apply different styles to your images", description: "Try a new visual language for an image concept.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Develop a sticker concept with AI", description: "Explore an illustrated sticker idea as an image. Vector conversion and final SVG preparation are separate steps.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Explore images with visual prompts", description: "Use a reference image when your selected model supports it.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Localize a campaign concept", description: "Develop a message and visual direction for another audience. Review translations and brand details before use.", steps: ["Write the audience, market and purpose of the campaign in your brief.","Explore copy directions in the chat workspace.","Develop matching visuals and review the final language and brand details."], href: "/app", action: "Open workspace" },
      { title: "Create branded visual concepts", description: "Describe the brand cues that should guide your next image.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Create product visuals with AI", description: "Explore lighting, composition and settings for a product concept.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Plan an AI video from your brief", description: "Explore available video models and choose the controls that fit the scene.", steps: ["Describe the subject, action, framing and mood.","Choose a video model and review its supported text or image inputs.","Check the duration, settings and current cost before starting generation."], href: "/app/models", action: "Explore video models" },
      { title: "Explore a recurring character", description: "Use a character description and supported image references to develop variations. Review consistency between results.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Put products in a new context", description: "Explore where a product might appear in your next campaign.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
      { title: "Explore mockup ideas with Nano Banana", description: "Try a design-placement concept with the reference inputs available for your chosen model.", steps: imageSteps, href: "/app/image", action: "Open image generator" },
    ] satisfies readonly MarketingGuide[],
  },
  plans: {
    title: "A creative process that works for you", description: "Choose your tools and review the current cost in the workspace before creating.", items: [
      { label: "Start with the brief", title: "Explore your direction", description: "Turn a campaign goal into a visual idea you can develop.", points: ["Define your audience and message","Explore image concepts","Compare creative directions"], action: "Open your workspace", href: "/app" },
      { label: "Choose your tools", title: "Create with a model", description: "Find the model and supported settings that fit your next asset.", points: ["Image, video and audio tools","Model-specific input options","Review the price before creating"], action: "Explore models", href: "/app/models" },
      { label: "Keep the process moving", title: "Refine and review", description: "Develop another version, save the result and prepare it for your campaign.", points: ["Refine the creative brief","Download your results","Manage creations in My files"], action: "Open My files", href: "/app/files" },
    ],
  },
  feedback: { title: "Help shape your creative workspace", description: "Tell us which marketing workflows you need next. Share a suggestion or a question with Epifera Support.", action: "Share your feedback", href: "mailto:support@epifera.com" },
  tools: {
    title: "Stay competitive with the right AI marketing tools", description: "Find the tools for your next idea. Available features open in the workspace; dedicated tools still in preparation are marked Coming soon.", label: "Marketing tools", previous: "Previous", next: "Next", items: [
      { title: "Spaces", description: "A shared canvas for connected creative workflows is planned.", planned: true },
      { title: "AI image generator", description: "Explore visual ideas from a written brief.", href: "/ai/image-generator" },
      { title: "AI video generator", description: "Explore video models and their supported inputs.", href: "/ai/kling" },
      { title: "AI Assistant", description: "Develop your next brief in the chat workspace.", href: "/app" },
      { title: "Image Editor", description: "A dedicated image-editing workspace is planned.", planned: true },
      { title: "Video Editor", description: "A dedicated video-editing workspace is planned.", planned: true },
      { title: "Image Upscaler", description: "A dedicated image-upscaling workflow is planned.", planned: true },
      { title: "Video Upscaler", description: "A dedicated video-upscaling workflow is planned.", planned: true },
      { title: "Text to Speech", description: "Explore speech and voice tools in your workspace.", href: "/app/speech" },
      { title: "Background Remover", description: "A dedicated background-removal tool is planned.", planned: true },
      { title: "Expand", description: "A dedicated image-expansion tool is planned.", planned: true },
      { title: "Retouch", description: "A dedicated retouching tool is planned.", planned: true },
      { title: "Mockup Generator", description: "A dedicated mockup workflow is planned. Image models can already help explore concepts.", planned: true },
      { title: "Reimagine", description: "Explore a new image direction in the image generator.", href: "/app/image" },
      { title: "Designer", description: "A dedicated template-design workspace is planned.", planned: true },
      { title: "AI icon concepts", description: "Explore illustrated icon ideas with image models.", href: "/app/image" },
    ],
  },
} as const;
