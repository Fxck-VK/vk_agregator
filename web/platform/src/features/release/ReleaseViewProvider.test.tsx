import { act, render, screen } from "@testing-library/react";
import { useEffect } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ refresh }) }));
const refresh = vi.fn();
import { ReleaseViewProvider, useReleaseView } from "./ReleaseViewProvider";
let current: ReturnType<typeof useReleaseView>;
function Reader() { const value = useReleaseView(); useEffect(() => { current = value; }, [value]); return <span>{value.view}</span>; }
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks(); });
describe("confirmed release view", () => {
  it("has safe defaults without a provider", async () => {
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    render(<Reader />);
    expect(current.view).toBe("prod"); expect(current.enabled("music")).toBe(false);
    expect(await current.changeView("dev")).toBe(false); expect(fetcher).not.toHaveBeenCalled();
  });
  it("coalesces repeated changes and updates only after server confirmation", async () => {
    let finish!: (response: Response) => void;
    const fetcher = vi.fn(() => new Promise<Response>(resolve => { finish = resolve; })); vi.stubGlobal("fetch", fetcher);
    render(<ReleaseViewProvider initial={{ view: "dev", canSwitch: true, localSimulation: true }}><Reader /></ReleaseViewProvider>);
    let first!: Promise<boolean>, second!: Promise<boolean>;
    act(() => { first = current.changeView("prod"); second = current.changeView("prod"); });
    expect(fetcher).toHaveBeenCalledTimes(1); expect(screen.getByText("dev")).toBeInTheDocument();
    await act(async () => { finish(Response.json({ view: "prod" })); await Promise.all([first, second]); });
    expect(screen.getByText("prod")).toBeInTheDocument(); expect(refresh).toHaveBeenCalledOnce();
    expect(fetcher.mock.calls[0]).toEqual(["/web/release-view", expect.objectContaining({ body: '{"view":"prod"}', credentials: "same-origin" })]);
  });
  it("keeps the confirmed profile on a failed switch", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(null, { status: 403 })));
    render(<ReleaseViewProvider initial={{ view: "prod", canSwitch: true, localSimulation: false }}><Reader /></ReleaseViewProvider>);
    await act(async () => { expect(await current.changeView("dev")).toBe(false); });
    expect(current.view).toBe("prod"); expect(current.error).toBe(true); expect(refresh).not.toHaveBeenCalled();
  });
});
