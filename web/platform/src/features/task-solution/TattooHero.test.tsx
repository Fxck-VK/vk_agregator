import {fireEvent,render,screen} from '@testing-library/react';
import {afterEach,describe,expect,it,vi} from 'vitest';
import {LocaleProvider} from '@/i18n/LocaleProvider';
import {clearImagePrompt,readImagePrompt} from '@/features/image-generation/image-prompt-draft';
const request=vi.hoisted(()=>({push:vi.fn()}));
vi.mock('@/i18n/navigation',()=>({useRouter:()=>({push:request.push})}));
import {TattooHero} from './TattooHero';

afterEach(()=>{clearImagePrompt(readImagePrompt());request.push.mockReset();vi.restoreAllMocks();});

describe('Russian tattoo brief handoff',()=>{
  it('stages localized editorial text only after submit, retaining raw input and editable presets',()=>{
    const fetchData=vi.spyOn(globalThis,'fetch');
    render(<LocaleProvider locale="ru"><TattooHero onPreview={vi.fn()}/></LocaleProvider>);
    fireEvent.click(screen.getByRole('button',{name:'Блэкворк'}));
    fireEvent.click(screen.getByRole('button',{name:'Предплечье'}));
    expect(screen.getByLabelText('Стиль татуировки')).toHaveValue('Блэкворк');
    expect(screen.getByLabelText('Где на теле?')).toHaveValue('Предплечье');
    expect(readImagePrompt()).toBe('');expect(request.push).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText('Ваша идея татуировки'),{target:{value:'Tattoo ideas: волк, 4K / id_123'}});
    fireEvent.change(screen.getByLabelText('Стиль татуировки'),{target:{value:'Fine line & Блэкворк'}});
    fireEvent.submit(screen.getByRole('form',{name:'Ваши данные для татуировки'}));
    expect(readImagePrompt()).toBe([
      'Концепция дизайна татуировки: Tattoo ideas: волк, 4K / id_123',
      'Стиль: Fine line & Блэкворк',
      'Предполагаемое размещение: Предплечье',
      'Чётко представьте дизайн как концепцию для обсуждения с тату-мастером.',
    ].join('\n'));
    expect(request.push).toHaveBeenCalledExactlyOnceWith('/app/image');
    expect(fetchData).not.toHaveBeenCalled();
  });
});
