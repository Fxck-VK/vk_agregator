import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const route = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("../navigation", () => ({ usePathname: () => route.pathname }));

import { LocaleProvider } from "../LocaleProvider";
import { LocalizedPublicShell } from "./LocalizedPublicShell";

describe("LocalizedPublicShell", () => {
  it("lets only the known company page own its shell",()=>{route.pathname="/company/about-us";render(<LocalizedPublicShell><h1>About</h1></LocalizedPublicShell>);expect(screen.queryByRole("banner")).not.toBeInTheDocument();});
  it("lets the known customer story own its shell",()=>{route.pathname="/customers/football-campaign";render(<LocalizedPublicShell><h1>Case study</h1></LocalizedPublicShell>);expect(screen.queryByRole("banner")).not.toBeInTheDocument();});
  it("lets the business landing own its shell",()=>{route.pathname="/business";render(<LocalizedPublicShell><h1>Business</h1></LocalizedPublicShell>);expect(screen.queryByRole("banner")).not.toBeInTheDocument();});
  it("lets only the pricing landing own its reference shell",()=>{route.pathname="/pricing";render(<LocalizedPublicShell><h1>Pricing</h1></LocalizedPublicShell>);expect(screen.queryByRole("banner")).not.toBeInTheDocument();});
  it("lets only the integration landing own its reference shell",()=>{route.pathname="/mcp";render(<LocalizedPublicShell><h1>Integration</h1></LocalizedPublicShell>);expect(screen.queryByRole("banner")).not.toBeInTheDocument();});
  it("lets only the known material detail own its shell",()=>{route.pathname="/stock/items/dried-gypsophila/gypsophila-01";render(<LocalizedPublicShell><h1>Photo</h1></LocalizedPublicShell>);expect(screen.queryByRole("banner")).not.toBeInTheDocument();});
  it("lets only the known nature category own its reference shell",()=>{
    route.pathname="/photos/nature";render(<LocalizedPublicShell><h1>Nature</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the known public example own its matching reference shell",()=>{
    route.pathname="/inspiration/examples/aesthetic-flowers";render(<LocalizedPublicShell><h1>Example</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets only the known cinematic collection own its reference shell",()=>{
    route.pathname="/inspiration/cinematic-scenes";render(<LocalizedPublicShell><h1>Collection</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the product workflow own its reference shell",()=>{
    route.pathname="/workflows/product-campaign";render(<LocalizedPublicShell><h1>Workflow</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it.each(["/ai/docs","/ai/docs/account-management"])("lets %s own its documentation frame",pathname=>{
    route.pathname=pathname;render(<LocalizedPublicShell><h1>Help</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it.each(["/academy/courses/epifera-essentials","/academy/courses/epifera-essentials/find-your-way-around"])("lets %s own its learning frame",pathname=>{
    route.pathname=pathname;render(<LocalizedPublicShell><h1>Academy</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the article own its editorial shell", () => {
    route.pathname = "/blog/ai-art-prompts";
    render(<LocalizedPublicShell><h1>Art prompts</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the comparison page own its editorial shell", () => {
    route.pathname = "/compare/ai-avatar-models";
    render(<LocalizedPublicShell><h1>Avatar comparison</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the marketing audience page own its video hero and body", () => {
    route.pathname = "/ai/marketing";
    render(<LocalizedPublicShell><h1>Marketing tools</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the tattoo solution own its dark hero and light body", () => {
    route.pathname = "/ai/tattoo-generator";
    render(<LocalizedPublicShell><h1>Tattoo ideas</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the Kling version page own its reference chrome", () => {
    route.pathname = "/ai/kling/kling-3-0";
    render(<LocalizedPublicShell><h1>Kling 3.0</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the Kling family page own the matching reference chrome", () => {
    route.pathname = "/ai/kling";
    render(<LocalizedPublicShell><h1>Kling</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the tools overview own its matching reference chrome", () => {
    route.pathname = "/ai";
    render(<LocalizedPublicShell><h1>Tools</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it.each(["/legal", "/legal/privacy-policy"])("lets %s own its reference-specific legal shell", pathname => {
    route.pathname = pathname;
    render(<LocalizedPublicShell><h1>Legal page</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the generator landing own its reference-specific chrome", () => {
    route.pathname = "/ai/image-generator";
    render(<LocalizedPublicShell><h1>Image generator</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the event project own its reference chrome", () => {
    route.pathname = "/events/creative-sessions";
    render(<LocalizedPublicShell><h1>Creative Sessions</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the author archive own its reference chrome", () => {
    route.pathname = "/blog/author/epifera";
    render(<LocalizedPublicShell><h1>Epifera author</h1></LocalizedPublicShell>);
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("lets the homepage own its chrome instead of rendering duplicate navigation", () => {
    route.pathname = "/";
    render(<LocalizedPublicShell><h1>Homepage</h1></LocalizedPublicShell>);
    expect(screen.getByRole("heading", { name: "Homepage" })).toBeInTheDocument();
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("preserves shared localized chrome on other public routes", () => {
    route.pathname = "/guides";
    render(<LocaleProvider locale="en"><LocalizedPublicShell><h1>Guide</h1></LocalizedPublicShell></LocaleProvider>);
    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Guide" }));
    expect(screen.getByRole("link", { name: "Open platform" })).toHaveAttribute("href", "/ru/app");
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });
});
