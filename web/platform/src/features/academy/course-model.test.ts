import {describe,expect,it} from "vitest";
import {courseEn} from "@/i18n/public/course-en";
import {courseLessons,courseReadingMinutes,findCourseLesson,getCourseLessons,getCourseReadingMinutes,lessonReadingMinutes,searchCourseModules} from "./course-model";

describe("localized course catalogue",()=>{
  it("keeps the English catalogue and exact lesson lookup stable",()=>{
    expect(getCourseLessons("en")).toBe(courseLessons);
    expect(findCourseLesson(courseLessons[0].slug,"en")).toBe(courseLessons[0]);
    expect(findCourseLesson("../find-your-way-around","ru")).toBeUndefined();
    expect(getCourseReadingMinutes("en")).toBe(courseReadingMinutes);
  });
  it("projects lesson bodies and module titles using the requested locale",()=>{
    const lessons=getCourseLessons("ru"),first=lessons[0];
    expect(lessons.map(lesson=>lesson.slug)).toEqual(courseLessons.map(lesson=>lesson.slug));
    expect(first.title).toBe("Разберитесь в навигации");
    expect(first.moduleTitle).toBe("Навигация");
    expect(first.goal).toContain("рабочие пространства");
    expect(findCourseLesson(first.slug,"ru")).toBe(first);
    expect(courseLessons[0].title).toBe("Find your way around");
    expect(getCourseReadingMinutes("ru")).toBe(lessons.reduce((sum,lesson)=>sum+lessonReadingMinutes(lesson),0));
  });
  it("searches the localized module and lesson names",()=>{
    expect(searchCourseModules("  ВИЗУАЛЬНЫЙ БРИФ  ","ru").flatMap(module=>module.lessons.map(lesson=>lesson.slug))).toContain("write-a-visual-brief");
    expect(searchCourseModules("visual brief","en").flatMap(module=>module.lessons.map(lesson=>lesson.slug))).toContain("write-a-visual-brief");
    expect(searchCourseModules("визуальный","en")).toEqual([]);
    expect(searchCourseModules("unknown-lesson","ru")).toEqual([]);
  });
  it("keeps projections stable per course without reusing another course's lessons",()=>{
    const source={...courseEn,modules:courseEn.modules.map((module,index)=>index===0?{
      ...module,title:"A separate course module",
      lessons:module.lessons.map((lesson,lessonIndex)=>lessonIndex===0?{...lesson,title:"A separate course lesson"}:lesson),
    }:module)};
    const english=getCourseLessons("en",source);
    const russian=getCourseLessons("ru",source);
    expect(getCourseLessons("en",source)).toBe(english);
    expect(getCourseLessons("ru",source)).toBe(russian);
    expect(english).not.toBe(courseLessons);
    expect(russian).not.toBe(getCourseLessons("ru"));
    expect(english[0].title).toBe("A separate course lesson");
    expect(russian[0].moduleTitle).toBe("A separate course module");
    expect(courseLessons[0].title).toBe("Find your way around");
    expect(source.modules[0].lessons[0].title).toBe("A separate course lesson");
  });
});
