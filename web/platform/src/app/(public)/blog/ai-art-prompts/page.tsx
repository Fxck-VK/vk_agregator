import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/blog/ai-art-prompts");}
export default async function Page(){return renderContentPage("/blog/ai-art-prompts");}
