import {describe,expect,it} from 'vitest';
import {composeTattooPrompt} from './tattoo-prompt';

describe('localized tattoo prompt',()=>{
  it('composes Russian editorial labels around untouched user text and localized choices',()=>{
    const prompt=composeTattooPrompt('  FLUX 2 Pro: wolf & moon\n4K / id_123  ','Блэкворк','Предплечье','ru');
    expect(prompt).toBe([
      'Концепция дизайна татуировки: FLUX 2 Pro: wolf & moon\n4K / id_123',
      'Стиль: Блэкворк',
      'Предполагаемое размещение: Предплечье',
      'Чётко представьте дизайн как концепцию для обсуждения с тату-мастером.',
    ].join('\n'));
  });
  it('preserves English defaults and never translates user-entered strings matching catalog text',()=>{
    expect(composeTattooPrompt('Tattoo ideas','Blackwork','Forearm')).toBe([
      'Tattoo design concept: Tattoo ideas','Style: Blackwork','Intended placement: Forearm',
      'Present the design clearly as a concept for discussion with a tattoo artist.',
    ].join('\n'));
    expect(composeTattooPrompt('Tattoo ideas','Fine line','Forearm','ru')).toContain('Концепция дизайна татуировки: Tattoo ideas\nСтиль: Fine line\nПредполагаемое размещение: Forearm');
  });
  it('keeps empty and bounded-input handling unchanged for either locale',()=>{
    for(const locale of ['en','ru']as const){
      expect(composeTattooPrompt('   ','style','placement',locale)).toBe('');
      const prompt=composeTattooPrompt('x'.repeat(5000),'y'.repeat(400),'z'.repeat(300),locale);
      expect(prompt).toContain('x'.repeat(4000));expect(prompt).not.toContain('x'.repeat(4001));
      expect(prompt).toContain('y'.repeat(240));expect(prompt).not.toContain('y'.repeat(241));
      expect(prompt).toContain('z'.repeat(160));expect(prompt).not.toContain('z'.repeat(161));
    }
  });
});
