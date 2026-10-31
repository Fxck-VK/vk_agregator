import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/stock/items/dried-gypsophila/gypsophila-01");}
export default async function Page(){return renderContentPage("/stock/items/dried-gypsophila/gypsophila-01");}
