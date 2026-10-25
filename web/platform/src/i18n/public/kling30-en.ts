import { brandName } from "@/config/brand";
import { klingCatalogEn } from "./kling-catalog-en";

const profile=klingCatalogEn.find(model=>model.id==="video_kling_v3")!;
export const kling30En={
  metadata:{title:`Kling 3.0 AI video model — ${brandName}`,description:"Explore Kling 3.0 in Epifera. Review its application settings, browse reference examples, compare Kling versions and prepare your next video brief."},
  modelId:profile.id,
  hero:{status:"In catalog",eyebrow:`Kling on ${brandName}`,title:"Kling 3.0: bring your cinematic ideas to life",description:"Explore Kling 3.0 in Epifera, from a written brief to a new direction for your next video. Review the available duration, resolution and aspect-ratio options, then check the estimate before you create.",action:"Create with Kling 3.0",secondary:"See specs"},
  galleryTitle:"Kling 3.0 reference gallery",dialogTitle:"Kling 3.0 reference example",
  sections:[{id:"whats-new",label:"What's new"},{id:"specs",label:"Specs"},{id:"best-for",label:"Best for"},{id:"numbers",label:"Numbers"}],
  whatsNew:{eyebrow:"What's new",title:"What's different from Kling 2.6",body:"Kling 3.0 offers a choice of 3–15 second clips and a 4K option. Compare these with the 5 or 10 second, 720p and 1080p options for Kling 2.6. Start with a written prompt in Epifera; reference uploads and native-audio selection are not available here.",action:"Explore the Kling family"},
  specs:{eyebrow:"Specs",title:"Kling 3.0 specs",intro:"Review the settings for Kling 3.0 before you start. Your workspace shows the current options and availability for your account.",rows:[
    {label:"Inputs",value:"Text prompt",note:"Image and video reference uploads are not connected in this interface."},
    {label:"Length",value:profile.duration,note:"Choose from the durations offered in the generator."},
    {label:"Resolution",value:profile.resolution},
    {label:"Frame rate",value:"Not confirmed"},
    {label:"Native audio",value:profile.audio,note:"Explore the separate speech and music tools for audio."},
    {label:"References",value:"Not connected",note:"Describe the scene in words. Reference uploads are not available here."},
    {label:"Aspect ratios",value:profile.aspectRatios.join(", ")},
    {label:"Credits",value:"Estimate in workspace",note:"The selected settings determine the estimate before confirmation."},
    {label:"Access",value:"Check current availability",note:"Sign in to check availability and the settings you can use."},
  ],note:"Settings and availability can change. Use the current generator and estimate when deciding to create.",compare:"Compare all Kling versions"},
  best:{eyebrow:"Best for",title:"A starting point for your next scene",intro:"Prepare a clear brief: who is in the scene, what moves, and where the camera goes. Choose the settings for the shot you want to explore.",cards:[
    {title:"Scenes with more than one person",body:"Describe the roles, action and camera movement in your scene. Keep the visual brief clear before choosing the model settings.",action:"Create with Kling 3.0",href:"/app?model=video_kling_v3"},
    {title:"Build the picture, then the sound",body:"Plan the dialogue, music and ambience alongside the image. Native-audio selection is not exposed here; explore speech and music separately.",action:"Explore speech tools",href:"/app/speech"},
    {title:"Product in motion",body:"Write down the product, light and camera move you want to explore. Compare the result with your brief before choosing the next step.",action:"Explore Kling 3.0",href:"/app?model=video_kling_v3"},
  ],aside:{title:"Need another starting point?",body:"Compare the other Kling entries before choosing. Duration, resolution, current availability and the displayed estimate can help you decide which model to explore.",action:"Compare Kling versions"}},
  research:{eyebrow:"Numbers",title:"Kling 3.0 in the reference study",intro:"Figures from Magnific's published usage study. They describe their platform, not Epifera usage.",source:"Source: Magnific usage data, September 5 to October 4, 2026. Shares refer to generations unless a figure says seconds.",link:"View the source page"},
  faqTitle:"Kling 3.0, answered",questions:[
    {q:"When should I choose Kling 3.0 over another version?",a:"Compare the available duration, resolution, current access and estimate for your task. Kling 3.0 offers 3–15 second clips and a 4K option. Other versions may offer a different set of controls."},
    {q:"What is Kling 3.0?",a:"Kling 3.0 is a video model in the Kling family. Start with a written prompt, choose your duration, resolution and aspect ratio, then review the estimate in your Epifera workspace."},
    {q:"How much does Kling 3.0 cost?",a:"Open the model in your workspace, choose your settings and review the current credit estimate before confirming a generation."},
    {q:"What inputs can I use here?",a:"A text prompt. Image and video reference uploads, character-reference controls and native-audio selection are not available here. Your workspace shows the current options."},
    {q:"How do I access Kling 3.0?",a:"Use your Epifera account and open the current catalog entry. The workspace shows whether that model is available to your account and the estimate for the options you select."},
  ],
  closing:{title:"Your next scene starts with a clear idea",body:"Explore Kling 3.0, review the settings, and bring your next brief into the workspace.",action:"Create with Kling 3.0",secondary:"Compare versions"},
  related:{title:"Keep exploring",all:"See all AI tools",groups:[{title:"The Kling family",links:[{label:"Every Kling version",href:"/ai/kling"},{label:"Kling 3.0 Turbo",href:"/app?model=video_kling_3_0_turbo"},{label:"Kling O3 Standard",href:"/app?model=video_kling_o3_standard"},{label:"Kling O1",href:"/app?model=kling_video_o1"},{label:"Kling 2.6",href:"/app?model=kling_2_6"}]},{title:"Tools for the scene",links:[{label:"All video models",href:"/app/models"},{label:"AI image generator",href:"/ai/image-generator"},{label:"Text to speech",href:"/app/speech"},{label:"Music generator",href:"/app/music"}]}]},
} as const;
