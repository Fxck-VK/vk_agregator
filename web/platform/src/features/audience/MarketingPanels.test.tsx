import {fireEvent,render,screen} from '@testing-library/react';
import {expect,it,vi} from 'vitest';
import {LocaleProvider} from '@/i18n/LocaleProvider';
import {marketingEn} from '@/i18n/public/marketing-en';
import {MarketingExamples} from './MarketingPanels';

it('keeps all examples visible in Russian and preserves the selected category across language changes',()=>{
 const onOpen=vi.fn();
 const {container,rerender}=render(<LocaleProvider locale="ru"><MarketingExamples onOpen={onOpen}/></LocaleProvider>);
 expect(container.querySelectorAll('[data-marketing-example]')).toHaveLength(marketingEn.workflows.examples.length);
 fireEvent.click(screen.getByRole('button',{name:'Все'}));
 expect(container.querySelectorAll('[data-marketing-example]')).toHaveLength(marketingEn.workflows.examples.length);
 const buttons=screen.getByRole('group').querySelectorAll('button');fireEvent.click(buttons[1]);
 const expected=marketingEn.workflows.examples.filter(item=>item.category===marketingEn.workflows.categories[1]).length;
 expect(container.querySelectorAll('[data-marketing-example]')).toHaveLength(expected);
 rerender(<LocaleProvider locale="en"><MarketingExamples onOpen={onOpen}/></LocaleProvider>);
 expect(container.querySelectorAll('[data-marketing-example]')).toHaveLength(expected);
 expect(screen.getByRole('button',{name:marketingEn.workflows.categories[1]})).toHaveAttribute('aria-pressed','true');
});
