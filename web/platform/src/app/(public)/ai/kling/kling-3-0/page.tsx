import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/ai/kling/kling-3-0");}
export default async function Page(){return renderContentPage("/ai/kling/kling-3-0");}
