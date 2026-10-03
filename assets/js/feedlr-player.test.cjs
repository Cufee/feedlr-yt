const test = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const { PlaybackWatchdog, preferredMode, applyQuality, expirationError, restoredState, initialVolume, qualityLabel, qualityHeight, qualityFromLabel } = require('./feedlr-player.js');

function storage() {
  const values = new Map();
  return { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)) };
}

test('manual mode is tab-local and video-specific; guests always use iframe', () => {
  const tab = storage();
  tab.setItem('feedlr-player-mode:one', 'iframe');
  assert.equal(preferredMode(tab, 'one', true), 'iframe');
  assert.equal(preferredMode(tab, 'two', true), 'native');
  assert.equal(preferredMode(storage(), 'one', true), 'native');
  assert.equal(preferredMode(tab, 'two', false), 'iframe');
});

test('startup and continuous stall deadlines exclude suspended playback', () => {
  const watchdog = new PlaybackWatchdog();
  assert.equal(watchdog.tick(9, true, 60, false), null);
  assert.equal(watchdog.tick(300, false, 60, false), null);
  assert.equal(watchdog.tick(1, true, 60, false), 'startup_timeout');
  watchdog.reset();
  assert.equal(watchdog.tick(0, true, 60, true), null);
  assert.equal(watchdog.tick(14, true, 60, true), null);
  assert.equal(watchdog.tick(300, false, 60, true), null);
  assert.equal(watchdog.tick(1, true, 61, true), null); // a decoded advance resets stall
  assert.equal(watchdog.tick(15, true, 61, true), 'playback_stall');
});

test('manual quality selection disables ABR; Auto removes all manual bounds', () => {
  const configs = [];
  const player = { configure: (config) => configs.push(config), getVariantTracks: () => [{ id: 7, height: 720, bandwidth: 200 }], selectVariantTrack: (...args) => { player.selected = args; } };
  assert.equal(applyQuality(player, '720'), '720');
  assert.equal(configs[0].abr.enabled, false);
  assert.equal(player.selected[0].id, 7);
  assert.equal(player.selected[1], true);
  assert.equal(applyQuality(player, 'auto'), 'auto');
  assert.equal(configs[1].abr.enabled, true);
  assert.equal(configs[1].restrictions.maxHeight, Infinity);
  assert.equal(configs[1].abr.restrictions.maxBandwidth, Infinity);
  assert.equal(applyQuality(player, '2160'), '720');
});

test('only expiry or authentication HTTP failures justify an expiry retry', () => {
  assert.equal(expirationError({ category: 1, data: ['url', 403] }, 10000, 1), true);
  assert.equal(expirationError({ category: 1, data: ['url', 500] }, 10000, 1), false);
  assert.equal(expirationError({ category: 3 }, 10000, 10001), true);
});

function harness({ resolutions = [], authenticated = true, blocked = false, shakaLoadError, queued = false, staleQueue = false } = {}) {
  const timers = new Map();
  let nextTimer = 1;
  const requests = [];
  const players = [];
  const order = [];
  const event = (type, values = {}) => Object.assign(new Event(type), values);
  class Element extends EventTarget {
    constructor(tag) {
      super(); this.tagName = tag; this.style = { setProperty() {} }; this.dataset = {}; this.children = [];
      this.classList = { add() {}, remove() {}, toggle() {} }; this.volume = 1; this.muted = false;
      this.currentTime = 0; this.playbackRate = 1; this.paused = true; this.ended = false; this.duration = 5000;
    }
    append(...children) { this.children.push(...children); }
    replaceChildren(...children) { this.children = children; }
    setAttribute(key, value) { this[key] = value; }
    removeAttribute() {}
    querySelector() { return null; }
    querySelectorAll() { return []; }
    contains(element) { return this === element || this.children.some((child) => child.contains(element)); }
    remove() {}
    load() {}
    async play() {
      if (blocked === true || blocked === 'native') { const error = new Error('blocked'); error.name = 'NotAllowedError'; throw error; }
      this.paused = false; this.dispatchEvent(event('play')); this.dispatchEvent(event('playing'));
      this.frameCallback?.();
    }
    pause() { if (!this.paused) { this.paused = true; this.dispatchEvent(event('pause')); } }
    requestVideoFrameCallback(callback) { this.frameCallback = callback; return 1; }
  }
  const elements = new Map(['player', 'player-mode-toggle', 'player-loading', 'close-button', 'notification-toast'].map((id) => [id, new Element('div')]));
  const doc = Object.assign(new EventTarget(), {
    getElementById: (id) => elements.get(id), createElement: (tag) => new Element(tag),
    querySelector: () => null, head: new Element('head'), hidden: false,
  });
  class Shaka extends EventTarget {
    static isBrowserSupported() { return true; }
    constructor() { super(); this.configs = []; players.push(this); order.push('create'); }
    configure(config) { this.configs.push(config); }
    async attach(video) { this.video = video; }
    async load(url, position) { if (shakaLoadError) throw shakaLoadError; this.video.currentTime = position; this.url = url; this.audioOnly = url.endsWith("audio.mpd"); }
    getPlaybackRate() { return this.video.playbackRate; }
    trickPlay(rate) { this.video.playbackRate = rate; }
    getVariantTracks() { return this.audioOnly ? [{id: 3, bandwidth: 128000}] : [{ id: 1, height: 720, bandwidth: 1500 }, { id: 2, height: 1080, bandwidth: 3000 }]; }
    selectVariantTrack(track) { this.selected = track; }
    async destroy() { order.push('destroy'); this.video.pause(); this.video.currentTime = 0; this.video.dispatchEvent(event('ended')); }
  }
  class Overlay { constructor(player) { this.player = player; this.controls = new EventTarget(); } configure(config) { this.config = config; } setEnabled() {} getControls() { return this.controls; } async destroy() { await this.player.destroy(); } }
  class YouTube {
    constructor(_frame, options) {
      this.options = options; this.position = options.playerVars.start; this.state = -1; this.volume = 100; this.muted = false; this.rate = 1;
      queueMicrotask(() => options.events.onReady());
    }
    setVolume(value) { this.volume = value; }
    getVolume() { return this.volume; }
    mute() { this.muted = true; } unMute() { this.muted = false; } isMuted() { return this.muted; }
    getAvailablePlaybackRates() { return [0.5, 1, 1.5, 2]; }
    setPlaybackRate(value) { this.rate = value; } getPlaybackRate() { return this.rate; }
    seekTo(value) { this.position = value; } getCurrentTime() { return this.position; }
    playVideo() { if (blocked === 'iframe') { this.options.events.onAutoplayBlocked(); return; } this.state = 1; this.options.events.onStateChange({ data: 1 }); }
    pauseVideo() { this.state = 2; this.options.events.onStateChange({ data: 2 }); }
    getPlayerState() { return this.state; } unloadModule() {} destroy() { this.position = 0; this.options.events.onStateChange({ data: 0 }); }
  }
  const win = Object.assign(new EventTarget(), {
    document: doc, navigator: { onLine: true }, location: { origin: 'https://feedlr.test' },
    sessionStorage: storage(), localStorage: storage(), shaka: { Player: Shaka, ui: { Overlay }, polyfill: { installAll() {} } }, YT: { Player: YouTube },
    setTimeout: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms }); return id; },
    clearTimeout: (id) => timers.delete(id),
    setInterval: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms }); return id; },
    clearInterval: (id) => timers.delete(id),
  });
  const fresh = (progress = 120, audioOnly = false) => ({ mode: 'native', reason: 'available', progress, audioOnly, qualities: [{width:1280,height:720},{width:1920,height:1080}], manifestUrl: audioOnly ? '/api/playback/session/audio.mpd' : '/api/playback/session/manifest.mpd', expiresAt: new Date(Date.now() + 3600000).toISOString() });
  const fetch = async (url, options) => {
    requests.push({ url, ...options });
    if (url.endsWith('/playback')) {
      const next = resolutions.length ? resolutions.shift() : (JSON.parse(options.body).mode === 'iframe' ? { mode: 'iframe', progress: 120 } : fresh(120, JSON.parse(options.body).audioOnly));
      if (next instanceof Error) throw next;
      return { ok: true, json: async () => next };
    }
    return { ok: true };
  };
  const options = { video: 'one', channel: 'channel', progress: 1, volume: 50, authenticated, withProgress: authenticated, returnURL: '/', segments: [] };
  if (queued) win.feedlrPendingPlayer = { options, root: staleQueue ? new Element('div') : elements.get('player') };
  vm.runInNewContext(fs.readFileSync(require.resolve('./feedlr-player.js'), 'utf8'), { window: win, document: doc, fetch, performance, AbortController, URL, console });
  const controller = queued ? win.feedlr_player : win.FeedlrPlayer.mount(options);
  return { controller, win, doc, timers, requests, players, elements, order, fresh, event };
}

async function settle() { for (let i = 0; i < 6; i++) await new Promise(setImmediate); }

test('initial resolution uses fresh database progress; switch preserves full state and native quality', async () => {
  const h = harness();
  await h.controller.start;
  assert.equal(h.controller.getCurrentTime(), 120);
  const video = h.controller.adapter.video;
  video.currentTime = 321.75; video.volume = 0.35; video.muted = true; video.playbackRate = 1.5; video.pause();
  h.controller.quality = '720';
  await h.controller.switchPlayer('iframe', h.controller.snapshot(), 'manual_switch');
  assert.deepEqual(JSON.parse(JSON.stringify(h.controller.snapshot())), { position: 321.75, playing: false, volume: 35, muted: true, rate: 1.5 });
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'manual_switch');
  assert.equal(h.controller.getCurrentTime(), 321.75);
  assert.equal(h.controller.adapter.video.paused, true);
  assert.equal(h.controller.adapter.player.selected.height, 720);
  assert.deepEqual(h.order, ['create', 'destroy', 'create']);
  assert.equal(h.requests.some((request) => request.url.includes('progress=0')), false);
  h.controller.cleanup();
  await settle();
  assert.equal(h.timers.size, 0);
});

test('guests skip resolution and native scripts; manual choice survives remount', async () => {
  const h = harness({ authenticated: false });
  await h.controller.start;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.requests.length, 0);
  assert.equal(h.players.length, 0);
  h.controller.cleanup();
  const signed = harness();
  await signed.controller.start;
  signed.elements.get('player-mode-toggle').dispatchEvent(signed.event('click'));
  await settle();
  assert.equal(signed.win.sessionStorage.getItem('feedlr-player-mode:one'), 'iframe');
  const again = signed.win.FeedlrPlayer.mount(signed.controller.options);
  await again.start;
  assert.equal(again.mode, 'iframe');
  again.cleanup();
});

test('expired media retries resolution once, preserves position, then falls back', async () => {
  const h = harness();
  await h.controller.start;
  h.controller.adapter.video.currentTime = 567;
  h.controller.adapter.video.pause();
  await h.controller.nativeError({ category: 1, data: ['media', 403] });
  assert.equal(h.controller.mode, 'native');
  assert.equal(h.controller.getCurrentTime(), 567);
  await h.controller.nativeError({ category: 1, data: ['media', 403] });
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.getCurrentTime(), 567);
  assert.equal(h.controller.snapshot().playing, false);
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback') && JSON.parse(request.body).mode === 'native').length, 2);
  h.controller.cleanup();
});

test('renewal rejects unchanged expired responses and does not restore older database progress', async () => {
  const h = harness();
  await h.controller.start;
  h.controller.adapter.video.currentTime = 750;
  h.controller.resolve = async () => ({ ...h.fresh(1), expiresAt: new Date(Date.now() - 1).toISOString() });
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.getCurrentTime(), 750);
  h.controller.cleanup();
});

test('autoplay rejection suspends deadlines and keeps manual native playback available', async () => {
  const h = harness({ blocked: true });
  await h.controller.start;
  assert.equal(h.controller.mode, 'native');
  assert.equal(h.controller.blocked, true);
  assert.equal(h.controller.snapshot().playing, false);
  for (let i = 0; i < 100; i++) h.controller.tick();
  assert.equal(h.controller.watchdog.startup, 0);
  assert.equal(h.controller.mode, 'native');
  h.controller.cleanup();
});

test('history restoration resolves again while retaining the current paused position', async () => {
  const h = harness();
  await h.controller.start;
  h.controller.adapter.video.currentTime = 892;
  h.controller.adapter.video.pause();
  h.win.dispatchEvent(h.event('pagehide', { persisted: true }));
  h.win.dispatchEvent(h.event('pageshow', { persisted: true }));
  await settle();
  assert.equal(h.controller.getCurrentTime(), 892);
  assert.equal(h.controller.snapshot().playing, false);
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback')).length, 2);
  h.controller.cleanup();
});

test('unsupported codecs and fatal load failures settle on iframe without retry loops', async () => {
  const h = harness({ shakaLoadError: new Error('unsupported_codec') });
  await h.controller.start;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.getCurrentTime(), 120);
  assert.equal(h.players.length, 1);
  h.controller.cleanup();
});

test('asynchronous setters cannot replace restored state with transient default values', () => {
  const target = { position: 424, playing: false, volume: 40, muted: true, rate: 1.5 };
  const pending = { ...target };
  const transient = { position: 0, playing: true, volume: 100, muted: false, rate: 0 };
  assert.deepEqual(restoredState(transient, pending), target);
  assert.deepEqual(restoredState({ ...target }, pending), target);
  assert.deepEqual(pending, {});
  // Once acknowledged, normal user controls can change any value, including zero.
  assert.deepEqual(restoredState({ ...target, position: 0, volume: 0 }, pending), { ...target, position: 0, volume: 0 });
});

test('progress reports volume and suppresses teardown zeroes even during fallback', async () => {
  const h = harness();
  await h.controller.start;
  h.controller.setVolume(25);
  h.controller.seekTo(300);
  h.controller.saveProgress();
  assert.equal(h.requests.some((request) => request.url.endsWith('progress=300&volume=25')), true);
  await h.controller.fallback('playback_stall');
  h.controller.cleanup();
  assert.equal(h.requests.some((request) => /progress=0(?:&|$)/.test(request.url)), false);
});

test('hidden, offline, and paused native players do not proactively renew', async () => {
  const h = harness();
  await h.controller.start;
  h.controller.renewAt = 0;
  h.doc.hidden = true;
  h.controller.checkRenewal();
  h.doc.hidden = false;
  h.win.navigator.onLine = false;
  h.controller.checkRenewal();
  h.win.navigator.onLine = true;
  h.controller.pauseVideo();
  h.controller.checkRenewal();
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback')).length, 1);
  h.controller.playVideo();
  await settle();
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback')).length, 2);
  h.controller.cleanup();
});

test('iframe autoplay policy updates the pending restored play state', async () => {
  const h = harness({ blocked: 'iframe', authenticated: false });
  await h.controller.start;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.snapshot().playing, false);
  h.controller.cleanup();
});

test('native UI pause overrides an unacknowledged requested play state', async () => {
  const h = harness();
  await h.controller.start;
  const video = h.controller.adapter.video;
  video.dispatchEvent(h.event('play'));
  video.paused = true;
  video.dispatchEvent(h.event('pause'));
  assert.equal(h.controller.snapshot().playing, false);
  video.paused = false;
  video.dispatchEvent(h.event('play'));
  assert.equal(h.controller.snapshot().playing, true);
  h.controller.cleanup();
});


test('saved quality selects the highest available below the ceiling', () => {
  const player = { configure() {}, getVariantTracks: () => [
    { height: 360, bandwidth: 100 }, { height: 720, bandwidth: 200 }, { height: 2160, bandwidth: 500 },
  ], selectVariantTrack(track) { this.selected = track; } };
  assert.equal(applyQuality(player, '1080'), '720');
  assert.equal(player.selected.height, 720);
  assert.equal(applyQuality(player, '2160'), '2160');
  assert.equal(applyQuality(player, '144'), '360');
});

test('quality preference survives lower-resolution videos and remounts', async () => {
  const h = harness();
  await h.controller.start;
  h.win.localStorage.setItem('feedlr-player-quality', '2160');
  const again = h.win.FeedlrPlayer.mount(h.controller.options);
  await again.start;
  assert.equal(again.quality, '2160');
  assert.equal(again.adapter.player.selected.height, 1080);
  assert.equal(h.win.localStorage.getItem('feedlr-player-quality'), '2160');
  again.cleanup();
});

test('resolution labels and new-page sound defaults', () => {
  assert.equal(qualityLabel(720), '720p');
  assert.equal(qualityLabel(1080), '1080p');
  assert.equal(qualityLabel(1440), '2K');
  assert.equal(qualityLabel(2160), '4K');
  assert.equal(qualityLabel(4320), '8K');
  assert.equal(initialVolume(null, 0), 100);
  assert.equal(initialVolume('0', 0), 100);
  assert.equal(initialVolume('35', 0), 35);
});

test('native video requests autoplay with a single controls implementation', async () => {
  const h = harness();
  await h.controller.start;
  assert.equal(h.controller.adapter.video.autoplay, true);
  assert.equal(h.controller.adapter.video.controls, false);
  assert.equal(h.controller.adapter.video.muted, false);
  h.controller.cleanup();
});


test('portrait and ultrawide quality names and ceilings use displayed resolution', () => {
  assert.equal(qualityLabel(qualityHeight({width:1080,height:1920})), '1080p');
  assert.equal(qualityLabel(qualityHeight({width:3840,height:1600})), '4K');
  const player = { configure() {}, getVideoTracks: () => [
    {width:1080,height:1920,bandwidth:300}, {width:720,height:1280,bandwidth:200},
  ], selectVideoTrack(track) { this.selected = track; } };
  assert.equal(applyQuality(player, '720'), '720');
  assert.equal(player.selected.width, 720);
});


test('persist the clicked quality label even while the old track remains active', () => {
  assert.equal(qualityFromLabel('2K'), '1440');
  assert.equal(qualityFromLabel('4K'), '2160');
  assert.equal(qualityFromLabel('1080p'), '1080');
  assert.equal(qualityFromLabel('Auto'), null);
});


test('HTMX inline initialization can arrive before the external bundle', async () => {
  const h = harness({queued: true});
  await h.controller.start;
  assert.equal(h.controller.ready, true);
  assert.equal(h.controller.mode, 'native');
  assert.equal(h.players.length, 1);
  assert.equal(h.win.feedlrPendingPlayer, undefined);
  h.controller.cleanup();
});

test('a delayed player bundle does not mount a page that was already replaced', () => {
  const h = harness({queued: true, staleQueue: true});
  assert.equal(h.controller, undefined);
  assert.equal(h.requests.length, 0);
  assert.equal(h.win.feedlrPendingPlayer, undefined);
});

test('HTMX back navigation cleans up a player whose page was removed', async () => {
  const h = harness();
  await h.controller.start;
  h.elements.delete('player');
  h.doc.dispatchEvent(h.event('htmx:historyRestore'));
  await settle();
  assert.equal(h.controller.disposed, true);
  assert.equal(h.win.feedlr_player, undefined);
  assert.equal(h.timers.size, 0);
});


test('audio-only switches resolve audio manifests and preserve paused playback state', async () => {
  const h = harness();
  await h.controller.start;
  const video = h.controller.adapter.video;
  video.currentTime = 321.5; video.volume = 0.35; video.muted = true; video.playbackRate = 1.5; video.pause();
  const before = JSON.parse(JSON.stringify(h.controller.snapshot()));
  await h.controller.selectQuality('audio');
  assert.equal(h.controller.audioOnly, true);
  assert.equal(h.controller.adapter.player.getVariantTracks().some(track => track.height), false);
  assert.equal(h.controller.adapter.waveform.dataset.playing, 'false');
  assert.deepEqual(JSON.parse(JSON.stringify(h.controller.snapshot())), before);
  assert.equal(h.win.localStorage.getItem('feedlr-player-quality'), 'audio');
  const requests = h.requests.filter(request => request.url.endsWith('/playback'));
  assert.equal(JSON.parse(requests.at(-1).body).audioOnly, true);
  await h.controller.selectQuality('720');
  assert.equal(h.controller.audioOnly, false);
  assert.equal(h.controller.adapter.player.selected.height, 720);
  assert.deepEqual(JSON.parse(JSON.stringify(h.controller.snapshot())), before);
  h.controller.cleanup();
});

test('audio-only persists on reload and renews as audio', async () => {
  const h = harness();
  await h.controller.start;
  await h.controller.selectQuality('audio');
  const again = h.win.FeedlrPlayer.mount(h.controller.options);
  await again.start;
  assert.equal(again.quality, 'audio');
  assert.equal(again.audioOnly, true);
  again.pauseVideo();
  assert.equal(again.adapter.waveform.dataset.playing, 'false');
  await again.switchPlayer('native', again.snapshot(), 'renewal');
  assert.equal(again.audioOnly, true);
  const request = h.requests.filter(item => item.url.endsWith('/playback')).at(-1);
  assert.equal(JSON.parse(request.body).audioOnly, true);
  again.cleanup();
});
