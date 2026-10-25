import "server-only";
import type {PublicPagePublication} from "@/content/publication/types";
import {documentationArticles} from "@/i18n/public/documentation-en";
import {documentationArticlePath} from "@/features/documentation/documentation-model";
export const publications:readonly PublicPagePublication[]=documentationArticles.map(article=>({id:`epifera-help-${article.slug}`,status:"draft",indexing:"noindex",paths:{en:documentationArticlePath(article.slug),ru:documentationArticlePath(article.slug)}}));
