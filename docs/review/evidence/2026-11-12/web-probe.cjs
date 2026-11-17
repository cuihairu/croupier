// Runs the actual TypeScript parser with two network chunk layouts.
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const assert = require('assert/strict');
const root = path.resolve(__dirname, '../../../..');
const ts = require(require.resolve('typescript', {paths: [path.join(root, 'web')]}));
const source = fs.readFileSync(path.join(root, 'web/src/services/core/http.ts'), 'utf8') + '\nexport { FetchSSEStream };\n';
const compiled = ts.transpileModule(source, {compilerOptions: {module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022}}).outputText;
const sandbox = {
  exports: {}, TextDecoder, Headers, MessageEvent, setTimeout, clearTimeout,
  require: () => ({}),
};
vm.runInNewContext(compiled, sandbox);
async function parse(chunks) {
  const events=[];
  const parser=Object.create(sandbox.exports.FetchSSEStream.prototype);
  parser.listeners=new Map([
    ['update',new Set([e=>events.push(['update',e.data])])],
    ['message',new Set([e=>events.push(['message',e.data])])],
  ]);
  let i=0;
  await parser.readStream({body:{getReader:()=>({read:async()=>i<chunks.length ? {done:false,value:Buffer.from(chunks[i++])}:{done:true}})}});
  return events;
}
(async()=>{
 const control=await parse(['event: update\r\ndata: hello\r\n\r\n']);
 const split=await parse(['event: update\r','\ndata: hello\r\n\r\n']);
 assert.deepEqual(control,[['update','hello']]);
 assert.deepEqual(split,[['message','hello']]);
 console.log('REPRODUCED: same SSE bytes, intact chunk -> '+JSON.stringify(control)+', CRLF split across chunks -> '+JSON.stringify(split));
})().catch(err=>{console.error(err);process.exitCode=1});
