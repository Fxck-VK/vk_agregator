import {describe,expect,it} from "vitest";
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
});
