import type {templateDefaults} from './defaults';
import type {TemplateId} from './registry';

export type Json = null|boolean|number|string|Json[]|{[key:string]:Json};
export type ContentPage = {
  id:string; template:TemplateId; path:string; locale:'ru';
  status:'draft'|'review'|'published'|'archived'; indexing:'index'|'noindex';
  seo:{title:string;description:string};
  publishedAt?:string;updatedAt?:string;reviewedAt?:string;
  relatedIds:string[];
  /** Full named sources, not arbitrary JSX or partial nested patches. */
  content:Record<string,Json>;
};
export type TemplateSources<K extends TemplateId> = typeof templateDefaults[K];
