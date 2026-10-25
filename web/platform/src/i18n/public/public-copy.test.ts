import {describe,expect,it} from 'vitest';
import {localizePublicData,translatePublicText,translatePublicMessage} from './public-copy';

describe('locale-scoped public copy',()=>{
  it('returns the exact English object without rewriting it',()=>{
    const source={title:'Search',href:'/app',nested:[{title:'Close'}]};
    expect(localizePublicData(source,'en')).toBe(source);
  });
  it('localizes nested text while preserving structural strings and numbers',()=>{
    const source={title:'Search',id:'Search',icon:'image',tone:'purple',href:'/app',src:'/assets/a.webp',price:549,status:'concept',value:'Monthly',items:[{label:'Close'}]};
    const ru=localizePublicData(source,'ru');
    expect(ru.title).toBe('Поиск');expect(ru.items[0].label).toBe('Закрыть');
    expect(ru).toMatchObject({id:'Search',icon:'image',tone:'purple',href:'/app',src:'/assets/a.webp',price:549,status:'concept',value:'Monthly'});
    expect(source.title).toBe('Search');
  });
  it('does not leak request language between repeated RU and EN projections',()=>{
    const source={title:'Search'};
    expect(localizePublicData(source,'ru').title).toBe('Поиск');
    expect(localizePublicData(source,'en').title).toBe('Search');
    expect(localizePublicData(source,'ru')).toBe(localizePublicData(source,'ru'));
  });
  it('preserves unknown text, model names, asset paths and runtime values',()=>{
    for(const text of ['Nano Banana Pro','/assets/picture.webp','model_id','https://epifera.com','Unknown catalogue label'])expect(translatePublicText(text,'ru')).toBe(text);
  });
  it('distinguishes editorial labels from canonical media taxonomy and metadata',()=>{
    const source={format:'Text + practice',support:'Contact support',status:'Team plans in preparation',
      media:{id:'flower',src:'/assets/flower.webp',tags:['flower','nature'],format:'webp',width:512},
      taxonIds:['flower'],references:{support:'supported',maxCount:3},value:'Not connected'};
    const ru=localizePublicData(source,'ru');
    expect(ru.format).toBe('Текст и практика');expect(ru.support).not.toBe(source.support);
    expect(ru.status).not.toBe(source.status);expect(ru.media).toEqual(source.media);
    expect(ru.taxonIds).toEqual(source.taxonIds);expect(ru.references).toEqual(source.references);
    expect(ru.value).toBe('Not connected');
  });
  it('formats translated editorial messages without translating parameter values',()=>{
    expect(translatePublicMessage('Step {index}: {title}','ru',{index:2,title:'My RAW prompt'})).toBe('Шаг 2: My RAW prompt');
  });
});
