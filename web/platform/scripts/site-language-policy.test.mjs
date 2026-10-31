import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import test from 'node:test';
import {assertRussianOnlyWebsite} from './site-language-policy.mjs';

test('rejects English activation and an English default until the owner changes the release policy',()=>{
  assert.doesNotThrow(()=>assertRussianOnlyWebsite({siteLocales:['ru'],defaultLocale:'ru'}));
  for(const policy of [
    {siteLocales:['ru','en'],defaultLocale:'ru'},
    {siteLocales:['en'],defaultLocale:'en'},
    {siteLocales:['ru'],defaultLocale:'en'},
    {siteLocales:[],defaultLocale:'ru'},
  ])assert.throws(()=>assertRussianOnlyWebsite(policy),/Russian-only website policy/);
});

for(const environment of ['development','production'])test(`checks actual website languages in ${environment}`,()=>{
  const result=spawnSync(process.execPath,[fileURLToPath(new URL('./site-language-policy.mjs',import.meta.url))],{
    encoding:'utf8',env:{...process.env,NODE_ENV:environment},
  });
  assert.equal(result.status,0,result.stderr);
  assert.match(result.stdout,/Russian-only website policy passed/);
});

test('keeps the policy mandatory in build and CI source validation',()=>{
  const {scripts}=JSON.parse(readFileSync(new URL('../package.json',import.meta.url),'utf8'));
  assert.match(scripts['public-copy:check'],/node scripts\/site-language-policy\.mjs/);
  assert.match(scripts.prebuild,/npm run public-copy:check/);
  assert.match(scripts['test:public-copy'],/site-language-policy\.test\.mjs/);
});
