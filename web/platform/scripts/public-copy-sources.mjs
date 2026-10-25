import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {createRequire} from 'node:module';
import ts from 'typescript';

// Evaluate only the repository's trusted, static public editorial data modules.
// This check never imports UI components, request data, services or providers.
const require=createRequire(import.meta.url),base=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../src'),cache=new Map();
const sources=new Set(Object.values(JSON.parse(fs.readFileSync(path.join(base,'i18n/public/translations/sources.json'),'utf8'))));
const excluded=new Set(['id','slug','href','url','src','file','poster','key','value','type','kind','status','mode','tier','currency','period','group','formats','extensions','executionMode','modelId','model_id','operation','operationId','assetId','ownerId','ownerKind','publicId','categoryId','icon','tone','mark','color','className','indexing','sourceWidth','sourceHeight','taxonId','taxonIds','relatedCategoryIds','categoryIds','orderedMemberIds','license','people','mediaType','downloadPolicy','origin','memberType','taskIds','publicMediaIds','sortPolicy','modelVersionId','mime','filename','path','moderationState','direction','orientation']);
function load(name){
 if(name==='server-only')return {};
 if(!name.startsWith('@/')&&!path.isAbsolute(name))return require(name);
 let file=name.startsWith('@/')?path.join(base,name.slice(2)):name;
 if(!fs.existsSync(file))file+=fs.existsSync(file+'.ts')?'.ts':'.tsx';
 if(file.endsWith('.json'))return JSON.parse(fs.readFileSync(file,'utf8'));
 if(cache.has(file))return cache.get(file).exports;
 const evaluated={exports:{}};cache.set(file,evaluated);
 const code=ts.transpileModule(fs.readFileSync(file,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022,esModuleInterop:true,resolveJsonModule:true}}).outputText;
 new Function('require','module','exports',code)(ref=>load(ref.startsWith('.')?path.resolve(path.dirname(file),ref):ref),evaluated,evaluated.exports);
 return evaluated.exports;
}
const missing=new Map();
function walk(value,scope,key){
 if(excluded.has(key))return;
 if(typeof value==='string'){
   if(key==='support'&&['supported','unsupported','unspecified'].includes(value)||key==='format'&&['webp','png','jpeg','jpg','mp4','webm'].includes(value))return;
   if(!/[A-Za-z]/.test(value)||/^(@\/|\.\/|\/|#|https?:|mailto:|data:)/.test(value)||/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)||/^[^\s]+\.(webp|png|jpg|jpeg|svg|webm|mp4|avif)$/i.test(value))return;
   if(!sources.has(value))missing.set(value,scope);
 }else if(value&&typeof value==='object')for(const[k,v]of Object.entries(value))walk(v,scope+'.'+k,k);
}
for(const file of fs.readdirSync(path.join(base,'i18n/public')).filter(file=>file.endsWith('-en.ts')))walk(load(path.join(base,'i18n/public',file)),file);
if(missing.size){console.error([...missing].map(([source,scope])=>JSON.stringify({source,scope})).join('\n'));console.error(`${missing.size} public source strings need EN → RU → reverse review.`);process.exitCode=1;}
else console.log('Public editorial sources match the reviewed translation inventory.');
