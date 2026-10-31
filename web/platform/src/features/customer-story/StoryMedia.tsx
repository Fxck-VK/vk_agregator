"use client";
import Image from 'next/image';
import {MediaImage} from '@/components/media/MediaImage/MediaImage';
import {MediaPreviewDialogTemplate} from '@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate';
import {LandingCarousel} from '@/components/public/landing/LandingCarousel';
import {customerStoryEn as t} from '@/i18n/public/customer-story-en';
import {customerStoryReference as media} from '@/i18n/public/customer-story-reference-en';
import styles from './CustomerStory.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

const images=[media['gallery-1'],media['gallery-2'],media['gallery-3']];
export const caseGallery=t.gallery.items.map((item,index)=>({...item,...images[index]}));
export const getCaseGallery=(copy:typeof t,art:typeof media)=>copy.gallery.items.map((item,index)=>({...item,...art[`gallery-${index+1}` as 'gallery-1'|'gallery-2'|'gallery-3']}));
export function StoryGallery({onView}:{onView:(index:number)=>void}){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"customerStoryEn","media":"customerStoryReference"});
  const caseGallery=getCaseGallery(t,media);
return <section className={styles.gallery} aria-label={t.gallery.title}><LandingCarousel label={t.controls.gallery} previous={t.controls.previous} next={t.controls.next} bounded className={styles.galleryStrip}>{caseGallery.map((item,index)=><button type="button" key={item.id} aria-label={`${t.controls.enlarge}: ${item.title}`} onClick={()=>onView(index)}><span><Image src={item.src} width={item.width} height={item.height} alt="" sizes="(max-width:679px) 85vw, 320px"/></span><h3>{item.title}</h3></button>)}</LandingCarousel><p>{t.gallery.note}</p></section>;}
export function StoryViewer({index,onSelect,onClose}:{index:number;onSelect:(index:number)=>void;onClose:()=>void}){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"customerStoryEn","media":"customerStoryReference"});
  const caseGallery=getCaseGallery(t,media);
return <MediaPreviewDialogTemplate ariaLabel={t.gallery.viewer} backdropTestId="customer-story-preview-backdrop" closeLabel={t.controls.close} items={caseGallery} selectedIndex={index} onSelect={onSelect} onClose={onClose} previousLabel={t.gallery.previous} nextLabel={t.gallery.next} thumbnailRailLabel={t.gallery.thumbnails} testIdPrefix="customer-story-preview" getItemKey={item=>item.id} getPreviewDimensions={item=>item} getThumbnailLabel={item=>item.title} renderPreview={(item,classes)=><MediaImage src={item.src} alt={item.alt} width={item.width} height={item.height} fit="contain" className={classes.previewMedia}/>} renderThumbnail={(item,classes)=><MediaImage src={item.src} alt="" width={item.width} height={item.height} fit="cover" passive loading="lazy" className={classes.thumbnailMedia}/>} renderPreviewFooter={item=><p className={styles.viewerCaption}>{item.title} · {t.controls.reference}</p>}/>;}
export function StoryFilm({index}:{index:number}){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"customerStoryEn","media":"customerStoryReference"});
const film=t.films[index],picture=media[film.key];return <figure className={styles.film}><a href={`https://www.youtube.com/watch?v=${film.id}`} target="_blank" rel="noopener noreferrer" data-case-film aria-label={`${t.watchOriginal}: ${film.title}`}><Image src={picture.src} width={picture.width} height={picture.height} alt={film.description} sizes="(max-width:1000px) 90vw, 70vw"/><span className={styles.filmTitle}>{film.description}</span><span className={styles.play} aria-hidden="true"><svg viewBox="0 0 24 24" fill="currentColor"><path d="m9 5 11 7-11 7Z"/></svg></span><span className={styles.videoDestination}>{t.watchOriginal}<span aria-hidden="true">↗</span></span></a><figcaption>{film.title} <span>· {t.outboundNote}</span></figcaption></figure>;}

const publicCopySourcesForLocale={t,media,images,caseGallery};
