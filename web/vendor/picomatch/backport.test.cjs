const assert = require('node:assert/strict');
const test = require('node:test');
const picomatch = require('./index');
const posix = require('./posix');

test('keeps ordinary and bounded glob patterns available', () => {
  assert.equal(picomatch('src/**/*.{ts,tsx}')('src/audio/DownloadPage.tsx'), true);
  assert.equal(picomatch('@(report|draft).txt')('report.txt'), true);
  assert.equal(picomatch('report?(s).txt')('reports.txt'), true);
  assert.equal(picomatch('audio/!(draft).txt')('audio/draft.txt'), false);
  assert.equal(picomatch('\\+\\(note\\).txt')('+(note).txt'), true);
  assert.equal(posix('src/*.js')('src/index.js'), true);
});

test('rejects unbounded extglob tokens on both public entry points', () => {
  const rejected = {
    name: 'SyntaxError',
    message: 'Unbounded repeated extglobs are disabled in the Epifera build toolchain'
  };

  for (const match of [picomatch, posix]) {
    assert.throws(() => match('+(note).txt'), rejected);
    assert.throws(() => match.makeRe('*(note).txt'), rejected);
    assert.throws(() => match.parse('?(+(note)).txt', { fastpaths: false }), rejected);
    assert.throws(() => match('+(note).txt', { maxExtglobRecursion: false }), rejected);
  }
});
