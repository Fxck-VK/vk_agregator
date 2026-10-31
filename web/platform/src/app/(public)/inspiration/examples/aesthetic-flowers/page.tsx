import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/inspiration/examples/aesthetic-flowers");}
export default async function Page(){return renderContentPage("/inspiration/examples/aesthetic-flowers");}
