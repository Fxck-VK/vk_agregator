import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
const platformRoot=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const sourcePath=path.join(platformRoot,'src/i18n/public/translations/sources.json');
const outputPath=path.join(platformRoot,'src/i18n/public/translations/ru.json');
const auditDir=path.join(root,'docs/research/ru-template-translations');
const sources=JSON.parse(fs.readFileSync(sourcePath,'utf8'));
const files=fs.readdirSync(auditDir).filter(file=>/^(g[123]|additional(?:-[a-z0-9-]+)?)\.json$/.test(file)).sort((a,b)=>Number(a.startsWith('additional'))-Number(b.startsWith('additional'))||a.localeCompare(b));
const sourceIds=new Map(Object.entries(sources).map(([id,source])=>[source,id]));
if(process.argv.includes('--write')){
  for(const file of files.filter(file=>file.startsWith('additional'))){
    for(const row of JSON.parse(fs.readFileSync(path.join(auditDir,file),'utf8'))){
      if(!row.source||sourceIds.has(row.source)||!row.ru?.trim()||!row.back?.trim())continue;
      const id='u'+String(Object.keys(sources).length+1).padStart(4,'0');
      sources[id]=row.source;sourceIds.set(row.source,id);
    }
  }
}
const output={},errors=[];
for(const file of files){
  const rows=JSON.parse(fs.readFileSync(path.join(auditDir,file),'utf8'));
  for(const row of rows){
    if(row.source){row.id=sourceIds.get(row.source);if(Object.hasOwn(output,row.id))continue;}
    const source=sources[row.id];
    if(source===undefined){errors.push(`${file}: unknown source ${row.id}`);continue;}
    if(Object.hasOwn(output,row.id))errors.push(`${file}: duplicate ${row.id}`);
    if(typeof row.ru!=='string'||!row.ru.trim()||typeof row.back!=='string'||!row.back.trim())errors.push(`${row.id}: missing Russian/reverse text`);
    if(row.ru===source&&!row.keepReason)errors.push(`${row.id}: unchanged text needs a reason`);
    if(/\uFFFD/.test(row.ru+row.back))errors.push(`${row.id}: invalid encoding`);
    const tokens=text=>(text.match(/\{[a-zA-Z][a-zA-Z0-9_]*\}/g)||[]).sort().join('|');
    if(tokens(source)!==tokens(row.ru)||tokens(source)!==tokens(row.back))errors.push(`${row.id}: message parameters changed`);
    const numbers=text=>(text.match(/\d+(?:(?:[.,]\d+)|(?:[ \u00a0\u202f]\d{3}(?!\d)))*/g)||[]).map(token=>{
      const compact=token.replace(/[ \u00a0\u202f]/g,'');
      return /^[1-9]\d{0,2}(?:,\d{3})+(?:\.\d+)?$/.test(compact)?compact.replaceAll(',',''):compact.replaceAll(',','.');
    }).join('|');
    if(numbers(source)!==numbers(row.ru))errors.push(`${row.id}: numbers changed`);
    for(const edge of [/^\s*/,/\s*$/])if(source.match(edge)[0]!==row.ru.match(edge)[0])errors.push(`${row.id}: inline whitespace changed`);
    output[row.id]=row.ru;
  }
}
for(const id of Object.keys(sources))if(!Object.hasOwn(output,id))errors.push(`${id}: missing translation`);
if(errors.length){console.error(errors.slice(0,50).join('\n'));process.exitCode=1;}
else{
  const compiled=JSON.stringify(Object.fromEntries(Object.keys(sources).map(id=>[id,output[id]])),null,2)+'\n';
  if(process.argv.includes('--write')){
    fs.writeFileSync(sourcePath,JSON.stringify(sources,null,2)+'\n');
    fs.writeFileSync(outputPath,compiled);
  }
  else if(fs.readFileSync(outputPath,'utf8').replace(/\r\n/g,'\n')!==compiled){console.error('Russian public catalog is stale. Run npm run public-copy:generate.');process.exitCode=1;}
  if(!process.exitCode)console.log(`Russian public copy: ${Object.keys(output).length} source/RU/reverse records validated.`);
}
