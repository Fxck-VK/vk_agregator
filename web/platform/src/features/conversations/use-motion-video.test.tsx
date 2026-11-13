import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { useMotionVideo } from "./use-motion-video";
vi.mock("@/lib/web-api/browser",()=>({webBrowserMutation:vi.fn()}));
const model={id:"video_kling_2_6_motion_control",name:"Motion",category:"video",motion_control:true} as GenerationModel;
const id="00000000-0000-4000-8000-000000000001";
function Harness({orientation="image",active=true}:{orientation?:"image"|"video";active?:boolean}) {
 const video=useMotionVideo(active?model:undefined,{character_orientation:orientation},false);
 return <>{video.controls}<button disabled={video.blocked}>Отправить</button><output>{video.options.reference_video_artifact_id ?? ""}</output></>;
}
function select(type="video/mp4",name="ref.mp4") {fireEvent.change(screen.getByLabelText("Видеореференс Motion Control"),{target:{files:[new File(["synthetic"],name,{type})]}});}
function result(ms=5042) {return Response.json({artifact_id:id,mime_type:"video/mp4",size_bytes:9,duration_ms:ms,duration_sec:Math.ceil(ms/1000)});}
beforeEach(()=>{vi.stubGlobal("URL",Object.assign(URL,{createObjectURL:vi.fn(()=>"blob:synthetic"),revokeObjectURL:vi.fn()}));});
afterEach(()=>{cleanup();vi.resetAllMocks();vi.unstubAllGlobals();});
it("blocks submission until server validation, previews the file and clears it",async()=>{
 vi.mocked(webBrowserMutation).mockResolvedValue(result());render(<Harness/>);
 expect(screen.getByRole("button",{name:"Отправить"})).toBeDisabled();select();
 await waitFor(()=>expect(screen.getByRole("button",{name:"Отправить"})).toBeEnabled());
 expect(screen.getByLabelText("Просмотр видеореференса")).toHaveAttribute("src","blob:synthetic");
 expect(screen.getByText("Длительность: 5,04 с")).toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"Убрать видео"}));
 expect(screen.getByRole("button",{name:"Отправить"})).toBeDisabled();
 expect(screen.queryByLabelText("Просмотр видеореференса")).toBeNull();
});
it("enforces the orientation limit from server metadata",async()=>{
 vi.mocked(webBrowserMutation).mockResolvedValue(result(10001));const view=render(<Harness/>);select();
 await screen.findByText("Длительность: 10 с");expect(screen.getByRole("button",{name:"Отправить"})).toBeDisabled();
 view.rerender(<Harness orientation="video"/>);expect(screen.getByRole("button",{name:"Отправить"})).toBeEnabled();
 view.rerender(<Harness/>);expect(screen.getByRole("button",{name:"Отправить"})).toBeDisabled();
});
it("discards late uploads after removing or changing the model",async()=>{
 let finish!:(value:Response)=>void;vi.mocked(webBrowserMutation).mockReturnValue(new Promise(resolve=>{finish=resolve;}));
 const view=render(<Harness/>);select();const signal=vi.mocked(webBrowserMutation).mock.calls[0][1].signal;
 view.rerender(<Harness active={false}/>);expect(signal?.aborted).toBe(true);
 await act(async()=>finish(result()));view.rerender(<Harness/>);
 expect(screen.getByRole("button",{name:"Отправить"})).toBeDisabled();expect(screen.queryByLabelText("Просмотр видеореференса")).toBeNull();
});
it("rejects mismatched MIME locally and fails closed on invalid server metadata",async()=>{
 vi.mocked(webBrowserMutation).mockResolvedValue(result(1));render(<Harness/>);select("image/png","ref.mp4");expect(webBrowserMutation).not.toHaveBeenCalled();
 select();await screen.findByText("Не удалось загрузить и проверить видео. Выберите файл повторно.");expect(screen.getByRole("button",{name:"Отправить"})).toBeDisabled();
});
