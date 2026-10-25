import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {WorkflowLanding} from "@/features/workflow/WorkflowLanding";
import {productCampaignWorkflow,workflowEn} from "@/i18n/public/workflow-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {workflowPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),workflowPublication),...localizePublicData(workflowEn.metadata,await getRequestLocale())};}
export default async function WorkflowPage(){return <WorkflowLanding workflow={productCampaignWorkflow}/>;}
