import { GuestWorkspaceFrame } from "@/components/layout/WorkspaceFrame/WorkspaceFrame";
import { NotFoundContent } from "@/features/workspace/NotFoundContent/NotFoundContent";

// This fallback can be serialized alongside an existing public page. Keep account
// data in the missing-route layout, which runs only for an unmatched page URL.
export default function NotFound() {
  return <GuestWorkspaceFrame><NotFoundContent /></GuestWorkspaceFrame>;
}
