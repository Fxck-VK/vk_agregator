import {readFileSync,existsSync} from 'node:fs';
import {resolve,dirname,extname} from 'node:path';
import {createRequire} from 'node:module';
import ts from 'typescript';

/** Authoring CLI only. Loads trusted tracked TS data, never files supplied by a web request. */
export function sourceLoader(root){
  const cache=new Map(),native=createRequire(resolve(root,'package.json'));
  function load(filename){
    let file=resolve(filename);
    if(!extname(file))file=['.ts','.tsx','.json'].map(e=>file+e).find(existsSync)??file;
    if(cache.has(file))return cache.get(file).exports;
    if(extname(file)==='.json')return JSON.parse(readFileSync(file,'utf8'));
    const loaded={exports:{}};cache.set(file,loaded);
    const code=ts.transpileModule(readFileSync(file,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022,esModuleInterop:true,resolveJsonModule:true,jsx:ts.JsxEmit.ReactJSX}}).outputText;
    const require=specifier=>{
      if(specifier==='server-only')return {};
      if(specifier.startsWith('@/'))return load(resolve(root,'src',specifier.slice(2)));
      if(specifier.startsWith('.'))return load(resolve(dirname(file),specifier));
      return native(specifier);
    };
    new Function('require','module','exports',code)(require,loaded,loaded.exports);
    return loaded.exports;
  }
  return load;
}
