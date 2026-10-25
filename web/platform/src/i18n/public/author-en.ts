import {brandName,brandLogoPath} from '@/config/brand';

export const authorPath='/blog/author/epifera';
export const authorProfile={name:brandName,kind:'organization',avatar:brandLogoPath,bio:null,socials:[]} as const;
export type AuthorMaterial={
  readonly id:string;
  readonly title:string;
  readonly category:string;
  readonly href:string;
  readonly cover:{readonly src:string|null;readonly width:number;readonly height:number};
  readonly date:{readonly kind:'draft';readonly iso:string;readonly label:string}|null;
};
export const authorEn={
  metadata:{title:`${brandName} — Editorial guides and learning`,description:`Explore articles, lessons and practical guides from ${brandName}. Browse the archive and find a useful starting point for your next creative project.`},
  skip:'Skip to the author archive',materials:`Materials by ${brandName}`,draftPreview:'Draft preview',
  search:'Search',searchTitle:'Search the editorial library',searchPlaceholder:'Search articles, lessons and guides',
  clear:'Clear search',noResults:'No materials found. Try another word.',resultUnit:'materials',
  first:'First page',previous:'Previous page',next:'Next page',last:'Last page',page:'Page',
  pagination:'Author archive pagination',showing:'Showing',of:'of',
  close:'Close',home:`Go to ${brandName}`,tools:'AI tools',pricing:'Pricing',
  categories:{article:'Guides',comparison:'Comparisons',workflow:'Workflows',course:'Learning',lesson:'Lessons',documentation:'Help & guides'},
  source:{action:'About the cover illustrations',title:'About this editorial archive',link:'View the Magnific reference',description:`This archive brings together existing ${brandName} articles, lessons and guides. The layout and cover illustrations use the saved Magnific author-page reference. Covers are visual illustrations, not verified ${brandName} generation results. Known dates are labelled draft; other materials have no invented publication date. The author is the editorial brand, not an individual profile.`},
} as const;
