import { notFound } from "@/i18n/server";
import { isLocalWorkspacePreviewEnabled, localWorkspacePreviewImageJobs, localWorkspacePreviewImageJobResults } from "@/features/session/local-workspace-preview";
import { AsyncStatePreview } from "@/features/local-development/AsyncStatePreview";

export default function UIStatesPage() {
  if (!isLocalWorkspacePreviewEnabled()) notFound();
  const job = localWorkspacePreviewImageJobs.items[0];
  const result = localWorkspacePreviewImageJobResults[job.id];
  return <AsyncStatePreview job={job} result={result} />;
}
