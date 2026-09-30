const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync('public/js/personal-tracking.js', 'utf8');

function browser(secure = true) {
  const elements = Object.fromEntries(['share-start', 'share-stop', 'share-status'].map(id => [id, {
    disabled: id === 'share-stop', handlers: {},
    addEventListener(event, handler) { this.handlers[event] = handler; }
  }]));
  const sockets = [], cleared = [], retries = [];
  let position, gpsError;
  class Socket {
    static OPEN = 1;
    constructor(url) { this.url = url; this.readyState = 0; this.sent = []; sockets.push(this); }
    send(value) { this.sent.push(JSON.parse(value)); }
    close() { this.readyState = 3; this.closed = true; if (this.onclose) this.onclose({code: 1000}); }
    open() { this.readyState = 1; this.onopen(); }
  }
  vm.runInNewContext(source, {
    document: {
      getElementById: id => elements[id],
      addEventListener: (event, handler) => handler()
    },
    navigator: {geolocation: {
      watchPosition: (success, failure) => { position = success; gpsError = failure; return 7; },
      clearWatch: id => cleared.push(id)
    }},
    window: {isSecureContext: secure, addEventListener() {}},
    location: {protocol: 'https:', host: 'halcon.example'}, WebSocket: Socket,
    setInterval: () => 1, clearInterval() {},
    setTimeout: callback => { retries.push(callback); return 1; }, clearTimeout() {}
  });
  return {
    elements, sockets, cleared, retries,
    start: () => elements['share-start'].handlers.click(),
    stop: () => elements['share-stop'].handlers.click(),
    position: (latitude, longitude, timestamp = Date.now()) => position({coords: {latitude, longitude}, timestamp}),
    deny: () => gpsError({code: 1})
  };
}

test('transmits GPS including zero coordinates and stops the watch', () => {
  const b = browser(); b.start(); b.sockets[0].open(); b.position(0, 0);
  assert.equal(b.sockets[0].url, 'wss://halcon.example/location');
  assert.deepEqual(b.sockets[0].sent, [{latitude: 0, longitude: 0}]);
  b.stop();
  assert.deepEqual(b.cleared, [7]);
  assert.equal(b.sockets[0].closed, true);
  assert.equal(b.elements['share-start'].disabled, false);
});

test('replacement and revoked authorization stop reconnection', () => {
  for (const code of [4009, 1008]) {
    const b = browser(); b.start(); b.sockets[0].onclose({code});
    assert.deepEqual(b.cleared, [7]);
    assert.equal(b.retries.length, 0);
    assert.equal(b.elements['share-stop'].disabled, true);
  }
});

test('permission denial stops transmission and stale GPS is not sent', () => {
  const b = browser(); b.start(); b.sockets[0].open(); b.position(10, 20, Date.now() - 60000);
  assert.equal(b.sockets[0].sent.length, 0);
  b.deny();
  assert.equal(b.sockets[0].closed, true);
  assert.deepEqual(b.cleared, [7]);
});

test('insecure context does not start GPS or a socket', () => {
  const b = browser(false); b.start();
  assert.equal(b.sockets.length, 0);
  assert.match(b.elements['share-status'].textContent, /HTTPS/);
});
