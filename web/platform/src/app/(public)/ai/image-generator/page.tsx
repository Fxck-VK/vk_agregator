import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/ai/image-generator");}
export default async function Page(){return renderContentPage("/ai/image-generator");}
