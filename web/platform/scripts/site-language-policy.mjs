import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import ts from 'typescript';

/** Owner policy applies to every website build, including DEV and public. */
export function assertRussianOnlyWebsite({siteLocales,defaultLocale}){
  if(defaultLocale!=='ru'||!Array.isArray(siteLocales)||siteLocales.length!==1||siteLocales[0]!=='ru'){
    throw new Error('Russian-only website policy: only ru may be deployed. Enabling English requires the owner\'s explicit request and a reviewed policy change.');
  }
}

async function check(){
  // Trusted repository configuration only; no request data or environment switch.
  const source=readFileSync(new URL('../src/i18n/locales.ts',import.meta.url),'utf8');
  const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
  const policy=await import('data:text/javascript;base64,'+Buffer.from(compiled).toString('base64'));
  assertRussianOnlyWebsite(policy);
  console.log('Russian-only website policy passed: /en remains a legacy redirect, not an English website.');
}

if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
  check().catch(error=>{console.error(error.message);process.exitCode=1;});
}
