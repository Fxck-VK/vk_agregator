import {contentMetadata,renderContentPage} from '@/content/templates/render';
export async function generateMetadata(){return contentMetadata("/academy/courses/epifera-essentials");}
export default async function Page(){return renderContentPage("/academy/courses/epifera-essentials");}
