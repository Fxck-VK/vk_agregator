import {brandName} from "@/config/brand";
import type {ArticleBlock,ArticleInline,ArticleRecord} from "./article-types";
const bold=(text:string):ArticleInline=>({text,strong:true});
const link=(text:string,href:`/${string}`|`#${string}`):ArticleInline=>({text,href});
const p=(...content:ArticleInline[]):ArticleBlock=>({type:"paragraph",content});
const item=(...content:ArticleInline[])=>({content});
const sections:ArticleRecord["sections"]=[
  {id:"what-is-an-ai-prompt",label:"What is an AI Prompt?",title:"What is an AI Prompt?",blocks:[
    p(bold("An AI art prompt is a creative brief.")," It tells an image model what you want to see: a subject, a setting, a composition and a visual direction. A useful brief describes the details that matter to the picture instead of collecting impressive-sounding words."),
    p("Think about the difference between “a portrait” and a portrait with a specific crop, expression and light. Both are valid starting points. The second gives you clearer details to review when an image appears. A prompt does not need to describe every leaf or thread; it needs to make your priorities understandable."),
    {type:"image",imageIndex:0},
  ]},
  {id:"how-do-ai-art-generators-work",label:"How do AI art generators work?",title:"How do AI art generators work?",blocks:[
    p("An image model uses your instructions to produce a visual interpretation. The selected model, the words you provide and the settings you choose all contribute to the result. A useful way to approach the process is to separate the brief from the controls and the review."),
    p(bold("Describe the picture.")," The prompt communicates what belongs in the scene and how it should feel. Concrete relationships help: a cup on a table, a person beside a window, or a mountain behind a lake. Explain which element should draw the eye first."),
    p(bold("Choose the available controls.")," Aspect ratio, resolution and reference inputs depend on the selected model. Use the workspace’s resolution controls for output dimensions; writing “8K” into a sentence does not set the exported file size. A reference can help explain a direction when the chosen model supports it."),
    p(bold("Review the interpretation.")," Check the result against your brief. Look at the framing, important details, unwanted objects and any text. Two generations can differ even with similar wording. Treat each result as a direction to inspect and refine, rather than assuming that a detailed sentence guarantees an exact picture."),
    p("You can explore the available workflow in the ",link("image generator","/ai/image-generator"),". If photo references matter to your idea, the ",link("avatar model comparison","/compare/ai-avatar-models")," explains how the listed input controls differ."),
  ]},
  {id:"how-to-write-an-ai-art-prompt",label:"How to write an AI art prompt",title:"How to write an AI art prompt",blocks:[
    p("Start with a plain description of the image you actually need. For example, replace “a dog” with the kind of dog, what it is doing and where it is. Then add the few visual decisions that make this particular picture useful to you."),
    {type:"list",items:[
      item(bold("Subject and action: "),"Name the main subject and describe its pose or activity. “A small white dog looking up from an open book” gives you a more useful focal point than a list of unrelated objects."),
      item(bold("Setting and relationships: "),"Say where the subject belongs and how the scene is arranged. Describe the foreground, background and any space you need to keep clear. A product image for a banner may need room beside the subject."),
      item(bold("Light, color and mood: "),"Choose a direction that fits the idea: soft window light, a cool night palette, warm sunset reflections or a cheerful illustrated garden. Keep the lighting description consistent with the setting."),
      item(bold("Visual style: "),"Describe the medium or visual qualities you want, such as a watercolor illustration, a textured collage or a studio photograph. Specific qualities are easier to revise than a long stack of style labels."),
      item(bold("Framing and important details: "),"Ask for a close-up, a wide view or a centered composition when it matters. Mention materials, expressions and shapes that need attention. Set the actual aspect ratio and output size in the controls offered by the selected model."),
    ]},
    {type:"image",imageIndex:1},
  ]},
  {id:"tips-to-write-better-prompts",label:"Tips to write better prompts",title:"Tips to write better prompts",blocks:[
    p("The most useful prompt is one you can understand, test and revise. Keep a clear relationship between the words you change and the part of the picture you want to improve. These habits make that process easier:"),
    {type:"list",ordered:true,items:[
      item(bold("Use direct language. "),"Write the subject and the important relationships plainly. “A red ceramic cup on a pale wooden table” is a clear starting point. Add specialist terms when they communicate a visual decision you can recognize in the result."),
      item(bold("Put priorities first. "),"Decide what makes the image successful before adding decorative details. If the crop and expression matter most, describe them clearly. A longer prompt is useful only when the extra information serves the picture; there is no universal ideal word count."),
      item(bold("Keep the brief consistent. "),"Check for conflicting directions, such as an empty background filled with objects. You can combine styles deliberately, but explain the relationship rather than leaving the model to guess which instruction should dominate."),
      item(bold("Change one important variable. "),"When you like the composition but want softer light, revise the lighting description first. Rewriting the subject, palette and framing at the same time makes it harder to learn which change helped."),
      item(bold("Review at the intended size. "),"Inspect the image closely for small errors, then check it in the crop where it will be used. A background that looks attractive full-screen may distract from a tiny profile image. Keep useful wording alongside the result so you can build on it."),
    ]},
  ]},
  {id:"ai-art-prompt-examples-to-try",label:"AI art prompt examples to try",title:"AI art prompt examples to try",blocks:[
    p(bold("Use these briefs as starting points.")," They describe four different visual directions. Adapt the subject, crop and atmosphere to your project, choose a model and check its settings before generating. The accompanying pictures are reference illustrations, not tested outputs from these rewritten briefs."),
    {type:"list",items:[
      {example:true,content:[bold("Lakeside landscape: "),"A quiet mountain lake at dusk, distant peaks reflected in still water, pine trees framing the foreground, a warm strip of sunset near the horizon, soft blue shadows, detailed digital illustration with a calm atmosphere and a wide composition."]},
      {example:true,content:[bold("Futuristic city: "),"A rain-soaked street in a fictional city at night, magenta and cyan lights reflected on the pavement, a few pedestrians beneath umbrellas, tall buildings fading into mist, cinematic photographic style, eye-level view with a clear path through the scene."]},
      {example:true,content:[bold("Vineyard portrait: "),"A fictional adult character in a cream linen jacket standing beside grapevines, relaxed expression, warm afternoon light coming from the side, soft green background, natural portrait photography, chest-up composition with attention on the face and the texture of the fabric."]},
      {example:true,content:[bold("Playful illustration: "),"A small fluffy white dog sitting beside an open storybook in a colorful garden, curious expression, rounded shapes, soft painted textures, bright flowers and gentle daylight, a cheerful children’s-book illustration with an uncluttered central composition."]},
    ]},
    {type:"image",imageIndex:2},
  ]},
];
const lead=[
  p(bold("A good image starts with a clear idea.")," Writing an AI art prompt is a way to turn that idea into a brief: what the subject is, where it belongs, how the light falls and what the picture should feel like. You can start with everyday language and add detail as your direction becomes clearer."),
  p("This guide walks through the building blocks of a prompt, practical ways to refine it and four examples you can adapt. When you are ready, open the ",link("Epifera image generator","/app/image")," and choose the model and settings that fit your project."),
];
const closing=[
  p("Keep the parts of a brief that work and revise the parts that do not. The goal is a repeatable creative process: describe an idea, inspect the result and make a deliberate next change. A beautiful first image is useful, but understanding why it fits your project helps you develop the next one."),
  p("For another starting point, explore the ",link("image-generator guide","/ai/image-generator")," or see how a brief becomes a ",link("marketing visual direction","/ai/marketing"),". You can also ",link("compare avatar model controls","/compare/ai-avatar-models")," before choosing an option for a portrait concept."),
];
export const artPromptsEn={
  metadata:{title:`How to Write AI Art Prompts | ${brandName}`,description:"Learn how to describe a subject, setting, style, lighting and framing for AI images. Explore practical writing tips and four example briefs with Epifera."},
  title:"How to write AI art prompts for generating images",lead,sections,closing,
  breadcrumbs:[{label:"Blog"},{label:"Guides"}],
  notes:"Epifera editorial draft · October 25, 2026. Illustrations are from the saved Magnific reference and do not represent Epifera generation results.",
  controls:{home:`Go to ${brandName}`,search:"Search",tools:"AI tools",openWorkspace:"Open workspace",breadcrumb:"Breadcrumb",listen:"Listen to this article",share:"Share this article",shareLink:"Share link",copy:"Copy link",copied:"Link copied",copying:"Copying…",copyFailure:"Select and copy the link below. Clipboard access was unavailable.",pageLink:"Page link",close:"Close",comingSoon:"Coming soon",audioTitle:"Audio reading",audioDescription:"Audio narration for this article is not available yet. You can keep reading here or explore the speech tools separately.",speech:"Explore speech tools",backTop:"Back to top"},
  source:"Magnific reference artwork",onPage:"On this page",skip:"Skip to content",readTime:"min read",author:`By ${brandName}`,draft:"Draft · October 25, 2026",
  inspiration:{title:"Find your next visual idea",action:"Explore inspiration",href:"/app/inspiration"},
  related:{title:"Related guides",items:[
    {category:"Creative tools",title:"Learn Epifera essentials",description:"Work through short lessons on briefs, models, media and your results.",href:"/academy/courses/epifera-essentials"},
    {category:"Comparisons",title:"Compare models for avatar concepts",description:"Review reference inputs and output controls across eight image models.",href:"/compare/ai-avatar-models"},
    {category:"For marketers",title:"Build a campaign direction",description:"Explore briefs, visual concepts and the tools for your next campaign.",href:"/ai/marketing"},
  ]},
} as const satisfies ArticleRecord & Record<string,unknown>;
