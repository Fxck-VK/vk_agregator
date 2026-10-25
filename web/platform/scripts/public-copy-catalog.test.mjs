import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,readFileSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawnSync} from 'node:child_process';
import test from 'node:test';

const script=readFileSync(new URL('./public-copy-catalog.mjs',import.meta.url),'utf8');
const repositoryRoot=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../../..');
function fixture(t,layout){
 const root=mkdtempSync(path.join(tmpdir(),'epifera-copy-catalog-'));
 t.after(()=>rmSync(root,{recursive:true,force:true}));
 const app=path.join(root,...layout),scripts=path.join(app,'scripts'),catalog=path.join(app,'src/i18n/public/translations'),audit=path.join(root,'docs/research/ru-template-translations');
 for(const dir of [scripts,catalog,audit])mkdirSync(dir,{recursive:true});
 writeFileSync(path.join(scripts,'public-copy-catalog.mjs'),script);
 writeFileSync(path.join(catalog,'sources.json'),JSON.stringify({u0001:'Search'}));
 writeFileSync(path.join(catalog,'ru.json'),JSON.stringify({u0001:'Поиск'},null,2)+'\n');
 writeFileSync(path.join(audit,'g1.json'),JSON.stringify([{id:'u0001',ru:'Поиск',back:'Search'}]));
 return {app,audit,script:path.join(scripts,'public-copy-catalog.mjs')};
}
for(const layout of [['web','platform'],['build','app']])test(`checks reviewed translations in ${layout.join('/')} layout`,t=>{
 const item=fixture(t,layout),result=spawnSync(process.execPath,[item.script],{cwd:item.app,encoding:'utf8'});
 assert.equal(result.status,0,result.stderr);assert.match(result.stdout,/1 source\/RU\/reverse records validated/);
});
test('fails if the build does not contain the reverse-translation review',t=>{
 const item=fixture(t,['build','app']);rmSync(path.join(item.audit,'g1.json'));
 const result=spawnSync(process.execPath,[item.script],{cwd:item.app,encoding:'utf8'});
 assert.notEqual(result.status,0);assert.match(result.stderr,/missing translation/);
});
test('accepts checkout CRLF line endings without changing the reviewed catalog',t=>{
 const item=fixture(t,['web','platform']),file=path.join(item.app,'src/i18n/public/translations/ru.json');
 writeFileSync(file,readFileSync(file,'utf8').replace(/\n/g,'\r\n'));
 const result=spawnSync(process.execPath,[item.script],{cwd:item.app,encoding:'utf8'});
 assert.equal(result.status,0,result.stderr);
});
test('Docker build copies the audit to the checker location without shipping it in runtime',()=>{
 const dockerfile=readFileSync(path.join(repositoryRoot,'Dockerfile.platform'),'utf8');
 const [build,runtime]=dockerfile.split(/FROM node:[^\n]+ AS runtime/);
 assert.match(build,/COPY docs\/research\/ru-template-translations\/ \/docs\/research\/ru-template-translations\//);
 assert.doesNotMatch(runtime,/COPY.*(?:\/docs|ru-template-translations)/);
});
