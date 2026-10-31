import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/inspiration/cinematic-scenes");}
export default async function Page(){return renderContentPage("/inspiration/cinematic-scenes");}
