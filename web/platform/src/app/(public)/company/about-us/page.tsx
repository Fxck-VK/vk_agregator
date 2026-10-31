import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/company/about-us");}
export default async function Page(){return renderContentPage("/company/about-us");}
