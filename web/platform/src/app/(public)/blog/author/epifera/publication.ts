import 'server-only';
import type {PublicPagePublication} from '@/content/publication/types';
export const authorPublication:PublicPagePublication={
  id:'epifera-author-profile',status:'draft',indexing:'noindex',
  paths:{en:'/blog/author/epifera',ru:'/blog/author/epifera'},
};
export const publications=[authorPublication];
