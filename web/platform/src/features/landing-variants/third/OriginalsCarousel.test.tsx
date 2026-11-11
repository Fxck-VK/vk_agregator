import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { LocaleProvider } from '@/i18n/LocaleProvider';
import { homeEn } from '@/i18n/public/home-en';
import { OriginalsCarousel } from './OriginalsCarousel';
vi.mock('next/navigation', () => ({ useRouter: vi.fn() }));
vi.mock('@/features/release/ReleaseViewProvider', () => ({ useReleaseView: () => ({ view:'dev', canSwitch:true, localSimulation:true, enabled:()=>true, allowsPath:()=>true }) }));

beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal('matchMedia', () => ({ matches:false,addEventListener(){},removeEventListener(){} }));
  vi.stubGlobal('IntersectionObserver', class {
    constructor(private callback: IntersectionObserverCallback) {}
    observe() { this.callback([{isIntersecting:true}] as IntersectionObserverEntry[],this as unknown as IntersectionObserver); }
    disconnect() {}
  });
  vi.stubGlobal('PointerEvent', class extends MouseEvent {
    readonly pointerType: string; readonly pointerId: number;
    constructor(type:string,init:PointerEventInit={}) {super(type,init);this.pointerType=init.pointerType??'mouse';this.pointerId=init.pointerId??1;}
  });
});
afterEach(() => { cleanup();vi.useRealTimers();vi.unstubAllGlobals(); });
function mount() {
  const {container}=render(<LocaleProvider locale="ru"><OriginalsCarousel copy={homeEn}/></LocaleProvider>);
  const region=screen.getByRole('region',{name:homeEn.originals.title});
  const dots=within(region).getAllByRole('button');
  return {container,region,dots};
}
const selected=(region:HTMLElement,index:number)=>expect(region).toHaveAttribute('data-original-selected',String(index));
function settle(region:HTMLElement) {
  fireEvent.transitionEnd(region.querySelector('[data-original-track]')!,{propertyName:'transform'});
  act(()=>vi.advanceTimersByTime(80));
  act(()=>vi.advanceTimersByTime(20));
}

it('exposes one active card and three dots while loop copies stay hidden and inert',()=>{
  const {region,dots}=mount();selected(region,0);
  expect(dots).toHaveLength(3);
  expect(within(region).getAllByRole('link')).toHaveLength(1);
  expect(within(region).getByRole('heading',{name:homeEn.originals.slides[0].title})).toBeInTheDocument();
  for(const card of region.querySelectorAll('[data-original-card][data-active="false"]')){
    expect(card).toHaveAttribute('aria-hidden','true');expect(card.querySelector('[inert]')).not.toBeNull();
  }
});
it.each(['neighbor','dot'])('continues forward from the last card through a %s click',method=>{
  const {region,dots}=mount();
  fireEvent.click(dots[2]);settle(region);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','5');
  if(method==='neighbor')fireEvent.click(region.querySelector('[data-original-card="6"]')!,{detail:1});
  else fireEvent.click(dots[0]);
  selected(region,0);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','6');
  settle(region);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','3');
});
it('moves whole cards, honours rapid choices and normalizes both loop boundaries',()=>{
  const {region,dots}=mount();
  fireEvent.click(dots[1]);fireEvent.click(dots[2]);settle(region);selected(region,2);settle(region);
  fireEvent.keyDown(dots[2],{key:'ArrowRight'});selected(region,0);settle(region);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','3');
  fireEvent.keyDown(dots[0],{key:'ArrowLeft'});selected(region,2);settle(region);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','5');
  expect(dots[2]).toHaveFocus();
});
it('queues repeated next actions across the boundary without reversing or leaving the loop copies',()=>{
  const {region,dots}=mount();fireEvent.click(dots[2]);settle(region);
  fireEvent.keyDown(dots[2],{key:'ArrowRight'});
  fireEvent.keyDown(dots[0],{key:'ArrowRight'});
  fireEvent.keyDown(dots[0],{key:'ArrowRight'});
  selected(region,0);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','6');
  settle(region);selected(region,2);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','5');
  settle(region);
  expect(region.querySelector('[data-original-track]')).toHaveAttribute('data-position','5');
});
it('advances automatically and pauses for hover, focus and a hidden document',()=>{
  const {region,dots}=mount();
  act(()=>vi.advanceTimersByTime(6000));selected(region,1);settle(region);
  fireEvent.pointerEnter(region,{pointerType:'mouse'});
  act(()=>vi.advanceTimersByTime(9000));selected(region,1);
  fireEvent.pointerLeave(region,{pointerType:'mouse'});
  fireEvent.focus(dots[1]);act(()=>vi.advanceTimersByTime(9000));selected(region,1);
  fireEvent.blur(dots[1],{relatedTarget:document.body});
  const hidden=vi.spyOn(document,'hidden','get').mockReturnValue(true);
  fireEvent(document,new Event('visibilitychange'));act(()=>vi.advanceTimersByTime(9000));selected(region,1);
  hidden.mockReturnValue(false);fireEvent(document,new Event('visibilitychange'));
  act(()=>vi.advanceTimersByTime(6000));selected(region,2);hidden.mockRestore();
});
it('keeps reduced motion manual and preserves the active destination',()=>{
  vi.stubGlobal('matchMedia',()=>({matches:true,addEventListener(){},removeEventListener(){}}));
  const {region,dots}=mount();act(()=>vi.advanceTimersByTime(20000));selected(region,0);
  fireEvent.click(dots[2]);selected(region,2);
  expect(region).toHaveAttribute('data-original-animate','false');
  expect(within(region).getByRole('link')).toHaveAttribute('href',`/ru${homeEn.originals.slides[2].href}`);
});
it('swipes horizontally without intercepting vertical wheel scrolling or activating a dragged link',()=>{
  const {region}=mount();const viewport=region.querySelector('[data-original-viewport]')!;
  fireEvent.pointerDown(viewport,{pointerType:'touch',clientX:240,clientY:100});
  fireEvent.pointerMove(viewport,{pointerType:'touch',clientX:140,clientY:103});
  fireEvent.pointerUp(viewport,{pointerType:'touch',clientX:140,clientY:103});selected(region,1);settle(region);
  expect(fireEvent.click(within(region).getByRole('link'),{detail:1})).toBe(false);
  expect(fireEvent.wheel(viewport,{deltaY:200})).toBe(true);
  fireEvent.pointerDown(viewport,{pointerType:'touch',clientX:180,clientY:100});
  fireEvent.pointerMove(viewport,{pointerType:'touch',clientX:184,clientY:210});
  fireEvent.pointerCancel(viewport);selected(region,1);
});
