import type {MaterialRecord} from "@/i18n/public/material-category-en";
export type MaterialLayout="masonry"|"square";
export type MaterialSort="relevance"|"name"|"largest";
export type MaterialFilters={query:string;topic:string;media:string;license:string;source:string;orientation:string;color:string;people:string;format:string;sort:MaterialSort};
export const initialMaterialFilters:MaterialFilters={query:"Nature",topic:"all",media:"photos",license:"all",source:"all",orientation:"all",color:"all",people:"all",format:"all",sort:"relevance"};
export function filterMaterials(items:readonly MaterialRecord[],filters:MaterialFilters,searchText:(item:MaterialRecord)=>string=item=>`${item.alt} ${item.tags.join(" ")}`){
  const words=filters.query.normalize("NFKD").replace(/\p{M}/gu,"").toLowerCase().trim().split(/\s+/).filter(Boolean);
  const found=items.filter(item=>{
    if(!words.every(word=>searchText(item).normalize("NFKD").replace(/\p{M}/gu,"").toLowerCase().includes(word)))return false;
    if(filters.topic!=="all"&&!item.tags.includes(filters.topic))return false;
    if(filters.source==="ai"&&!item.sourceAI||filters.source==="unspecified"&&item.sourceAI)return false;
    return (["media","license","orientation","color","people","format"] as const).every(key=>filters[key]==="all"||filters[key]===item[key]);
  });
  if(filters.sort==="name")found.sort((a,b)=>a.alt.localeCompare(b.alt,"en"));
  else if(filters.sort==="largest")found.sort((a,b)=>b.width*b.height-a.width*a.height);
  return found;
}
export function paginateMaterials<T>(items:readonly T[],requestedPage:number,pageSize=24){const pages=Math.max(1,Math.ceil(items.length/pageSize)),page=Math.max(1,Math.min(pages,Math.floor(requestedPage)||1)),start=(page-1)*pageSize;return {items:items.slice(start,start+pageSize),page,pages,start:items.length?start+1:0,end:Math.min(items.length,start+pageSize)};}
export function packMaterials(items:readonly {width:number;height:number}[],width:number,viewport:number,layout:MaterialLayout,gap=24){
  const desired=viewport<640?1:viewport<768?2:layout==="square"&&viewport>=1280?4:viewport>=1536?4:3,columns=Math.min(desired,Math.max(1,Math.floor((width+gap)/(100+gap)))),cardWidth=Math.max(0,(width-(columns-1)*gap)/columns),heights=Array<number>(columns).fill(0);
  const cards=items.map((item,index)=>{const column=layout==="square"?index%columns:heights.indexOf(Math.min(...heights)),height=layout==="square"?cardWidth:cardWidth*item.height/item.width,y=heights[column];heights[column]=y+height+gap;return {x:column*(cardWidth+gap),y,width:cardWidth,height};});
  return {columns,cards,height:Math.max(0,Math.max(...heights)-gap)};
}
