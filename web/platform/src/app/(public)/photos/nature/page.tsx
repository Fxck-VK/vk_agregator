import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/photos/nature");}
export default async function Page(){return renderContentPage("/photos/nature");}
