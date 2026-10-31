import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/customers/football-campaign");}
export default async function Page(){return renderContentPage("/customers/football-campaign");}
