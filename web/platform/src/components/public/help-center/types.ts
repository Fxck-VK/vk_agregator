import type {ReactNode} from "react";
export type HelpIconKind="home"|"grid"|"file"|"support"|"book"|"search"|"play"|"clock"|"level";
export type HelpRailItem={readonly href:string;readonly label:string;readonly icon:HelpIconKind|"plus";readonly current?:boolean;readonly bottom?:boolean};
export type HelpFrameLabels={readonly skip:string;readonly openNavigation:string;readonly navigation:string;readonly closeMenu:string;readonly breadcrumb:string;readonly workspace:string;readonly support:string;readonly login:string;readonly signup:string;readonly sectionTitle:string;readonly openSectionMenu:string};
export type HelpCenterFrameProps={kind:"legal"|"academy"|"documentation";labels:HelpFrameLabels;rail:readonly HelpRailItem[];breadcrumbs:readonly {label:string;href?:string}[];readingKey?:string;mainClassName?:string;hideMobileBreadcrumb?:boolean;utilityAction?:ReactNode;renderNavigation:(onNavigate?:()=>void)=>ReactNode;children:ReactNode};
