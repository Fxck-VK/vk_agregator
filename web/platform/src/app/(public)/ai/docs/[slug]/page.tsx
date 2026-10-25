import {localizePublicData,translatePublicText} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {notFound} from "@/i18n/server";
import {brandName} from "@/config/brand";
import {DocumentationCenter} from "@/features/documentation/DocumentationCenter";
import {findDocumentationArticle} from "@/features/documentation/documentation-model";
import {documentationArticles} from "@/i18n/public/documentation-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {publications} from "./publication";
type Props={params:Promise<{slug:string}>};
export function generateStaticParams(){return documentationArticles.map(article=>({slug:article.slug}));}
export async function generateMetadata({params}:Props):Promise<Metadata>{const {slug}=await params,article=localizePublicData(findDocumentationArticle(slug),await getRequestLocale()),publication=publications.find(item=>item.id===`epifera-help-${slug}`);if(!article||!publication)notFound();return {...publicPageMetadata(await getRequestLocale(),publication),title:`${article.title} | ${translatePublicText(`${brandName} Help`,await getRequestLocale())}`,description:article.description};}
export default async function DocumentationPage({params}:Props){const article=findDocumentationArticle((await params).slug);if(!article)notFound();return <DocumentationCenter article={article}/>;}
