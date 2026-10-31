import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/events/creative-sessions");}
export default async function Page(){return renderContentPage("/events/creative-sessions");}
