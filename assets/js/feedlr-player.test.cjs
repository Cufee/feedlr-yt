const test = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const { PlaybackWatchdog, applyQuality, expirationError, restoredState, initialVolume, qualityLabel, qualityHeight, qualityFromLabel } = require('./feedlr-player.js');

function storage() {
  const values = new Map();
  return { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)) };
}

function chooseAudio(player, index) {
  const track = player.getAudioTracks()[index];
  // The built-in Shaka menu both selects the track and updates preferences.
  player.selectAudioTrack(track);
  player.configure({ preferredAudio: [{ language: track.language, role: '', label: track.label || '', channelCount: track.channelsCount || 0, spatialAudio: track.spatialAudio }] });
}

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

function harness({ resolutions = [], tvStatuses = [], tvSends = [], progressResponses = [], shakaLoads = [], audioTracks = [{ language: 'und', roles: [], active: true }], nativeAutoplay = false, authenticated = true, blocked = false, shakaLoadError, queued = false, staleQueue = false, legacyIframeChoice = false, mobile = false, savedVolume } = {}) {
  const timers = new Map();
  let nextTimer = 1;
  const requests = [];
  const players = [];
  const order = [];
  const event = (type, values = {}) => Object.assign(new Event(type), values);
  class Element extends EventTarget {
    constructor(tag) {
      super(); this.tagName = tag; this.style = { setProperty() {} }; this.dataset = {}; this.children = [];
      const classes = new Set();
      this.classList = { add: (name) => classes.add(name), remove: (name) => classes.delete(name), contains: (name) => classes.has(name), toggle: (name, force) => { if (force ?? !classes.has(name)) classes.add(name); else classes.delete(name); } }; this.volume = 1; this.muted = false;
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
  const elements = new Map(['player', 'player-loading', 'close-button', 'notification-toast', 'send-to-tv-btn'].map((id) => [id, new Element('div')]));
  elements.get('send-to-tv-btn').hidden = true;
  elements.get('send-to-tv-btn').disabled = true;
  const toastText = new Element('span');
  elements.get('notification-toast').querySelector = () => toastText;
  const doc = Object.assign(new EventTarget(), {
    getElementById: (id) => elements.get(id), createElement: (tag) => new Element(tag),
    querySelector: () => null, head: new Element('head'), hidden: false,
  });
  class Shaka extends EventTarget {
    static isBrowserSupported() { return true; }
    constructor() { super(); this.configs = []; this.audioTracks = structuredClone(audioTracks); players.push(this); order.push('create'); }
    configure(config) { this.configs.push(config); }
    getConfiguration() { return { preferredAudio: this.configs.findLast((config) => config.preferredAudio)?.preferredAudio }; }
    async attach(video) { this.video = video; }
    async load(url, position) {
      this.autoplayAtLoad = this.video.autoplay;
      this.audioPreferencesAtLoad = this.configs.findLast((config) => config.preferredAudio)?.preferredAudio;
      if (shakaLoadError) throw shakaLoadError;
      if (shakaLoads.length) await shakaLoads.shift();
      this.video.currentTime = position; this.url = url; this.audioOnly = url.endsWith("audio.mpd");
      if (nativeAutoplay && this.video.autoplay) await this.video.play();
    }
    getPlaybackRate() { return this.video.playbackRate; }
    trickPlay(rate) { this.video.playbackRate = rate; }
    getVariantTracks() { return this.audioOnly ? [{id: 3, bandwidth: 128000}] : [{ id: 1, height: 720, bandwidth: 1500 }, { id: 2, height: 1080, bandwidth: 3000 }]; }
    getAudioTracks() { return this.audioTracks; }
    selectAudioTrack(track) { this.audioTracks.forEach((item) => { item.active = item === track; }); }
    selectVariantTrack(track) { this.selected = track; }
    async destroy() { order.push('destroy'); this.video.pause(); this.video.currentTime = 0; this.video.dispatchEvent(event('ended')); }
  }
  const menuFactories = new Map();
  class UIElement {
    constructor(_parent, controls) {
      this.abort = new AbortController();
      this.eventManager = { listen: (target, type, callback) => target.addEventListener(type, callback, { signal: this.abort.signal }) };
      this.isSubMenuOpened = false;
      this.eventManager.listen(controls, 'submenuopen', () => { this.isSubMenuOpened = true; this.checkAvailability(); });
      this.eventManager.listen(controls, 'submenuclose', () => { this.isSubMenuOpened = false; this.checkAvailability(); });
    }
    release() { this.abort.abort(); }
  }
  class Overlay {
    static TrackLabelFormat = { LABEL: 3, LABEL_OR_LANGUAGE: 4 };
    constructor(player) {
      this.player = player;
      player.ui = this;
      this.controls = Object.assign(new EventTarget(), { getLocalPlayer: () => player, getPlayer: () => ({}), getMediaSession: () => ({ setupTitle() {}, setupArtist() {}, setupPoster() {} }), hideSettingsMenus: () => { this.menuHidden = true; } });
      this.children = [];
    }
    configure(config) {
      this.config = { ...this.config, ...config };
      this.children.forEach((child) => child.release());
      this.children = this.config.overflowMenuButtons.filter((name) => menuFactories.has(name)).map((name) => menuFactories.get(name).create(new Element('div'), this.controls));
    }
    setEnabled() {}
    getControls() { return this.controls; }
    async destroy() { this.children.forEach((child) => child.release()); await this.player.destroy(); }
  }
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
    document: doc, navigator: { onLine: true }, location: { origin: 'https://feedlr.test' }, matchMedia: () => ({ matches: mobile }),
    sessionStorage: storage(), localStorage: storage(), shaka: { Player: Shaka, ui: { Overlay, Element: UIElement, OverflowMenu: { registerElement: (name, factory) => menuFactories.set(name, factory) } }, polyfill: { installAll() {} } }, YT: { Player: YouTube },
    setTimeout: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms }); return id; },
    clearTimeout: (id) => timers.delete(id),
    setInterval: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms }); return id; },
    clearInterval: (id) => timers.delete(id),
  });
  const fresh = (progress = 120, audioOnly = false) => ({ mode: 'native', reason: 'available', progress, audioOnly, qualities: [{width:1280,height:720},{width:1920,height:1080}], manifestUrl: audioOnly ? '/api/playback/session/audio.mpd' : '/api/playback/session/manifest.mpd', expiresAt: new Date(Date.now() + 3600000).toISOString() });
  const fetch = async (url, options) => {
    requests.push({ url, ...options });
    if (url === '/api/tv/status') {
      const next = tvStatuses.length ? await tvStatuses.shift() : { online: false, screenName: '' };
      if (next instanceof Error) throw next;
      return next.ok === undefined ? { ok: true, json: async () => next } : next;
    }
    if (url.endsWith('/tv')) {
      const next = tvSends.length ? await tvSends.shift() : { ok: true, status: 204 };
      if (next instanceof Error) throw next;
      return next;
    }
    if (url.includes('/progress?') && progressResponses.length) return progressResponses.shift();
    if (url.endsWith('/playback')) {
      const next = resolutions.length ? resolutions.shift() : (JSON.parse(options.body).mode === 'iframe' ? { mode: 'iframe', progress: 120 } : fresh(120, JSON.parse(options.body).audioOnly));
      if (next instanceof Error) throw next;
      return { ok: true, json: async () => next };
    }
    return { ok: true };
  };
  const options = { video: 'one', channel: 'channel', progress: 1, volume: 50, authenticated, withProgress: authenticated, returnURL: '/', segments: [] };
  if (legacyIframeChoice) win.sessionStorage.setItem('feedlr-player-mode:one', 'iframe');
  if (savedVolume !== undefined) win.localStorage.setItem('player-volume', savedVolume);
  if (queued) win.feedlrPendingPlayer = { options, root: staleQueue ? new Element('div') : elements.get('player') };
  vm.runInNewContext(fs.readFileSync(require.resolve('./feedlr-player.js'), 'utf8'), { window: win, document: doc, fetch, performance, AbortController, URL, console });
  const controller = queued ? win.feedlr_player : win.FeedlrPlayer.mount(options);
  return { controller, win, doc, timers, requests, players, elements, order, fresh, event, toastText };
}

async function settle() { for (let i = 0; i < 6; i++) await new Promise(setImmediate); }

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

test('TV action appears for an online paired screen and waits for a ready player', async () => {
  const h = harness({ tvStatuses: [{ online: false }, { online: true, screenName: 'Living room' }] });
  const button = h.elements.get('send-to-tv-btn');
  assert.equal(button.hidden, true);
  await h.controller.start;
  assert.equal(button.hidden, true);
  const poll = h.timers.get(h.controller.tvTimer);
  assert.equal(poll.ms, 15000);
  await poll.callback();
  assert.equal(button.hidden, false);
  assert.equal(button.disabled, false);
  assert.equal(button['aria-label'], 'Send to TV (Living room)');
  assert.equal(button.title, button['aria-label']);
  const switching = h.controller.switchPlayer('iframe', h.controller.snapshot(), 'manual_switch');
  assert.equal(button.disabled, true);
  await h.controller.sendToTV();
  assert.equal(h.requests.some((request) => request.url.endsWith('/tv')), false);
  await switching;
  assert.equal(button.disabled, false);
  h.controller.cleanup();
});

test('TV polling pauses when hidden or offline and refreshes on visibility, pageshow and reconnect', async () => {
  const h = harness();
  await h.controller.start;
  const statusCount = () => h.requests.filter((request) => request.url === '/api/tv/status').length;
  assert.equal(statusCount(), 1);
  h.doc.hidden = true;
  h.doc.dispatchEvent(h.event('visibilitychange'));
  await h.controller.refreshTVStatus();
  assert.equal(h.controller.tvTimer, null);
  assert.equal(statusCount(), 1);
  h.doc.hidden = false;
  h.doc.dispatchEvent(h.event('visibilitychange'));
  await settle();
  assert.equal(statusCount(), 2);
  h.win.dispatchEvent(h.event('pageshow', { persisted: false }));
  await settle();
  assert.equal(statusCount(), 3);
  h.win.navigator.onLine = false;
  h.win.dispatchEvent(h.event('offline'));
  await h.controller.refreshTVStatus();
  assert.equal(h.controller.tvTimer, null);
  assert.equal(statusCount(), 3);
  h.win.navigator.onLine = true;
  h.win.dispatchEvent(h.event('online'));
  await settle();
  assert.equal(statusCount(), 4);
  h.controller.cleanup();
  h.win.dispatchEvent(h.event('online'));
  h.doc.dispatchEvent(h.event('visibilitychange'));
  await settle();
  assert.equal(statusCount(), 4);
  assert.equal(h.timers.size, 0);
});

test('failed TV status checks hide a previously available action', async () => {
  const h = harness({ tvStatuses: [{ online: true }, { ok: false }, new Error('offline')] });
  await h.controller.start;
  const button = h.elements.get('send-to-tv-btn');
  assert.equal(button.hidden, false);
  await h.controller.refreshTVStatus();
  assert.equal(button.hidden, true);
  await h.controller.refreshTVStatus();
  assert.equal(button.hidden, true);
  h.controller.cleanup();
});

for (const mode of ['native', 'iframe']) {
  test(`${mode} TV handoff sends the current position, pauses on success and protects TV progress`, async () => {
    const send = deferred();
    const h = harness({ tvStatuses: [{ online: true }], tvSends: [send.promise], resolutions: mode === 'iframe' ? [{ mode: 'iframe', progress: 120 }] : [] });
    await h.controller.start;
    const button = h.elements.get('send-to-tv-btn');
    h.controller.seekTo(321.75);
    const progressCount = () => h.requests.filter((request) => request.url.includes('/progress?')).length;
    const savedBefore = progressCount();
    const sending = h.controller.sendToTV();
    assert.equal(button.disabled, true);
    assert.equal(button['aria-busy'], 'true');
    await settle();
    assert.equal(h.controller.snapshot().playing, true);
    const request = h.requests.find((item) => item.url.endsWith('/tv'));
    assert.equal(request.method, 'POST');
    assert.deepEqual(JSON.parse(request.body), { position: 321.75 });
    await h.controller.sendToTV();
    assert.equal(h.requests.filter((item) => item.url.endsWith('/tv')).length, 1);
    h.controller.seekTo(323.25);
    h.controller.saveProgress();
    assert.equal(progressCount(), savedBefore);
    send.resolve({ ok: true, status: 204 });
    await sending;
    assert.equal(h.controller.snapshot().playing, false);
    assert.equal(h.toastText.textContent, 'Sent to TV');
    assert.equal(button.disabled, false);
    assert.equal(button['aria-busy'], 'false');
    h.controller.saveProgress();
    // Seeking while paused must also leave the TV's newer progress alone.
    h.controller.seekTo(400);
    h.controller.saveProgress();
    assert.equal(progressCount(), savedBefore);
    h.controller.cleanup();
    await settle();
    assert.equal(progressCount(), savedBefore);
    assert.equal(h.timers.size, 0);
  });

  test(`${mode} progress resumes if the user plays locally after sending to TV`, async () => {
    const h = harness({ tvStatuses: [{ online: true }], resolutions: mode === 'iframe' ? [{ mode: 'iframe', progress: 120 }] : [] });
    await h.controller.start;
    await h.controller.sendToTV();
    h.controller.seekTo(450);
    h.controller.playVideo();
    h.controller.saveProgress();
    assert.equal(h.requests.some((request) => request.url.includes('/progress?progress=450&')), true);
    h.controller.cleanup();
  });
}

test('TV handoff waits for older progress writes and captures position when it sends', async () => {
  const progress = deferred();
  const h = harness({ tvStatuses: [{ online: true }], progressResponses: [progress.promise] });
  await h.controller.start;
  h.controller.seekTo(300);
  h.controller.saveProgress();
  const sending = h.controller.sendToTV();
  await settle();
  assert.equal(h.requests.some((request) => request.url.endsWith('/tv')), false);
  h.controller.seekTo(301.5);
  progress.resolve({ ok: true });
  await sending;
  const request = h.requests.find((item) => item.url.endsWith('/tv'));
  assert.deepEqual(JSON.parse(request.body), { position: 301.5 });
  assert.equal(h.controller.snapshot().playing, false);
  h.controller.cleanup();
});

test('TV handoff failure keeps local playback and progress active with a retryable error', async () => {
  const h = harness({ tvStatuses: [{ online: true }, { online: true }], tvSends: [{ ok: false, json: async () => ({ error: 'The connected TV is offline' }) }] });
  await h.controller.start;
  await h.controller.sendToTV();
  await settle();
  assert.equal(h.controller.snapshot().playing, true);
  assert.equal(h.toastText.textContent, 'The connected TV is offline');
  assert.equal(h.elements.get('send-to-tv-btn').disabled, false);
  h.controller.seekTo(500);
  h.controller.saveProgress();
  assert.equal(h.requests.some((request) => request.url.includes('/progress?progress=500&')), true);
  await h.controller.sendToTV();
  assert.equal(h.controller.snapshot().playing, false);
  assert.equal(h.toastText.textContent, 'Sent to TV');
  h.controller.cleanup();
});

test('TV handoff rejects a successful HTTP response from an expired-session login redirect', async () => {
  const h = harness({ tvStatuses: [{ online: true }], tvSends: [{ ok: true, status: 200, redirected: true, json: async () => { throw new SyntaxError('HTML login page'); } }] });
  await h.controller.start;
  await h.controller.sendToTV();
  assert.equal(h.controller.snapshot().playing, true);
  assert.equal(h.controller.tvHandedOff, false);
  assert.equal(h.toastText.textContent, 'Could not send video to TV. Try again.');
  h.controller.cleanup();
});

test('navigation aborts TV status and ignores a late response after remount', async () => {
  const status = deferred();
  const h = harness({ tvStatuses: [status.promise, { online: true, screenName: 'New TV' }] });
  await h.controller.start;
  const oldRequest = h.requests.find((request) => request.url === '/api/tv/status');
  const again = h.win.FeedlrPlayer.mount(h.controller.options);
  await again.start;
  assert.equal(oldRequest.signal.aborted, true);
  status.resolve({ online: false });
  await settle();
  assert.equal(h.elements.get('send-to-tv-btn').hidden, false);
  assert.equal(h.elements.get('send-to-tv-btn').title, 'Send to TV (New TV)');
  again.cleanup();
  await settle();
  assert.equal(h.timers.size, 0);
});

test('navigation aborts an in-flight handoff without pausing a newly mounted player', async () => {
  const send = deferred();
  const h = harness({ tvStatuses: [{ online: true }, { online: true }], tvSends: [send.promise] });
  await h.controller.start;
  const sending = h.controller.sendToTV();
  await settle();
  const request = h.requests.find((item) => item.url.endsWith('/tv'));
  const again = h.win.FeedlrPlayer.mount(h.controller.options);
  await again.start;
  assert.equal(request.signal.aborted, true);
  send.resolve({ ok: true, status: 204 });
  await sending;
  assert.equal(again.snapshot().playing, true);
  assert.notEqual(h.toastText.textContent, 'Sent to TV');
  assert.equal(h.elements.get('send-to-tv-btn').hidden, false);
  again.cleanup();
  await settle();
  assert.equal(h.timers.size, 0);
});

test('handoff timeout while waiting for progress preserves local playback', async () => {
  const progress = deferred();
  const h = harness({ tvStatuses: [{ online: true }, { online: true }], progressResponses: [progress.promise] });
  await h.controller.start;
  h.controller.saveProgress();
  const sending = h.controller.sendToTV();
  h.timers.get(h.controller.tvSendTimeout).callback();
  await sending;
  assert.equal(h.controller.snapshot().playing, true);
  assert.equal(h.requests.some((request) => request.url.endsWith('/tv')), false);
  assert.equal(h.toastText.textContent, 'Could not send video to TV. Try again.');
  progress.resolve({ ok: true });
  h.controller.cleanup();
});

test('successful TV handoff survives a concurrent native renewal with delayed autoplay', async () => {
  const send = deferred();
  const loading = deferred();
  const h = harness({ tvStatuses: [{ online: true }], tvSends: [send.promise], shakaLoads: [Promise.resolve(), loading.promise], nativeAutoplay: true });
  await h.controller.start;
  h.controller.seekTo(300);
  const sending = h.controller.sendToTV();
  await settle();
  const switching = h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  await settle();
  send.resolve({ ok: true, status: 204 });
  await sending;
  loading.resolve();
  await switching;
  assert.equal(h.controller.snapshot().playing, false);
  assert.equal(h.controller.tvHandedOff, true);
  assert.equal(h.controller.adapter.video.autoplay, false);
  h.controller.saveProgress();
  h.controller.cleanup();
  assert.equal(h.requests.some((request) => request.url.includes('/progress?')), false);
});

test('a pending iframe switch preserves the successful TV handoff', async () => {
  const send = deferred();
  const h = harness({ tvStatuses: [{ online: true }], tvSends: [send.promise] });
  await h.controller.start;
  const sending = h.controller.sendToTV();
  await settle();
  const switching = h.controller.switchPlayer('iframe', h.controller.snapshot(), 'manual_switch');
  send.resolve({ ok: true, status: 204 });
  await sending;
  await switching;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.snapshot().playing, false);
  assert.equal(h.controller.tvHandedOff, true);
  h.controller.seekTo(600);
  h.controller.saveProgress();
  assert.equal(h.requests.some((request) => request.url.includes('/progress?')), false);
  h.controller.playVideo();
  assert.equal(h.controller.tvHandedOff, false);
  assert.equal(h.controller.snapshot().playing, true);
  h.controller.cleanup();
});

test('a failed handoff lets a pending player switch resume the latest local position', async () => {
  const send = deferred();
  const h = harness({ tvStatuses: [{ online: true }, { online: true }], tvSends: [send.promise] });
  await h.controller.start;
  h.controller.seekTo(400);
  const sending = h.controller.sendToTV();
  await settle();
  const switching = h.controller.switchPlayer('iframe', h.controller.snapshot(), 'manual_switch');
  h.controller.seekTo(410);
  send.resolve({ ok: false, json: async () => ({ error: 'TV disconnected' }) });
  await sending;
  await switching;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.snapshot().playing, true);
  assert.equal(h.controller.getCurrentTime(), 410);
  assert.equal(h.controller.tvHandedOff, false);
  h.controller.cleanup();
});

test('scheduled renewal waits for a TV send and remains available after failure', async () => {
  const send = deferred();
  const h = harness({ tvStatuses: [{ online: true }, { online: true }], tvSends: [send.promise] });
  await h.controller.start;
  const sending = h.controller.sendToTV();
  h.controller.renewAt = 0;
  h.controller.checkRenewal();
  h.controller.checkRenewal();
  assert.equal(h.controller.renewAt, 0);
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback')).length, 1);
  send.resolve({ ok: false, json: async () => ({ error: 'TV disconnected' }) });
  await sending;
  h.controller.checkRenewal();
  await settle();
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback')).length, 2);
  assert.equal(h.controller.snapshot().playing, true);
  h.controller.cleanup();
});

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

test('guests skip resolution and native scripts', async () => {
  const h = harness({ authenticated: false });
  await h.controller.start;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.requests.length, 0);
  assert.equal(h.players.length, 0);
  h.controller.cleanup();
});

test('settings fallback preserves state for this page; remount retries native despite legacy preferences', async () => {
  const signed = harness({ legacyIframeChoice: true });
  await signed.controller.start;
  assert.equal(signed.controller.mode, 'native');
  const video = signed.controller.adapter.video;
  video.currentTime = 321.75; video.volume = 0.35; video.muted = true; video.playbackRate = 1.5; video.pause();
  const ui = signed.players[0].ui;
  const button = ui.children[0].button;
  button.dispatchEvent(signed.event('click'));
  await settle();
  assert.equal(ui.menuHidden, true);
  assert.equal(signed.controller.mode, 'iframe');
  assert.deepEqual(JSON.parse(JSON.stringify(signed.controller.snapshot())), { position: 321.75, playing: false, volume: 35, muted: true, rate: 1.5 });
  signed.controller.tick();
  signed.controller.checkRenewal();
  assert.equal(signed.controller.mode, 'iframe');
  const again = signed.win.FeedlrPlayer.mount(signed.controller.options);
  await again.start;
  assert.equal(again.mode, 'native');
  button.dispatchEvent(signed.event('click')); // Released controls cannot switch a new player.
  await settle();
  assert.equal(again.mode, 'native');
  again.cleanup();
});

test('server-side failure stays on iframe until a fresh mount checks native again', async () => {
  const h = harness({ resolutions: [{ mode: 'iframe', reason: 'health_failed', progress: 456 }] });
  await h.controller.start;
  assert.equal(h.controller.mode, 'iframe');
  assert.equal(h.controller.getCurrentTime(), 456);
  h.controller.tick();
  h.controller.checkRenewal();
  assert.equal(h.requests.filter((request) => request.url.endsWith('/playback')).length, 1);
  const again = h.win.FeedlrPlayer.mount(h.controller.options);
  await again.start;
  assert.equal(again.mode, 'native');
  assert.deepEqual(h.requests.filter((request) => request.url.endsWith('/playback')).map((request) => JSON.parse(request.body).mode), ['native', 'native']);
  again.cleanup();
});

test('iframe action hides inside quality submenus and releases on UI reconfiguration', async () => {
  const h = harness();
  await h.controller.start;
  const ui = h.players[0].ui;
  const button = ui.children[0].button;
  ui.controls.dispatchEvent(h.event('submenuopen'));
  assert.equal(button.classList.contains('shaka-hidden'), true);
  ui.controls.dispatchEvent(h.event('submenuclose'));
  assert.equal(button.classList.contains('shaka-hidden'), false);
  ui.configure(ui.config);
  button.dispatchEvent(h.event('click'));
  await settle();
  assert.equal(h.controller.mode, 'native');
  ui.children[0].button.dispatchEvent(h.event('click'));
  await settle();
  assert.equal(h.controller.mode, 'iframe');
  h.controller.cleanup();
});

test('mobile uses device volume and omits native mute/volume controls across switches', async () => {
  const h = harness({ mobile: true, savedVolume: '25' });
  await h.controller.start;
  const ui = h.players[0].ui;
  assert.equal(ui.config.controlPanelElements.includes('mute'), false);
  assert.equal(ui.config.controlPanelElements.includes('volume'), false);
  assert.equal(h.controller.adapter.video.volume, 1);
  h.controller.setVolume(15);
  assert.equal(h.controller.getVolume(), 100);
  ui.children[0].button.dispatchEvent(h.event('click'));
  await settle();
  assert.equal(h.controller.mode, 'iframe');
  h.controller.setVolume(10);
  assert.equal(h.controller.getVolume(), 100);
  h.controller.saveVolume();
  assert.equal(h.win.localStorage.getItem('player-volume'), '25');
  h.controller.cleanup();
});

test('desktop retains mute and volume controls and its saved volume', async () => {
  const h = harness({ savedVolume: '25' });
  await h.controller.start;
  assert.equal(h.players[0].ui.config.controlPanelElements.includes('mute'), true);
  assert.equal(h.players[0].ui.config.controlPanelElements.includes('volume'), true);
  assert.equal(h.players[0].ui.config.alwaysShowVolumeBar, true);
  assert.equal(h.controller.getVolume(), 25);
  h.controller.setVolume(15);
  assert.equal(h.controller.getVolume(), 15);
  h.controller.cleanup();
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

test('native audio starts original regardless of language, track order, or channel count', async () => {
  for (const [originalLanguage, dubLanguage, channelsCount] of [['ja', 'en', 2], ['en', 'ja', 6], ['uk', 'uk', 2]]) {
    const h = harness({ nativeAutoplay: true, audioTracks: [
      { language: dubLanguage, label: 'Dub', roles: ['alternate', 'dub'], channelsCount: 2, active: true },
      { language: originalLanguage, label: 'Source', roles: ['main'], channelsCount, active: false },
    ] });
    await h.controller.start;
    const player = h.players[0];
    assert.equal(h.controller.mode, 'native');
    assert.equal(player.autoplayAtLoad, false, 'audio must be checked before autoplay');
    assert.deepEqual(JSON.parse(JSON.stringify(player.audioPreferencesAtLoad)), [{ role: 'main', channelCount: 2 }, { role: 'main' }]);
    assert.equal(player.getAudioTracks().find((track) => track.active).label, 'Source');
    assert.equal(player.video.paused, false);
    assert.equal(player.getAudioTracks().length, 2, 'dubs remain selectable');
    assert.equal(player.ui.config.overflowMenuButtons.includes('language'), true);
    assert.equal(player.ui.config.trackLabelFormat, 3);
    h.controller.cleanup();
  }
});

test('native playback does not autoplay a dub when no playable original is identified', async () => {
  for (const audioTracks of [
    [{ language: 'en', roles: ['alternate', 'dub'], active: true }],
    [{ language: 'en', roles: [], active: true }, { language: 'ja', roles: [], active: false }],
    [{ language: 'en', label: 'One', roles: [], active: true }, { language: 'en', label: 'Two', roles: [], active: false }],
    [{ language: 'en', roles: ['main', 'dub'], active: true }],
  ]) {
    const h = harness({ nativeAutoplay: true, audioTracks });
    await h.controller.start;
    assert.equal(h.controller.mode, 'iframe');
    assert.equal(h.players[0].autoplayAtLoad, false);
    assert.equal(h.players[0].video.paused, true);
    h.controller.cleanup();
  }
});

test('explicit audio choice survives renewal and audio-only switches but resets for a new page', async () => {
  const h = harness({ audioTracks: [
    { language: 'en', label: 'English dub', roles: ['alternate', 'dub'], channelsCount: 2, active: true },
    { language: 'ja', label: 'Japanese original', roles: ['main'], channelsCount: 2, active: false },
  ] });
  await h.controller.start;
  const selected = () => h.controller.adapter.player.getAudioTracks().find((track) => track.active);
  assert.equal(selected().language, 'ja');
  const player = h.controller.adapter.player;
  chooseAudio(player, 0);
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  assert.equal(selected().label, 'English dub');
  assert.equal(h.players.at(-1).audioPreferencesAtLoad[0].language, 'en');
  await h.controller.selectQuality('audio');
  assert.equal(selected().label, 'English dub');
  await h.controller.selectQuality('720');
  assert.equal(selected().label, 'English dub');
  const again = h.win.FeedlrPlayer.mount({ ...h.controller.options, video: 'two' });
  await again.start;
  assert.equal(again.adapter.player.getAudioTracks().find((track) => track.active).language, 'ja');
  again.cleanup();
});

test('a missing saved dub returns to original while ordinary roleless audio remains playable', async () => {
  const audioTracks = [
    { language: 'en', label: 'Dub', roles: ['alternate', 'dub'], active: true },
    { language: 'ja', label: 'Source', roles: ['main'], active: false },
  ];
  const h = harness({ audioTracks });
  await h.controller.start;
  chooseAudio(h.controller.adapter.player, 0);
  audioTracks.shift();
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).label, 'Source');
  h.controller.cleanup();
  const legacy = harness({ audioTracks: [{ language: 'fr', roles: [], active: true }] });
  await legacy.controller.start;
  assert.equal(legacy.controller.mode, 'native');
  assert.equal(legacy.controller.adapter.video.paused, false);
  legacy.controller.cleanup();
});

test('an automatic legacy track never becomes a language override on renewal', async () => {
  const audioTracks = [{ language: 'en', roles: [], channelsCount: 2, active: true }];
  const h = harness({ audioTracks });
  await h.controller.start;
  assert.equal(h.controller.mode, 'native');
  audioTracks.splice(0, 1,
    { language: 'en', label: 'Dub', roles: ['alternate', 'dub'], channelsCount: 2, active: true },
    { language: 'en', label: 'Source', roles: ['main'], channelsCount: 2, active: false });
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  assert.equal(h.controller.audioPreference, null);
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).label, 'Source');
  chooseAudio(h.controller.adapter.player, 0);
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  assert.equal(h.controller.audioPreference.role, 'dub');
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).label, 'Dub');
  h.controller.cleanup();
});

test('regular original beats processed stereo; explicit codec choices survive renewal', async () => {
  const audioTracks = [
    { language: 'en', label: 'English (Original)', roles: ['main', 'enhanced-audio-intelligibility'], channelsCount: 2, codecs: 'mp4a.40.2', active: true },
    { language: 'en', label: 'English (Original)', roles: ['main'], channelsCount: 6, codecs: 'ec-3', active: false },
  ];
  const h = harness({ audioTracks });
  await h.controller.start;
  assert.equal(h.controller.mode, 'native');
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).codecs, 'ec-3');
  // Stable volume remains usable when its codec is the viewer's explicit choice.
  chooseAudio(h.controller.adapter.player, 0);
  // On renewal both encodings are stereo and Shaka initially activates regular
  // audio: restoring the explicit choice must distinguish them by codec.
  audioTracks[0].active = false;
  audioTracks[1].active = true;
  audioTracks[1].channelsCount = 2;
  await h.controller.switchPlayer('native', h.controller.snapshot(), 'renewal');
  assert.equal(h.controller.audioPreference.codec, 'mp4a.40.2');
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).codecs, 'mp4a.40.2');
  await h.controller.selectQuality('audio');
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).codecs, 'mp4a.40.2');
  h.controller.cleanup();
});

test('original available only with stable volume remains playable', async () => {
  const h = harness({ audioTracks: [
    { language: 'en', label: 'English (auto-dubbed)', roles: ['alternate', 'dub'], active: true },
    { language: 'ja', label: 'Japanese (Original)', roles: ['main', 'enhanced-audio-intelligibility'], active: false },
  ] });
  await h.controller.start;
  assert.equal(h.controller.mode, 'native');
  assert.equal(h.controller.adapter.player.getAudioTracks().find((track) => track.active).language, 'ja');
  h.controller.cleanup();
});
