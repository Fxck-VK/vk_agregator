import {fireEvent,render,screen} from "@testing-library/react";
import {describe,expect,it,vi} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {LandingHeader} from "./LandingHeader";
const labels={openMenu:'Open navigation',closeMenu:'Close navigation',navigation:'Main navigation',search:'Search',login:'Log in',signup:'Sign up'};
describe('LandingHeader variants',()=>{
  it('preserves the overlay login/signup defaults',()=>{render(<LocaleProvider locale="en"><LandingHeader navigation={[]} labels={labels} onSearch={()=>{}}/></LocaleProvider>);expect(screen.getByRole('link',{name:'Log in'})).toHaveAttribute('href','/ru/login');expect(screen.getByRole('link',{name:'Sign up'})).toHaveAttribute('href','/ru/login?mode=register');expect(screen.getByRole('banner')).not.toHaveAttribute('data-variant','editorial');});
  it('opts into editorial chrome with a single site action and the same working search',()=>{const search=vi.fn();render(<LocaleProvider locale="en"><LandingHeader variant="editorial" siteAction={{label:'Go to Epifera',href:'/'}} navigation={[]} labels={labels} onSearch={search}/></LocaleProvider>);expect(screen.getByRole('banner')).toHaveAttribute('data-variant','editorial');expect(screen.getByRole('link',{name:'Go to Epifera'})).toHaveAttribute('href','/ru');expect(screen.queryByRole('link',{name:'Log in'})).not.toBeInTheDocument();fireEvent.click(screen.getByRole('button',{name:'Search'}));expect(search).toHaveBeenCalledOnce();});
});
