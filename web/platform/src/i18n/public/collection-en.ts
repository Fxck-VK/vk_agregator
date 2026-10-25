import {brandName} from "@/config/brand";
import {cinematicMembers} from "./collection-reference-en";

export type CollectionMember={readonly id:string;readonly src:string;readonly alt:string;readonly width:number;readonly height:number;readonly tags:readonly string[];readonly prompt:string};
export type ExampleCollection={readonly id:string;readonly slug:string;readonly title:string;readonly description:string;readonly curator:string;readonly curatorNote:string;readonly memberType:"reference";readonly selectionRule:string;readonly orderedMemberIds:readonly string[];readonly taxonIds:readonly string[];readonly members:readonly CollectionMember[]};
export const cinematicCollection:ExampleCollection={
  id:"cinematic-scenes",slug:"cinematic-scenes",title:"Cinematic scenes",
  description:"Immersive landscapes, atmospheric city scenes, and moments that tell a story. Find a visual direction for your next creation.",
  curator:brandName,memberType:"reference",selectionRule:"A curated sequence of cinematic environments and editorial scenes, moving from nature to the city.",
  curatorNote:"Look at how light, color, framing and scale shape each scene. Open an example for a suggested prompt, then make the idea your own.",
  orderedMemberIds:cinematicMembers.map(member=>member.id),taxonIds:["cinematic","nature","city","people"],members:cinematicMembers,
};
export const collectionEn={
  metadata:{title:`Cinematic scenes — examples and prompts | ${brandName}`,description:cinematicCollection.description},
  skip:"Skip to collection",back:"Back to inspiration",navigation:"Explore Epifera",login:"Log in",signup:"Sign up",pricing:"Pricing",
  search:"Search this collection",searchPlaceholder:"Search this collection",openSearch:"Open collection search",clear:"Clear search",close:"Close",tools:"All tools",openTools:"Open tools",
  save:"Save collection",saved:"Saved collection",savedNotice:"Saved on this device.",removedNotice:"Removed from saved collections on this device.",saveError:"Could not save on this device.",
  share:"Share collection",shared:"Collection link copied.",shareError:"Could not copy the link. Copy it below.",link:"Collection link",by:"by",examples:"examples",
  view:"View example:",use:"Use",usePrompt:"Use this prompt",viewPrompt:"View suggested prompt",exampleActions:"Use example",previous:"Previous example",next:"Next example",thumbnails:"Collection examples",preview:"Example preview",
  prompt:"Suggested prompt",copy:"Copy prompt",copied:"Copied",copyError:"Could not copy. Select and copy the prompt below.",promptNote:"A suggested prompt inspired by this reference, not its original generation settings.",
  empty:"No matching examples",emptyHint:"Try a subject, mood or setting, such as forest, city or night.",results:"matching examples",all:"Show all examples",showResults:"Show results",
  contextTitle:"A starting point for your next idea",relatedTitle:"Keep creating",source:`Visual references: Magnific. Suggested prompts: ${brandName}.`,
  home:"Home",create:"Create an image",current:"This collection",workflows:"Workflows",imageGenerator:"Image Generator",videoGenerator:"Video Generator",musicGenerator:"Music Generator",academy:"Academy",help:"Help",quickNav:"Quick navigation",searchExamples:"Search examples",searchShort:"Search",collection:"Collection",files:"My files",browseTools:"Browse tools",toolsShort:"Tools",gallery:"Cinematic examples",shareShort:"Share",
  links:[{label:"Image Generator",href:"/app/image",icon:"grid"},{label:"Video Generator",href:"/app/models",icon:"play"},{label:"Music Generator",href:"/app/music",icon:"play"},{label:"All AI tools",href:"/ai",icon:"grid"},{label:"Workflows",href:"/workflows/product-campaign",icon:"file"},{label:"Documentation",href:"/ai/docs",icon:"book"}],
  related:[{label:"Browse nature photos",href:"/photos/nature"},{label:"Explore a floral example",href:"/inspiration/examples/aesthetic-flowers"},{label:"Explore image generation",href:"/ai/image-generator"},{label:"Build a product campaign",href:"/workflows/product-campaign"},{label:"Read the prompt guide",href:"/blog/ai-art-prompts"}],
} as const;
