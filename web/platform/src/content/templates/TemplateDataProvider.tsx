"use client";
import {createContext,useContext,type ReactNode} from 'react';

export type TemplateData = Readonly<Record<string,unknown>>;
const TemplateContext=createContext<TemplateData|undefined>(undefined);

/** One page's reviewed public data only. Never includes the whole catalogue. */
export function TemplateDataProvider({data,children}:{data:TemplateData;children:ReactNode}){
  return <TemplateContext.Provider value={data}>{children}</TemplateContext.Provider>;
}
export function useTemplateData(){return useContext(TemplateContext);}

export function bindTemplateSources<T>(source:T,bindings:Readonly<Record<string,string>>|undefined,data:TemplateData|undefined):T{
  if(!data||!bindings||!source||typeof source!=='object'||Array.isArray(source))return source;
  const result={...source};
  for(const [key,path]of Object.entries(bindings)){
    const segments=path.split('.');let value:unknown=data;
    for(const segment of segments){
      if(!value||typeof value!=='object'||!Object.prototype.hasOwnProperty.call(value,segment)){value=undefined;break;}
      value=(value as Record<string,unknown>)[segment];
    }
    if(value!==undefined&&(key in result))Object.assign(result,{[key]:value});
  }
  return result;
}
