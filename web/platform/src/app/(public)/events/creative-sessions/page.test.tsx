import {cleanup,fireEvent,render,screen,within,waitFor} from '@testing-library/react';
import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest';
import {LocaleProvider} from '@/i18n/LocaleProvider';
const request=vi.hoisted(()=>({locale:'en' as 'en'|'ru'}));
vi.mock('server-only',()=>({}));
vi.mock('@/i18n/server',()=>({getRequestLocale:async()=>request.locale}));
import Page,{generateMetadata} from './page';
import {eventProject,eventProjectEn as t} from '@/i18n/public/event-project-en';
import {publicSitemap} from '@/i18n/seo';
beforeEach(()=>{vi.spyOn(HTMLMediaElement.prototype,'play').mockResolvedValue(undefined);vi.spyOn(HTMLMediaElement.prototype,'pause').mockImplementation(()=>undefined);});
afterEach(()=>{cleanup();request.locale='en';vi.restoreAllMocks();vi.unstubAllGlobals();});
async function page(){return render(<LocaleProvider locale={request.locale}>{await Page()}</LocaleProvider>);}
describe('T22 event/project template',()=>{
  it.each(['en','ru'] as const)('keeps %s noindex outside sitemap',async locale=>{
    request.locale=locale;expect((await generateMetadata()).robots).toEqual({index:false,follow:false});
    expect(publicSitemap().some(p=>p.url.endsWith('/events/creative-sessions'))).toBe(false);
  });
  it("renders Russian content and localized links",async()=>{request.locale='ru';const{container}=await page();expect(container.querySelector('h1')).not.toBeNull();expect(container.querySelector('main')?.textContent).toMatch(/[А-Яа-яЁё]/);expect([...container.querySelectorAll('a[href]')].filter(link=>link.getAttribute('href')?.startsWith('/en/'))).toHaveLength(0);});
  it('owns one shell with the complete reference structure',async()=>{
    const{container}=await page();expect(screen.getAllByRole('heading',{level:1})).toHaveLength(1);
    expect(screen.getAllByRole('banner')).toHaveLength(1);expect(screen.getAllByRole('contentinfo')).toHaveLength(1);
    expect(container.querySelectorAll('[data-event-programme]')).toHaveLength(3);
    expect(container.querySelectorAll('[data-event-discussion]')).toHaveLength(4);
    expect(container.querySelectorAll('[data-event-feature]')).toHaveLength(3);
    expect(container.querySelectorAll('[data-event-journal]')).toHaveLength(1);
    expect(container.querySelectorAll('[data-ui=faq] details')).toHaveLength(5);
  });
  it('does not invent an announced event or registration',async()=>{
    const{container}=await page();expect(eventProject.status).toBe('concept');
    expect(eventProject.startsAt).toBeNull();expect(eventProject.endsAt).toBeNull();
    expect(eventProject.venue).toBeNull();expect(eventProject.registrationUrl).toBeNull();expect(eventProject.speakers).toEqual([]);
    expect(container.textContent).toContain(t.status);
    expect(container.textContent).not.toMatch(/official partners|250 million|built since 2010|June 22|Jun 24|spots are limited|tickets on sale/i);
    expect(container.querySelector('form,iframe,script[type="application/ld+json"],a[href*="luma.com"]')).toBeNull();
  });
  it('connects schedule cards to real in-page sections',async()=>{
    const{container}=await page();for(const a of container.querySelectorAll('[data-event-programme]')){
      const id=a.getAttribute('href')!;expect(id).toMatch(/^#/);expect(container.querySelector(id)).not.toBeNull();
    }
    expect(screen.getByRole('link',{name:t.hero.primary})).toHaveAttribute('href','#schedule');
    expect(container.querySelector('#project')).not.toBeNull();
  });
  it('loads the local trailer only after an explicit action',async()=>{
    const{container}=await page();expect(container.querySelector('video')).toBeNull();
    fireEvent.click(screen.getByRole('button',{name:t.film.action}));
    const dialog=screen.getByRole('dialog',{name:t.film.dialog});const video=dialog.querySelector('video')!;
    expect(video).toHaveAttribute('src','/assets/videos/event-project/candela-trailer.mp4');
    expect(video).toHaveAttribute('controls');expect(video).not.toHaveAttribute('autoplay');
    expect(dialog).toHaveTextContent(t.film.credit);
    fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));
    await waitFor(()=>expect(screen.queryByRole('dialog',{name:t.film.dialog})).toBeNull());
    expect(video.isConnected).toBe(false);
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
  });
  it('pauses the trailer when the document becomes hidden',async()=>{
    await page();fireEvent.click(screen.getByRole('button',{name:t.film.action}));
    vi.spyOn(document,'hidden','get').mockReturnValue(true);
    fireEvent(document,new Event('visibilitychange'));
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
  });
  it('shows honest project details and the real support contact',async()=>{
    await page();fireEvent.click(screen.getByRole('button',{name:t.hero.secondary}));
    const dialog=screen.getByRole('dialog',{name:t.details.title});expect(dialog).toHaveTextContent(t.details.description);
    expect(within(dialog).getByRole('link',{name:t.details.contact})).toHaveAttribute('href','mailto:support@epifera.com?subject=Creative%20Sessions');
    expect(dialog.querySelector('form,input')).toBeNull();
  });
  it('keeps the source explicit and local media same-origin',async()=>{
    const{container}=await page();expect([...container.querySelectorAll('img')].every(img=>new URL(img.getAttribute('src')!,location.origin).origin===location.origin)).toBe(true);
    fireEvent.click(screen.getByRole('button',{name:t.source.action}));
    expect(within(screen.getByRole('dialog',{name:t.source.title})).getByRole('link',{name:t.source.link})).toHaveAttribute('href','https://www.magnific.com/events/cannes-lions');
  });
  it('uses exclusive FAQ and internal reading/tool links',async()=>{
    const{container}=await page();expect([...container.querySelectorAll('[data-ui=faq] details')].every(e=>e.getAttribute('name')==='event-project-faq')).toBe(true);
    expect(container.querySelector('[data-event-journal]')).toHaveAttribute('href','/en/blog/ai-art-prompts');
    expect(screen.getByRole('link',{name:t.features[2].action})).toHaveAttribute('href','/en/workflows/product-campaign');
  });
  it('restores focus after native inert blurs the opening control',async()=>{
    const{container}=await page(),root=container.querySelector<HTMLElement>('[data-event-page]')!;let inert=false;
    Object.defineProperty(root,'inert',{configurable:true,get:()=>inert,set:(value:boolean)=>{inert=value;if(value&&root.contains(document.activeElement))(document.activeElement as HTMLElement).blur();}});
    const trigger=screen.getByRole('button',{name:t.hero.secondary});trigger.focus();fireEvent.click(trigger);
    expect(inert).toBe(true);fireEvent.keyDown(window,{key:'Escape'});fireEvent.animationEnd(screen.getByTestId('landing-dialog-backdrop'));
    await waitFor(()=>expect(trigger).toHaveFocus());expect(inert).toBe(false);
  });
  it('does not submit registrations, generations, payments or messages',async()=>{
    const fetchData=vi.fn();vi.stubGlobal('fetch',fetchData);await page();
    fireEvent.click(screen.getByRole('button',{name:t.closing.action}));expect(fetchData).not.toHaveBeenCalled();
    expect(localStorage.length).toBe(0);expect(location.search).toBe('');
  });
});
