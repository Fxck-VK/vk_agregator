"use client";
import {useCallback,useId,useRef,useState} from "react";
import {PopoverPanel} from "@/components/ui/PopoverPanel/PopoverPanel";
import {ScrollArea} from "@/components/ui/ScrollArea/ScrollArea";
import {ModeSwitchPanel} from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {materialCategoryEn as t,materialFilters,type MaterialFilterDefinition,type MaterialFilterKey} from "@/i18n/public/material-category-en";
import {initialMaterialFilters,type MaterialFilters as FilterValues,type MaterialLayout} from "./category-model";
import styles from "./MaterialCategory.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";


export function MaterialLayoutIcon({square=false}:{square?:boolean}){
  return <svg aria-hidden="true" viewBox="0 0 24 24" fill="currentColor">{square?Array.from({length:9},(_,i)=><rect key={i} x={4+i%3*6} y={4+Math.floor(i/3)*6} width="4" height="4" rx=".5"/>):<><rect x="4" y="4" width="7" height="10" rx="1"/><rect x="13" y="4" width="7" height="6" rx="1"/><rect x="4" y="16" width="7" height="4" rx="1"/><rect x="13" y="12" width="7" height="8" rx="1"/></>}</svg>;
}
function FilterPill({definition,value,onChange}:{definition:MaterialFilterDefinition;value:string;onChange:(key:MaterialFilterKey,value:string)=>void}){
  const {initialMaterialFilters} = usePublicCopy(publicCopySourcesForLocale,{"t":"materialCategoryEn","initialMaterialFilters":"initialMaterialFilters","materialFilters":"materialFilters"});
  const locale=useLocale();

  const [open,setOpen]=useState(false),anchor=useRef<HTMLButtonElement>(null),id=useId(),close=useCallback(()=>setOpen(false),[setOpen]),selected=definition.options.find(option=>option.value===value),active=value!==initialMaterialFilters[definition.key];
  return <><button type="button" ref={anchor} className={styles.filterPill} data-active={active||undefined} aria-label={translatePublicText("{label} options",locale).replace("{label}",definition.label)} aria-haspopup="dialog" aria-expanded={open} aria-controls={open?id:undefined} onClick={()=>setOpen(current=>!current)}>{definition.label}{active?<small>{selected?.label}</small>:null}<span aria-hidden="true">⌄</span></button><PopoverPanel anchorRef={anchor} id={id} isOpen={open} onClose={close} label={definition.label} width={275} itemVariant="action"><fieldset className={styles.filterOptions}><legend>{definition.label}</legend>{definition.options.map(option=><label key={option.value}><input type="radio" name={id} value={option.value} checked={value===option.value} onChange={()=>{onChange(definition.key,option.value);close();anchor.current?.focus();}}/>{option.color?<span className={styles.colorDot} style={{background:option.color}}/>:null}{option.label}</label>)}{definition.hint?<p>{definition.hint}</p>:null}</fieldset></PopoverPanel></>;
}
export function MaterialFilterBar({values,onChange,layout,onLayout,onAllFilters}:{values:FilterValues;onChange:(key:MaterialFilterKey,value:string)=>void;layout:MaterialLayout;onLayout:(layout:MaterialLayout)=>void;onAllFilters:()=>void}){
  const {materialFilters,t} = usePublicCopy(publicCopySourcesForLocale,{"t":"materialCategoryEn","initialMaterialFilters":"initialMaterialFilters","materialFilters":"materialFilters"});
  const locale=useLocale();

  return <div className={styles.filterBar}><ScrollArea orientation="horizontal" className={styles.filterScroll} viewportClassName={styles.filterViewport} viewportProps={{"aria-label":translatePublicText("Material filters",locale)}}>{materialFilters.map(definition=><FilterPill key={definition.key} definition={definition} value={values[definition.key]} onChange={onChange}/>)}</ScrollArea><button type="button" className={styles.allFilters} aria-label={t.openFilters} onClick={onAllFilters}>{t.filters}<span aria-hidden="true">☷</span></button><ModeSwitchPanel<MaterialLayout> ariaLabel={t.layout} className={styles.layoutSwitch} activeID={layout} onChange={onLayout} iconOnly items={[{id:"masonry",label:t.masonry,icon:<MaterialLayoutIcon/>},{id:"square",label:t.square,icon:<MaterialLayoutIcon square/>}]}/></div>;
}
export function MaterialFilterDialog({values,onChange,onApply,onReset,onClose,count}:{values:FilterValues;onChange:(key:MaterialFilterKey,value:string)=>void;onApply:()=>void;onReset:()=>void;onClose:()=>void;count:number}){
  const {t,materialFilters} = usePublicCopy(publicCopySourcesForLocale,{"t":"materialCategoryEn","initialMaterialFilters":"initialMaterialFilters","materialFilters":"materialFilters"});

  return <LandingDialog title={t.filters} closeLabel={t.close} onClose={onClose}><div className={styles.mobileFilterFields}>{materialFilters.map(definition=><label key={definition.key}><span>{definition.label}</span><select aria-label={definition.label} value={values[definition.key]} onChange={event=>onChange(definition.key,event.target.value)}>{definition.options.map(option=><option key={option.value} value={option.value}>{option.label}</option>)}</select>{definition.hint?<small>{definition.hint}</small>:null}</label>)}<p aria-live="polite">{count} {t.results}</p><div className={styles.dialogActions}><button type="button" onClick={onReset}>{t.reset}</button><button type="button" onClick={onApply}>{t.showResults}</button></div></div></LandingDialog>;
}

const publicCopySourcesForLocale={t,materialFilters,initialMaterialFilters};
