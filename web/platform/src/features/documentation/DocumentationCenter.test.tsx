import {fireEvent,render,screen,within} from "@testing-library/react";
import {describe,expect,it} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {documentationArticles} from "@/i18n/public/documentation-en";
import {DocumentationCenter} from "./DocumentationCenter";

describe("localized help contents",()=>{
  it("localizes an incoming English article and searches Russian body text",()=>{
    const article=documentationArticles.find(item=>item.slug==="account-management")!;
    const {rerender}=render(<LocaleProvider locale="ru"><DocumentationCenter article={article}/></LocaleProvider>);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent("Управление аккаунтом");
    expect(screen.getByRole("heading",{name:"Основная и резервная электронная почта"})).toBeInTheDocument();
    const search=screen.getByRole("searchbox");
    fireEvent.change(search,{target:{value:"РЕЗЕРВНЫЙ АДРЕС"}});
    const navigation=search.closest("nav")!;
    expect(within(navigation).getByRole("link",{name:"Управление аккаунтом"})).toHaveAttribute("href","/ru/ai/docs/account-management");
    expect(within(navigation).queryByText("Account management")).not.toBeInTheDocument();
    rerender(<LocaleProvider locale="en"><DocumentationCenter article={article}/></LocaleProvider>);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent("Account management");
    fireEvent.change(screen.getByRole("searchbox"),{target:{value:"BACKUP EMAIL"}});
    expect(within(screen.getByRole("searchbox").closest("nav")!).getByRole("link",{name:"Account management"})).toHaveAttribute("href","/ru/ai/docs/account-management");
  });
});
