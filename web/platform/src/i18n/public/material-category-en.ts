import {brandName} from "@/config/brand";
import type {CollectionMember} from "./collection-en";
import {natureReferenceMaterials} from "./material-category-reference-en";

export type MaterialRecord=CollectionMember&{readonly orientation:"landscape"|"portrait"|"square";readonly color:string;readonly dominant:{r:number;g:number;b:number};readonly sourceAI:boolean;readonly license:"unspecified";readonly people:"unspecified";readonly format:"webp";readonly media:"photos"};
export type MaterialFilterKey="media"|"license"|"source"|"orientation"|"color"|"people"|"format";
export type MaterialFilterDefinition={readonly key:MaterialFilterKey;readonly label:string;readonly options:readonly {value:string;label:string;color?:string}[];readonly hint?:string};
const all={value:"all",label:"All"};
export const natureMaterials:readonly MaterialRecord[]=natureReferenceMaterials;
export const materialFilters:readonly MaterialFilterDefinition[]=[
  {key:"media",label:"Photos",options:[{value:"photos",label:"Photos"},{value:"all",label:"All images"}]},
  {key:"license",label:"License",options:[all,{value:"unspecified",label:"License not supplied"}],hint:"License terms are not provided for these reference previews. Check the source terms before reusing an image."},
  {key:"source",label:"AI-generated",options:[all,{value:"ai",label:"AI label in source"},{value:"unspecified",label:"No AI label in source"}]},
  {key:"orientation",label:"Orientation",options:[all,{value:"landscape",label:"Landscape"},{value:"portrait",label:"Portrait"},{value:"square",label:"Square"}]},
  {key:"color",label:"Dominant color",options:[all,...[{value:"green",label:"Green",color:"#4e8660"},{value:"blue",label:"Blue",color:"#5f8eb4"},{value:"orange",label:"Orange",color:"#ce915a"},{value:"yellow",label:"Yellow",color:"#c4b55c"},{value:"purple",label:"Purple",color:"#a17daa"},{value:"red",label:"Red",color:"#b75c63"},{value:"neutral",label:"Neutral",color:"#93958b"}].filter(option=>natureMaterials.some(item=>item.color===option.value))],hint:"Main color is measured from the available preview."},
  {key:"people",label:"People",options:[all,{value:"unspecified",label:"People not specified"}],hint:"People metadata was not supplied. Unspecified does not mean an image contains no people."},
  {key:"format",label:"File type",options:[all,{value:"webp",label:"WebP preview"}]},
];
export const natureCategory={id:"nature-photos",mediaType:"photos",taxonId:"nature",selectionQuery:{tag:"nature"},sortPolicy:["relevance","name","largest"] as const,filterDefinitions:materialFilters,relatedCategoryIds:["forest","mountains","lakes","flowers","rivers","sunset","meadows","wildlife"]} as const;
export const materialCategoryEn={
  metadata:{title:`Nature Photos — Visual References | ${brandName}`,description:`Explore nature photographs, landscapes and visual references. Filter by orientation, color and source labels, then find a new direction for your work in ${brandName}.`},
  skip:"Skip to materials",current:"Nature photos",currentShort:"Photos",search:"Search nature photos",searchButton:"Open material search",searchPlaceholder:"Nature",clearSearch:"Clear material search",searchDialogTitle:"Search nature photos",heading:"Nature Photos",
  intro:"Explore forests, rivers, mountains and quiet moments in nature. Find a visual direction for a background, moodboard or your next creative idea. Take a reference further with our",generator:"AI Image Generator",introEnd:"Find the light, color and composition that make your next idea feel like yours.",
  topics:[{id:"forest",label:"Forest"},{id:"mountains",label:"Mountains"},{id:"lakes",label:"Lakes"},{id:"flowers",label:"Flowers"},{id:"rivers",label:"Rivers"},{id:"sunset",label:"Sunset"},{id:"meadows",label:"Meadows"},{id:"wildlife",label:"Wildlife"}],
  createImage:"Create an AI image",createVideo:"Explore AI video",filterTopic:"Filter topic:",sort:"Sort materials",sorts:[{value:"relevance",label:"Relevance"},{value:"name",label:"Name A–Z"},{value:"largest",label:"Largest preview"}],
  filters:"Filters",openFilters:"Open all filters",showResults:"Show results",reset:"Reset all filters",layout:"Result layout",masonry:"Masonry",square:"Square",close:"Close",results:"reference previews",showing:"Showing",of:"of",empty:"No matching materials",emptyHint:"Try a broader search or remove a filter to see more nature references.",
  banner:`Create with ${brandName}`,bannerClose:"Dismiss creative tools",bannerLinks:[{label:"Images",href:"/app/image"},{label:"Video",href:"/app/models"},{label:"Music",href:"/app/music"},{label:"My files",href:"/app/files"},{label:"All tools",href:"/ai"}],
  pagination:"Material pages",previous:"Previous page",next:"Next page",page:"Page",download:"Download",save:"Save",saved:"Saved",savedNotice:"Saved on this device.",removedNotice:"Removed from saved materials on this device.",saveError:"Could not save on this device.",
  relatedTitle:"Take your next idea further",related:[{label:"Explore cinematic references",href:"/inspiration/cinematic-scenes"},{label:"View a floral example",href:"/inspiration/examples/aesthetic-flowers"},{label:"Create images with AI",href:"/ai/image-generator"},{label:"Build a product campaign",href:"/workflows/product-campaign"}],
} as const;
