import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import fs from 'node:fs';

function background({firefox = false, queryTabs = []} = {}) {
  const listeners = {}; const updates = []; const menus = [];
  const event = key => ({addListener: cb => listeners[key] = cb});
  const api = {
    runtime: {id: 'finicky-test', onInstalled: event('install'), onMessage: event('message')},
    contextMenus: {onClicked: event('menu'), removeAll: async (...args) => assert.equal(args.length, 0), create: item => menus.push(item)},
    commands: {onCommand: event('command')},
    tabs: {update: (id, obj) => {updates.push([id, obj]); return Promise.resolve();}, query: async () => queryTabs},
    [firefox ? 'browserAction' : 'action']: {onClicked: event('action')},
  };
  vm.runInNewContext(fs.readFileSync(new URL('../background.js', import.meta.url), 'utf8'), {[firefox ? 'browser' : 'chrome']: api, URL});
  const message = (body, tab, senderID = api.runtime.id) => {
    let response;
    listeners.message(body, {id: senderID, tab}, value => {response = value;});
    return response;
  };
  return {listeners, updates, menus, message};
}

for (const firefox of [false, true]) {
  test(`${firefox ? 'Firefox' : 'Chrome'} toolbar and menus preserve encoding, mode and observed privacy`, async () => {
    const {listeners, updates, menus} = background({firefox});
    await listeners.install();
    assert.equal(menus.length, 3);
    listeners.action({id: 1, incognito: false, url: 'https://example.com/日本?q=a&b=two#hello'});
    const request = new URL(updates[0][1].url);
    assert.equal(request.protocol, 'finicky-plus:');
    assert.equal(request.searchParams.get('url'), 'https://example.com/%E6%97%A5%E6%9C%AC?q=a&b=two#hello');
    assert.equal(request.searchParams.get('prompt'), 'true');
    assert.equal(request.searchParams.get('sourcePrivacy'), 'public');
    listeners.menu({menuItemId: 'finicky-route-link', linkUrl: 'https://example.com'}, {id: 2, incognito: true});
    const route = new URL(updates[1][1].url);
    assert.equal(route.searchParams.has('prompt'), false);
    assert.equal(route.searchParams.get('sourcePrivacy'), 'private');
    listeners.menu({menuItemId: 'finicky-choose-link', linkUrl: 'https://example.com'}, {id: 3});
    assert.equal(new URL(updates[2][1].url).searchParams.has('sourcePrivacy'), false);
    listeners.menu({menuItemId: 'finicky-choose-page', pageUrl: 'https://example.com/page'}, {id: 4, incognito: false});
    assert.equal(new URL(updates[3][1].url).searchParams.get('prompt'), 'true');
    listeners.menu({menuItemId: 'toString', linkUrl: 'https://example.com'}, {id: 4, incognito: false});
    listeners.action({id: 1, incognito: false, url: 'javascript:alert(1)'});
    listeners.action(undefined);
    assert.equal(updates.length, 4);
  });
}

test('keyboard command uses privacy from the selected tab', async () => {
  const {listeners, updates} = background({queryTabs: [{id: 8, incognito: true, url: 'https://example.com'}]});
  await listeners.command('choose-profile');
  assert.equal(updates[0][0], 8);
  assert.equal(new URL(updates[0][1].url).searchParams.get('sourcePrivacy'), 'private');
  await listeners.command('unknown');
  assert.equal(updates.length, 1);
});

test('content message uses the browser sender tab and preserves embedded query parameters', () => {
  const {message, updates} = background();
  const body = {type: 'finicky-open-link', url: 'https://example.com/日本?a=1&sourcePrivacy=public#section', prompt: true};
  assert.equal(message(body, {id: 6, incognito: true}).accepted, true);
  assert.equal(updates[0][0], 6);
  const request = new URL(updates[0][1].url);
  assert.equal(request.searchParams.get('sourcePrivacy'), 'private');
  assert.equal(request.searchParams.get('prompt'), 'true');
  assert.equal(request.searchParams.get('url'), new URL(body.url).href);
  assert.equal(message({...body, prompt: false}, {id: 9, incognito: false}).accepted, true);
  assert.equal(new URL(updates[1][1].url).searchParams.has('prompt'), false);
  assert.equal(new URL(updates[1][1].url).searchParams.get('sourcePrivacy'), 'public');
});

test('missing or nonboolean privacy is unknown; a missing tab cannot navigate', () => {
  const {message, updates} = background();
  const body = {type: 'finicky-open-link', url: 'https://example.com', prompt: false};
  for (const incognito of [undefined, null, 'false', 0]) {
    assert.equal(message(body, {id: 3, incognito}).accepted, true);
    assert.equal(new URL(updates.at(-1)[1].url).searchParams.has('sourcePrivacy'), false);
  }
  for (const tab of [undefined, {}, {id: -1}, {id: '2'}]) assert.equal(message(body, tab).accepted, false);
  assert.equal(updates.length, 4);
});

test('rejects malformed messages, spoofed metadata and callers outside the extension', () => {
  const {message, updates} = background();
  const valid = {type: 'finicky-open-link', url: 'https://example.com', prompt: false};
  for (const body of [null, [], 'open', {}, {...valid, url: {}}, {...valid, prompt: 'false'},
    {...valid, type: 'open'}, {...valid, sourcePrivacy: 'public'}, {...valid, tabId: 99}, {...valid, tab: {id: 99, incognito: false}},
    {...valid, url: 'javascript:alert(1)'}, {...valid, url: 'finicky-plus://open?url=https://example.com'},
    {...valid, url: 'file:///tmp/a'}, {...valid, url: '/relative'}]) {
    message(body, {id: 1, incognito: false});
  }
  message(valid, {id: 1, incognito: false}, 'another-extension');
  assert.equal(updates.length, 0);
});
