import {fireEvent,render,screen,within} from '@testing-library/react';
import {describe,expect,it,vi} from 'vitest';
import {LocaleProvider} from '@/i18n/LocaleProvider';
import {localizePublicData} from '@/i18n/public/public-copy';
import type {AuthorMaterial} from '@/i18n/public/author-en';
import {AuthorSearch} from './AuthorSearch';
import {searchAuthorMaterials} from './author-model';

const source:readonly AuthorMaterial[]=[{
  id:'tattoo-guide',title:'Tattoo ideas',category:'Lessons',href:'/ai/tattoo-generator',
  cover:{src:null,width:100,height:100},date:null,
},{
  id:'model-guide',title:'Choose a model',category:'Guides',href:'/app/models',
  cover:{src:null,width:100,height:100},date:null,
}];

describe('localized author search',()=>{
  it('matches localized Russian titles and categories while preserving record identity and links',()=>{
    const items=localizePublicData(source,'ru');
    expect(items.map(item=>[item.id,item.href])).toEqual(source.map(item=>[item.id,item.href]));
    expect(searchAuthorMaterials(items,'  ТАТУИРОВОК  ','ru')).toEqual([items[0]]);
    expect(searchAuthorMaterials(items,'УРОКИ','ru')[0]).toBe(items[0]);
    expect(source[0].title).toBe('Tattoo ideas');
    const onClose=vi.fn();
    render(<LocaleProvider locale="ru"><AuthorSearch items={items} onClose={onClose}/></LocaleProvider>);
    const dialog=screen.getByRole('dialog',{name:'Поиск в редакционной библиотеке'});
    const input=within(dialog).getByRole('searchbox',{name:'Поиск статей, уроков и руководств'});
    fireEvent.change(input,{target:{value:'ТАТУИРОВОК'}});
    const link=within(dialog).getByRole('link',{name:'Уроки Идеи татуировок'});
    expect(link).toHaveAttribute('href','/ru/ai/tattoo-generator');
    expect(within(dialog).getAllByRole('link')).toHaveLength(1);
    expect(within(dialog).getByRole('status')).toHaveTextContent('1 материал');
    link.addEventListener('click',event=>event.preventDefault(),{once:true});
    fireEvent.click(link);expect(onClose).toHaveBeenCalledOnce();
    fireEvent.change(input,{target:{value:'нет совпадения'}});
    expect(dialog).toHaveTextContent('Материалы не найдены. Попробуйте другое слово.');
    fireEvent.click(within(dialog).getByRole('button',{name:'Очистить поиск'}));
    expect(input).toHaveValue('');expect(input).toHaveFocus();
    expect(within(dialog).getAllByRole('link')).toHaveLength(2);
  });
});
