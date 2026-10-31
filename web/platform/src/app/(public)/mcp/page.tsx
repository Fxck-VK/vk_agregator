import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/mcp");}
export default async function Page(){return renderContentPage("/mcp");}
