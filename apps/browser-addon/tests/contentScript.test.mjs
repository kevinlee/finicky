import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import fs from 'node:fs';

function contentScript({firefox = false} = {}) {
  let click;
  const messages = [];
  class Element {
    constructor(href) {this.href = href;}
    closest(selector) {assert.equal(selector, 'a[href]'); return this;}
  }
  const window = {
    addEventListener: (name, cb, capture) => {assert.equal(name, 'click'); assert.equal(capture, true); click = cb;},
    location: {set href(_) {assert.fail('Content script must route through the background');}},
  };
  const api = {runtime: {sendMessage: body => {messages.push(body); return Promise.resolve({accepted: true});}}};
  vm.runInNewContext(fs.readFileSync(new URL('../contentScript.js', import.meta.url), 'utf8'), {
    [firefox ? 'browser' : 'chrome']: api, window, document: {baseURI: 'https://example.com/base/'}, URL, Element,
  });
  const gesture = overrides => {
    const state = {prevented: false, stopped: false};
    click({isTrusted: true, altKey: true, shiftKey: false, button: 0, target: new Element('https://example.com/日本?q=a&b=two'),
      preventDefault: () => {state.prevented = true;}, stopImmediatePropagation: () => {state.stopped = true;}, ...overrides});
    return state;
  };
  return {messages, gesture, Element};
}

for (const firefox of [false, true]) {
  test(`${firefox ? 'Firefox' : 'Chrome'} Option and Option-Shift use background messaging without tab or privacy claims`, () => {
    const {messages, gesture} = contentScript({firefox});
    assert.deepEqual(gesture(), {prevented: true, stopped: true});
    assert.equal(messages[0].type, 'finicky-open-link');
    assert.equal(messages[0].url, 'https://example.com/%E6%97%A5%E6%9C%AC?q=a&b=two');
    assert.equal(messages[0].prompt, false);
    assert.deepEqual(Object.keys(messages[0]).sort(), ['prompt', 'type', 'url']);
    gesture({shiftKey: true});
    assert.equal(messages[1].prompt, true);
  });
}

test('synthetic clicks, ordinary clicks and unsupported URLs never send a request', () => {
  const {messages, gesture, Element} = contentScript();
  for (const overrides of [{isTrusted: false}, {altKey: false}, {button: 1}, {target: null},
    {target: new Element('javascript:alert(1)')}, {target: new Element('file:///tmp/test')},
    {target: new Element('finicky-plus://open?url=https://example.com')}]) {
    assert.deepEqual(gesture(overrides), {prevented: false, stopped: false});
  }
  assert.equal(messages.length, 0);
});

test('nested text targets retain the clicked link and resolve relative URLs', () => {
  const {messages, gesture, Element} = contentScript();
  gesture({target: {parentElement: new Element('../page?x=1#fragment')}});
  assert.equal(messages[0].url, 'https://example.com/page?x=1#fragment');
});
