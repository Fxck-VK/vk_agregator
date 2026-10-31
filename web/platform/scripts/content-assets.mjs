import {existsSync,statSync} from 'node:fs';
import {resolve} from 'node:path';

export function requirePublicMediaFile(publicRoot,url){
  const root=resolve(publicRoot),file=resolve(root,'.'+url);
  if(!file.startsWith(root+'\\')&&!file.startsWith(root+'/'))throw new Error('Media escapes public directory.');
  if(!existsSync(file)||!statSync(file).isFile())throw new Error('Missing public media file: '+url);
}
export function requireTemplateMediaFile(assets,filename,original){
  const candidates=assets.filter(file=>file.endsWith('/'+original));
  if(!candidates.length||!candidates.some(file=>{const target=resolve(file.slice(0,-original.length),filename);return existsSync(target)&&statSync(target).isFile();}))throw new Error('Media filename is missing from the template asset directory: '+filename);
}
