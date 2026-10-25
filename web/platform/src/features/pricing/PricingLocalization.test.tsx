import {fireEvent,render,screen,waitFor,within} from "@testing-library/react";
import {describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {PersonalPlans} from "./PricingPlans";
import {PricingComparison} from "./PricingComparison";
import {PricingInformation} from "./PricingDialogs";

describe("pricing display locale",()=>{
  it("localizes annual periods, currency grouping and dynamic preview action labels",()=>{
    const onPlan=vi.fn();
    render(<LocaleProvider locale="ru"><PersonalPlans period="yearly" capacity="ultima" onCapacity={()=>{}} onPlan={onPlan} onUnits={()=>{}}/></LocaleProvider>);
    fireEvent.click(screen.getByRole('button',{name:'Посмотреть предварительный вариант Start+'}));
    expect(onPlan).toHaveBeenCalledWith('start');
    expect(screen.getAllByText('/год')).toHaveLength(3);
    expect(screen.getByText(/6\s588/)).toBeInTheDocument();
    expect(screen.queryByText('/year')).not.toBeInTheDocument();
  });
  it("localizes comparison control names and team-only values",()=>{
    const props={period:'monthly' as const,onPeriod:()=>{},selected:['start'] as const,onSelected:()=>{},onPlan:()=>{}};
    const {rerender}=render(<LocaleProvider locale="ru"><PricingComparison {...props} teams={false}/></LocaleProvider>);
    expect(screen.getByRole('button',{name:'Убрать Start+ из сравнения'})).toBeDisabled();
    expect(screen.getByRole('button',{name:'Посмотреть сведения о Start+'})).toBeInTheDocument();
    expect(screen.getByRole('switch')).not.toHaveAccessibleName(/in comparison/);
    expect(screen.getByText(/\/месяц/)).toBeInTheDocument();
    rerender(<LocaleProvider locale="ru"><PricingComparison {...props} teams/></LocaleProvider>);
    expect(screen.getAllByText('Требует обсуждения').length).toBeGreaterThan(0);
    expect(screen.queryByText('To be discussed')).not.toBeInTheDocument();
  });
  it("copies the displayed Russian editorial brief only after an explicit action",async()=>{
    const writeText=vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText}});
    render(<LocaleProvider locale="ru"><PricingInformation dialog={{kind:'quote',seats:20,enterprise:false}} period="monthly" onClose={()=>{}}/></LocaleProvider>);
    const dialog=screen.getByRole('dialog'),quote=dialog.querySelector('pre')!.textContent!;
    expect(quote).toContain('Размер команды: 20 чел.');
    expect(quote).not.toMatch(/Team|people|Price|Please|requirements/);
    expect(within(dialog).queryByText('Team plans in preparation')).not.toBeInTheDocument();
    expect(writeText).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole('button',{name:'Скопировать бриф запроса'}));
    await waitFor(()=>expect(writeText).toHaveBeenCalledWith(quote));
  });
});
