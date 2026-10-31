import {describe,expect,it} from 'vitest';
import {templateDefinitions} from './registry';

describe('24 shared public templates',()=>{
  it('covers every agreed template exactly once',()=>{
    expect(templateDefinitions.map(t=>t.id)).toEqual(Array.from({length:24},(_,i)=>`T${String(i+1).padStart(2,'0')}`));
    expect(new Set(templateDefinitions.map(t=>t.path)).size).toBe(24);
  });
  it('keeps billing and legal sources managed',()=>{
    expect(templateDefinitions.filter(t=>t.managed).map(t=>t.id)).toEqual(['T18','T23']);
  });
});
