import type {ReactNode} from "react";
export type EditorialImage = {readonly src:string;readonly alt:string;readonly width:number;readonly height:number};
export type EditorialSection = {readonly id:string;readonly label:string};
export type EditorialLabels = {
  readonly home:string;readonly search:string;readonly openWorkspace:string;readonly breadcrumb:string;readonly tools:string;
  readonly listen:string;readonly share:string;readonly shareLink:string;readonly copy:string;readonly copied:string;
  readonly copying:string;readonly copyFailure:string;readonly pageLink:string;readonly close:string;
  readonly comingSoon:string;readonly audioTitle:string;readonly audioDescription:string;readonly speech:string;readonly backTop:string;
};
export type EditorialPageData = {
  readonly id:string;readonly kind:"article"|"comparison";readonly title:string;readonly skip:string;
  readonly breadcrumbs:readonly {readonly label:string;readonly href?:string}[];
  readonly author:{readonly label:string;readonly href:string;readonly date:string;readonly dateLabel:string};
  readonly minutes:number;readonly readTime:string;readonly controls:EditorialLabels;
  readonly onPage:string;readonly sections:readonly EditorialSection[];readonly hero:EditorialImage;readonly intro?:string;
  readonly source:string;readonly inspiration:{readonly title:string;readonly action:string;readonly href:string;readonly image:EditorialImage};
  readonly related:{readonly title:string;readonly items:readonly {readonly category:string;readonly title:string;readonly description:string;readonly href:string;readonly image:EditorialImage}[]};
};
export type EditorialPageProps = {page:EditorialPageData;children:ReactNode};
