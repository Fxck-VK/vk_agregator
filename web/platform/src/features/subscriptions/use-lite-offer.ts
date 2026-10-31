"use client";

import {useOptionalWorkspaceAccount} from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import {useReadResource} from "@/lib/use-read-resource";
import {loadSubscription} from "./subscriptions";

/** Private reads live under the account-keyed workspace; no storage-derived eligibility. */
export function useLiteOffer(){
  const account=useOptionalWorkspaceAccount();
  const state=useReadResource(loadSubscription,null,Boolean(account),account?.profile);
  return {
    visible:!account||!state.pending&&!state.failed&&state.data?.trial_used===false,
    refresh:()=>{if(account)state.retry();},
  };
}
