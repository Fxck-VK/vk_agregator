import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/compare/ai-avatar-models");}
export default async function Page(){return renderContentPage("/compare/ai-avatar-models");}
