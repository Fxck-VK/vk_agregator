import "server-only";
import type {PublicPagePublication} from "@/content/publication/types";
import {documentationPath} from "@/features/documentation/documentation-model";
export const documentationPublication:PublicPagePublication={id:"epifera-help-center",status:"draft",indexing:"noindex",paths:{en:documentationPath,ru:documentationPath}};
export const publications=[documentationPublication];
