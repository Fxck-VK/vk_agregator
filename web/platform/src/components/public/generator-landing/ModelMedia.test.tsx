import {render,screen} from '@testing-library/react';
import {expect,it} from 'vitest';
import {LocaleProvider} from '@/i18n/LocaleProvider';
import {ModelGallery,ModelGalleryDialog} from './ModelMedia';

it('resolves default gallery and dialog titles after selecting the active locale',()=>{
 const {rerender}=render(<LocaleProvider locale="ru"><ModelGallery items={[]} paused onOpen={()=>{}}/></LocaleProvider>);
 expect(screen.getByRole('heading',{name:'Галерея референсов Kling'})).toBeInTheDocument();
 rerender(<LocaleProvider locale="en"><ModelGallery items={[]} paused onOpen={()=>{}}/></LocaleProvider>);
 expect(screen.getByRole('heading',{name:'Kling reference gallery'})).toBeInTheDocument();
 rerender(<LocaleProvider locale="ru"><ModelGalleryDialog item={{file:'example.webp',alt:'Example',model:'Kling 3.0'}} onClose={()=>{}}/></LocaleProvider>);
 expect(screen.getByRole('dialog',{name:'Пример из референса Kling'})).toBeInTheDocument();
});
