import {render,screen} from "@testing-library/react";
import {describe,expect,it} from "vitest";
import {LocaleProvider} from "@/i18n/LocaleProvider";
import {courseLessons} from "./course-model";
import {AcademyCourse} from "./AcademyCourse";

describe("localized lesson props",()=>{
  it("localizes an incoming English lesson and its pager without changing destinations",()=>{
    render(<LocaleProvider locale="ru"><AcademyCourse lesson={courseLessons[1]}/></LocaleProvider>);
    expect(screen.getByRole("heading",{level:1})).toHaveTextContent("Выберите модель для задачи");
    expect(screen.getByRole("heading",{name:"Попробуйте сами"})).toBeInTheDocument();
    expect(screen.getByRole("link",{name:"Предыдущий урок"})).toHaveTextContent("Разберитесь в навигации");
    expect(screen.getByRole("link",{name:"Следующий урок"})).toHaveAttribute("href",`/ru/academy/courses/epifera-essentials/${courseLessons[2].slug}`);
    expect(screen.queryByText(courseLessons[1].goal)).not.toBeInTheDocument();
  });
});
