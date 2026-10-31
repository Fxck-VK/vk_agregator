import {describe,expect,it} from 'vitest';
import {render,screen} from '@testing-library/react';
import {TemplateDataProvider,bindTemplateSources} from './TemplateDataProvider';
import {usePublicCopy} from '@/i18n/public/use-public-copy';
import {LocaleProvider} from '@/i18n/LocaleProvider';

const original={copy:{title:'Original',items:['one']},unrelated:{title:'Keep'}};
function View(){const {copy,unrelated}=usePublicCopy(original,{copy:'article'});return <><h1>{copy.title}</h1><p>{copy.items.join(',')}</p><aside>{unrelated.title}</aside></>;}
describe('named public source binding',()=>{
  it('binds the chosen source without rewriting unrelated text or mutating defaults',()=>{
    expect(bindTemplateSources(original,{copy:'article'},{article:{title:'Новая статья',items:['два','три']}})).toEqual({copy:{title:'Новая статья',items:['два','три']},unrelated:{title:'Keep'}});
    expect(original.copy.title).toBe('Original');
  });
  it('isolates two instances of the same template and binds descendants',()=>{
    render(<LocaleProvider locale="ru"><TemplateDataProvider data={{article:{title:'Первая',items:['А']}}}><View/></TemplateDataProvider><TemplateDataProvider data={{article:{title:'Вторая',items:['Б']}}}><View/></TemplateDataProvider></LocaleProvider>);
    expect(screen.getByRole('heading',{name:'Первая'})).toBeInTheDocument();expect(screen.getByRole('heading',{name:'Вторая'})).toBeInTheDocument();
  });
});
