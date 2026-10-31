import {test} from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {resolve} from 'node:path';
import {requirePublicMediaFile,requireTemplateMediaFile} from './content-assets.mjs';
const root=fileURLToPath(new URL('../public',import.meta.url));
const mark='/assets/brand/marks/epifera-mark.svg';
test('accepts a real public image',()=>assert.doesNotThrow(()=>requirePublicMediaFile(root,mark)));
test('rejects a directory passed as an image URL',()=>assert.throws(()=>requirePublicMediaFile(root,'/assets/')));
test('rejects a directory passed as a template filename',()=>assert.throws(()=>requireTemplateMediaFile([resolve(root,'.'+mark).replaceAll('\\','/')],'.','epifera-mark.svg')));
test('rejects traversal and absent media',()=>{
  assert.throws(()=>requirePublicMediaFile(root,'/assets/../../not-public'));
  assert.throws(()=>requirePublicMediaFile(root,'/assets/not-an-image.svg'));
});
