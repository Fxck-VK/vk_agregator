import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/ai/tattoo-generator");}
export default async function Page(){return renderContentPage("/ai/tattoo-generator");}
