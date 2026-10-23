import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { FileCard } from "./FileCard/FileCard";
import { ArtifactDeleteButton } from "./ArtifactDeleteButton";
import { ConversationAssistantMessage, ConversationImageGallery } from "@/features/conversations/ConversationImageGallery/ConversationImageGallery";
import { WorkspaceDataCacheProvider, createWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { webBrowserMutation } from "@/lib/web-api/browser";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn(), webBrowserFetch: vi.fn() }));
afterEach(() => vi.clearAllMocks());
const id = "6ca96a58-a902-4f23-a92a-6726e1a0cd20";
const job: ImageJob = { id:"d7c979f5-24e5-4f88-924b-a592d6e5a906", status:"succeeded", cost_estimate:10, created_at:"2026-10-23T00:00:00Z", updated_at:"2026-10-23T00:00:00Z", prompt:"test output", model_id:"test", model_name:"Test", image_quality:"1K" };
const result: ImageJobResult = { job_id:job.id, status:"succeeded", artifacts:[{ id,mime_type:"image/png",size_bytes:42,width:100,height:100 }] };

it("confirms before deleting, removes the file card and replaces the same chat image with a tombstone", async () => {
 vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null,{status:202}));
 const image={job,artifact:result.artifacts[0]};
 const message={id:"80000000-0000-4000-8000-000000000001",seq:1,role:"assistant" as const,text:"Done",rating:null,created_at:job.created_at,images:[image]};
 render(<WorkspaceDataCacheProvider>
  <FileCard job={job} result={result} resultState="idle" isRetrying={false} onOpenPreview={vi.fn()} onRequestResult={vi.fn()} onRetryJob={vi.fn()} />
  <ConversationImageGallery conversationID="test" hasMoreBefore={false} messages={[message]}><ConversationAssistantMessage message={message} /></ConversationImageGallery>
 </WorkspaceDataCacheProvider>);
 fireEvent.click(screen.getByRole("button",{name:"Удалить файл"}));
 const dialog=screen.getByRole("dialog",{name:"Удалить файл?"});
 expect(dialog).toHaveTextContent("из «Моих файлов» и диалога");
 expect(webBrowserMutation).not.toHaveBeenCalled();
 fireEvent.click(within(dialog).getByRole("button",{name:"Удалить"}));
 await waitFor(()=>expect(screen.queryByRole("link",{name:/Скачать/})).not.toBeInTheDocument());
 expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/artifacts/${id}`,expect.objectContaining({method:"DELETE"}));
 expect(screen.getByText("Файл удалён")).toBeInTheDocument();
 expect(screen.getByText("Done")).toBeInTheDocument();
});

it("keeps tombstones across stale reads but isolates another account cache", () => {
 const cache=createWorkspaceDataCache();
 cache.setImageResult(result);
 cache.markArtifactDeleted(id);
 cache.setImageResult(result);
 expect(cache.getImageResults()[job.id].artifacts[0].deleted).toBe(true);
 expect(createWorkspaceDataCache().getDeletedArtifactIDs()).toEqual([]);
});

it("retains files on a busy response and lets the user retry the same deletion", async () => {
 vi.mocked(webBrowserMutation).mockResolvedValueOnce(new Response(null,{status:409})).mockResolvedValueOnce(new Response(null,{status:202}));
 render(<WorkspaceDataCacheProvider><FileCard job={job} result={result} resultState="idle" isRetrying={false} onOpenPreview={vi.fn()} onRequestResult={vi.fn()} onRetryJob={vi.fn()} /></WorkspaceDataCacheProvider>);
 fireEvent.click(screen.getByRole("button",{name:"Удалить файл"}));
 fireEvent.click(within(screen.getByRole("dialog")).getByRole("button",{name:"Удалить"}));
 expect(await screen.findByText(/Файл используется незавершённым заданием/)).toBeInTheDocument();
 expect(screen.getByRole("link",{name:/Скачать/})).toBeInTheDocument();
 fireEvent.click(within(screen.getByRole("dialog")).getByRole("button",{name:"Удалить"}));
 await waitFor(()=>expect(screen.queryByRole("link",{name:/Скачать/})).not.toBeInTheDocument());
 expect(webBrowserMutation).toHaveBeenCalledTimes(2);
});

it("deletes only the chosen artifact in a multi-image generation", async () => {
 vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null,{status:202}));
 const second = { ...result.artifacts[0], id:"6ca96a58-a902-4f23-a92a-6726e1a0cd21" };
 render(<WorkspaceDataCacheProvider><FileCard job={job} result={{...result,artifacts:[...result.artifacts,second]}} resultState="idle" isRetrying={false} onOpenPreview={vi.fn()} onRequestResult={vi.fn()} onRetryJob={vi.fn()} /></WorkspaceDataCacheProvider>);
 fireEvent.click(screen.getAllByRole("button",{name:"Удалить файл"})[0]);
 fireEvent.click(within(screen.getByRole("dialog")).getByRole("button",{name:"Удалить"}));
 await waitFor(()=>expect(screen.getAllByRole("link",{name:/Скачать/})).toHaveLength(1));
 expect(screen.getByRole("link",{name:/Скачать/})).toHaveAttribute("href",`/web/v1/image-artifacts/${second.id}`);
});

it("deletes related track formats before the main file and deduplicates their IDs", async () => {
 vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null,{status:202}));
 const related="6ca96a58-a902-4f23-a92a-6726e1a0cd22";
 render(<WorkspaceDataCacheProvider><ArtifactDeleteButton artifactID={id} relatedArtifactIDs={[related,related,id]} /></WorkspaceDataCacheProvider>);
 fireEvent.click(screen.getByRole("button",{name:"Удалить файл"}));
 fireEvent.click(within(screen.getByRole("dialog")).getByRole("button",{name:"Удалить"}));
 await waitFor(()=>expect(webBrowserMutation).toHaveBeenCalledTimes(2));
 expect(vi.mocked(webBrowserMutation).mock.calls.map(call=>call[0])).toEqual([`/web/v1/artifacts/${related}`,`/web/v1/artifacts/${id}`]);
});
