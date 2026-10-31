import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/ai/docs");}
export default async function Page(){return renderContentPage("/ai/docs");}
