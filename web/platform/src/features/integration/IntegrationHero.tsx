"use client";
import {useId,useRef} from "react";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {LandingArrow} from "@/components/public/landing/LandingPrimitives";
import {keyboardTab} from "@/features/public-home/HomeSuite";
import {mcpClients,mcpEn as t} from "@/i18n/public/integration-en";
import {integrationReference} from "@/i18n/public/integration-reference-en";
import {IntegrationClientIcon} from "./IntegrationClientIcon";
import styles from "./Integration.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function IntegrationHero({selected,onSelect,onStatus,onGuide}:{selected:number;onSelect:(index:number)=>void;onStatus:()=>void;onGuide:()=>void}){
  const {mcpClients,integrationReference,t} = usePublicCopy(publicCopySourcesForLocale);

 const tabs=useRef<(HTMLButtonElement|null)[]>([]),id=useId(),client=mcpClients[selected],image=integrationReference.images.hero;
 const select=(index:number,focus=false)=>{onSelect(index);if(focus){tabs.current[index]?.focus();tabs.current[index]?.scrollIntoView?.({block:"nearest",inline:"nearest"});}};
 return <section className={styles.hero} aria-labelledby={`${id}-title`}><div className={styles.heroBackground}><MediaImage src={image.src} width={image.width} height={image.height} alt="" fit="cover" passive loading="eager" showLoading={false}/></div><div className={styles.heroShade}/><div className={styles.heroInner}><div className={styles.badges}><span>{t.badge}</span><span>{t.planned}</span></div><h1 id={`${id}-title`}>{t.hero.title}</h1><p className={styles.heroDescription}>{t.hero.description}</p><div id="connection" className={styles.connection}>
 <div className={styles.clientTabs} role="tablist" aria-label={t.controls.clients}>{mcpClients.map((item,index)=><button type="button" role="tab" key={item.id} id={`${id}-${item.id}`} aria-controls={`${id}-panel`} aria-selected={selected===index} tabIndex={selected===index?0:-1} ref={node=>{tabs.current[index]=node;}} onClick={()=>select(index)} onKeyDown={event=>{const next=keyboardTab(event,index,mcpClients.length);if(next!==null)select(next,true);}}><span aria-hidden="true" className={styles.clientMark}>{item.icon?<IntegrationClientIcon name={item.icon}/>:item.mark}</span>{item.name}</button>)}</div>
 <div className={styles.connectionCard} role="tabpanel" id={`${id}-panel`} aria-labelledby={`${id}-${client.id}`} tabIndex={0}><ol>{client.steps.map(step=><li key={step}>{step}</li>)}</ol><div className={styles.connectionActions}><p className={styles.connectionField}>{t.controls.details}</p><button type="button" className={styles.primary} onClick={onStatus}>{t.controls.status}<LandingArrow/></button></div><button type="button" className={styles.guideLink} onClick={onGuide}>{t.controls.guide}<span aria-hidden="true">↗</span></button></div>
 </div></div></section>;
}

const publicCopySourcesForLocale={mcpClients,t,integrationReference};
