"use client";

import { useEffect, useRef, useState } from "react";
import { useMessages } from "@/i18n/LocaleProvider";
import { z } from "zod";
import { Button } from "@/components/ui/Button/Button";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import type { GenerationOptions } from "@/features/models/generation-options-contract";
import { webBrowserMutation } from "@/lib/web-api/browser";
import styles from "./motion-video.module.css";

const uploadedVideo = z.object({artifact_id:z.string().uuid(),mime_type:z.enum(["video/mp4","video/quicktime"]),size_bytes:z.number().int().positive().max(48*1024*1024),duration_ms:z.number().int().min(3000).max(30000),duration_sec:z.number().int().min(3).max(30)}).strict().refine(value=>Math.ceil(value.duration_ms/1000)===value.duration_sec);
type VideoState = {modelID:string; name?:string; url?:string; artifactID?:string; durationMS?:number; loading?:boolean; progress?:number|null; error?:string};

// Files stay in memory; only owned artifact IDs enter the existing pending intent.
export function useMotionVideo(model: GenerationModel | undefined, options: GenerationOptions, disabled: boolean) {
 const msg = useMessages();
 const modelID = model?.category === "video" && model.motion_control ? model.id : "";
 const [state,setState] = useState<VideoState>({modelID});
 if (state.modelID !== modelID) setState({modelID});
 const current = state.modelID === modelID ? state : {modelID};
 const input = useRef<HTMLInputElement>(null);
 const upload = useRef<AbortController|null>(null);
 useEffect(()=>()=>{ upload.current?.abort(); },[modelID]);
 useEffect(()=>()=>{ if (state.url) URL.revokeObjectURL(state.url); },[state.url]);
 const clear = () => { upload.current?.abort(); setState({modelID}); };
 const choose = async (file:File) => {
  upload.current?.abort();
  const typeOK = ["video/mp4","video/quicktime"].includes(file.type) || file.type === "" && /\.(mp4|mov)$/i.test(file.name);
  if (!typeOK || file.size <= 0 || file.size > 48*1024*1024) { setState({modelID,error:msg("motion.fileRequirement")}); return; }
  const controller = new AbortController(); upload.current = controller;
  const next = {modelID,name:file.name,url:URL.createObjectURL(file)};
  setState({...next,loading:true,progress:0});
  const form = new FormData(); form.append("file",file);
  try {
   const response = await webBrowserMutation("/web/v1/motion-video-input",{method:"POST",body:form,signal:controller.signal,onUploadProgress:progress=>{if (!controller.signal.aborted) setState(value=>({...value,progress}));}});
   if (!response.ok) throw new Error(response.status === 400 ? "invalid" : "upload");
   const data = uploadedVideo.parse(await response.json());
   if (!controller.signal.aborted) setState({...next,artifactID:data.artifact_id,durationMS:data.duration_ms});
  } catch (error) {
   if (!controller.signal.aborted) setState({...next,error:error instanceof Error && error.message === "invalid" ? msg("motion.invalidVideo") : msg("motion.uploadFailed")});
  }
 };
 const maximum = options.character_orientation === "video" ? 30 : 10;
 const tooLong = current.durationMS !== undefined && current.durationMS > maximum*1000;
 const blocked = !!modelID && (!current.artifactID || !!current.loading || !!current.error || tooLong);
 const videoOptions = modelID && !blocked ? {reference_video_artifact_id:current.artifactID} : {};
 const controls = !modelID ? null : <div className={styles.panel}>
  <p className={styles.hint}>{msg("motion.instructions",{seconds:maximum})}</p>
  <div className={styles.actions}>
   <input ref={input} type="file" hidden accept="video/mp4,video/quicktime,.mp4,.mov" aria-label={msg("motion.inputLabel")} disabled={disabled} onChange={event=>{ const file=event.target.files?.[0]; event.target.value=""; if (file) void choose(file); }}/>
   <Button variant="outline" disabled={disabled} onClick={()=>input.current?.click()}>{current.name ? msg("motion.replaceVideo") : msg("motion.uploadVideo")}</Button>
   {current.name ? <Button variant="outline" disabled={disabled} onClick={clear}>{msg("motion.removeVideo")}</Button> : null}
   {current.durationMS ? <span>{msg("motion.duration",{seconds:(current.durationMS/1000).toLocaleString(msg.locale,{maximumFractionDigits:2})})}</span> : null}
  </div>
  {current.url ? <video className={styles.preview} src={current.url} controls preload="metadata" aria-label={msg("motion.preview")}/> : null}
  {current.loading ? <p role="status">{current.progress === 100 ? msg("motion.validating") : msg("motion.uploadProgress",{progress:Math.round(current.progress ?? 0)})}</p> : null}
  {current.error || tooLong ? <StateNotice inline kind="error">{current.error ?? msg("motion.tooLong",{seconds:maximum})}</StateNotice> : null}
 </div>;
 return {blocked,options:videoOptions,controls,clear};
}
