import {fireEvent,render,screen,within} from "@testing-library/react";
import {describe,expect,it} from "vitest";
import {HelpCenterDrawer} from "./HelpCenterDrawer";
describe('Help center drawer focus',()=>{
  it('does not focus a nested summary hidden by a collapsed outer group',()=>{
    render(<HelpCenterDrawer id="test-nested" title="Nested contents" closeLabel="Close" testId="nested-backdrop" onClose={()=>{}}>{()=><><input type="search" aria-label="Search"/><details><summary>Closed outer group</summary><details><summary>Hidden nested group</summary><a href="#hidden">Hidden lesson</a></details></details></>}</HelpCenterDrawer>);
    const dialog=screen.getByRole('dialog',{name:'Nested contents'}),close=within(dialog).getByRole('button',{name:'Close'}),outer=within(dialog).getByText('Closed outer group');
    fireEvent.keyDown(close,{key:'Tab',shiftKey:true});expect(outer).toHaveFocus();fireEvent.keyDown(outer,{key:'Tab'});expect(close).toHaveFocus();
  });
  it('includes search inputs and excludes descendants of collapsed outer groups',()=>{
    render(<HelpCenterDrawer id="test-help" title="Contents" closeLabel="Close" testId="test-help-backdrop" onClose={()=>{}}>{()=><><details><summary>Closed outer group</summary><details><summary>Hidden nested group</summary><a href="#hidden">Hidden lesson</a></details></details><input type="search" aria-label="Find a lesson"/></>}</HelpCenterDrawer>);
    const dialog=screen.getByRole('dialog',{name:'Contents'}),close=within(dialog).getByRole('button',{name:'Close'}),input=within(dialog).getByRole('searchbox',{name:'Find a lesson'});
    fireEvent.keyDown(close,{key:'Tab',shiftKey:true});expect(input).toHaveFocus();fireEvent.keyDown(input,{key:'Tab'});expect(close).toHaveFocus();
  });
});
