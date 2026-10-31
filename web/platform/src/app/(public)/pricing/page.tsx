import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/pricing");}
export default async function Page(){return renderContentPage("/pricing");}
