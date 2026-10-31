import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/workflows/product-campaign");}
export default async function Page(){return renderContentPage("/workflows/product-campaign");}
