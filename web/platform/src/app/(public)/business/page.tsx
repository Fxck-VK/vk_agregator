import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/business");}
export default async function Page(){return renderContentPage("/business");}
