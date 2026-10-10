const test = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const template = fs.readFileSync(path.join(__dirname, '../../internal/templates/pages/podcast.templ'), 'utf8');
const script = template.match(/script podcastPlayerInit\([^)]*\)\s*\{([\s\S]*)\n\}\s*$/);
assert.ok(script, 'podcastPlayerInit script exists in the template');

const sponsor = { category: 'sponsor', start_ms: 10000, end_ms: 20000, start_text: 'Sponsor start', end_text: 'Sponsor end', skippable: true };
const ready = (overrides = {}) => ({ enabled: true, status: 'ready', phase: '', duration_ms: 600000, source: 'generated', error: '', segments: [sponsor], ...overrides });

function harness({ initialSegments = { enabled: true, status: 'idle', segments: [] }, responses = [], duration = 600, currentTime = 0, readyState = 0, paused = true } = {}) {
  const requests = [];
  const timers = new Map();
  let nextTimer = 1;
  class Element extends EventTarget {
    constructor(tag = 'div') {
      super();
      this.tagName = tag;
      this.children = [];
      this.style = {};
      this.dataset = {};
      this.textContent = '';
      const classes = new Set();
      this.classList = {
        add: (name) => classes.add(name),
        remove: (name) => classes.delete(name),
        contains: (name) => classes.has(name),
        toggle: (name, force) => { if (force ?? !classes.has(name)) classes.add(name); else classes.delete(name); },
      };
    }
    append(...children) { this.children.push(...children); }
    replaceChildren(...children) { this.children = children; }
    setAttribute(key, value) { this[key] = value; }
    removeAttribute(key) { delete this[key]; }
    contains(element) { return this === element || this.children.some((child) => child.contains(element)); }
    closest() { return null; }
    load() {}
    pause() { if (!this.paused) { this.paused = true; this.dispatchEvent(new Event('pause')); } }
    async play() { this.paused = false; this.dispatchEvent(new Event('play')); this.dispatchEvent(new Event('playing')); }
  }
  const root = new Element();
  const audio = Object.assign(new Element('audio'), { duration, currentTime, readyState, paused, ended: false, volume: 1, playbackRate: 1 });
  const selectors = [
    '#podcast-segments-tab', '#podcast-notes-tab', '#podcast-notes-panel', '#podcast-segments-panel',
    '[data-podcast-segment-count]', '[data-podcast-segment-spinner]', '#podcast-segment-markers',
    '#podcast-progress', '#podcast-elapsed', '#podcast-remaining', '#podcast-play-toggle',
    '[data-podcast-loading-icon]', '[data-podcast-play-icon]', '[data-podcast-pause-icon]',
    '#podcast-speed', '#podcast-volume',
  ];
  const elements = new Map(selectors.map((selector) => [selector, new Element()]));
  root.querySelector = (selector) => elements.get(selector) || null;
  const skipButtons = [-15, 15].map((seconds) => {
    const button = new Element('button');
    button.dataset.podcastSkip = String(seconds);
    return button;
  });
  root.querySelectorAll = (selector) => selector === '[data-podcast-skip]' ? skipButtons : [];
  root.append(audio, ...elements.values());
  const close = new Element('button');
  const doc = Object.assign(new EventTarget(), {
    getElementById: (id) => id === 'podcast-player-page' ? root : id === 'podcast-player' ? audio : id === 'close-button' ? close : null,
    querySelectorAll: () => [audio],
    createElement: (tag) => new Element(tag),
  });
  const values = new Map();
  const win = Object.assign(new EventTarget(), {
    localStorage: { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)) },
    location: { origin: 'https://feedlr.test' },
    setTimeout: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms, type: 'timeout' }); return id; },
    clearTimeout: (id) => timers.delete(id),
    setInterval: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms, type: 'interval' }); return id; },
    clearInterval: (id) => timers.delete(id),
  });
  const fetch = async (url, options = {}) => {
    requests.push({ url, method: options.method || 'GET', ...options });
    if (!url.endsWith('/sponsor-segments')) return { ok: true };
    const next = responses.length ? await responses.shift() : { status: 'running', phase: 'preparing', segments: [] };
    if (next instanceof Error) throw next;
    if (next.ok !== undefined) return next;
    return { ok: true, json: async () => next };
  };
  const initialize = vm.runInNewContext(`(function(episode, title, channel, progress, withProgress, initialSegments) {${script[1]}\n})`, {
    window: win, document: doc, navigator: {}, HTMLMediaElement: { HAVE_METADATA: 1, HAVE_FUTURE_DATA: 3 }, Element, AbortController, URL, fetch,
  });
  initialize('episode-one', 'Episode', 'Podcast', 0, false, initialSegments);
  const text = (node) => [node.textContent, ...node.children.map(text)].filter(Boolean).join(' ');
  return {
    audio, win, doc, elements, timers, requests, skipButtons,
    reopen: () => {
      win.feedlrPodcastPlayer.cleanup();
      audio.currentTime = 0;
      initialize('episode-one', 'Episode', 'Podcast', 0, false, initialSegments);
    },
    text: () => text(elements.get('#podcast-segments-panel')),
    emit: (type) => audio.dispatchEvent(new Event(type)),
    cleanup: () => win.feedlrPodcastPlayer.cleanup(),
    async poll() {
      const [id, timer] = [...timers].find(([, value]) => value.type === 'timeout') || [];
      assert.ok(timer, 'analysis poll is scheduled');
      timers.delete(id);
      timer.callback();
      await settle();
      return timer.ms;
    },
  };
}

async function settle() { for (let i = 0; i < 4; i++) await new Promise(setImmediate); }

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

test('opening a podcast waits for playing, then starts once across buffering and resume', async () => {
  const h = harness({ paused: false, readyState: 1, responses: [{ status: 'running', phase: 'scanning', poll_after_ms: 750, segments: [] }, ready({ segments: [] })] });
  h.emit('loadedmetadata');
  h.emit('canplay');
  h.emit('play');
  await settle();
  assert.equal(h.requests.length, 0);
  assert.match(h.text(), /starts when playback begins/);
  h.emit('playing');
  await settle();
  assert.deepEqual(h.requests.map((request) => request.method), ['POST']);
  assert.match(h.text(), /Scanning/);
  h.emit('playing');
  h.audio.pause();
  await h.audio.play();
  await settle();
  assert.equal(h.requests.filter((request) => request.method === 'POST').length, 1);
  assert.equal(await h.poll(), 750);
  assert.deepEqual(h.requests.map((request) => request.method), ['POST', 'GET']);
  assert.equal(h.timers.size, 0);
  h.cleanup();
});

for (const reason of ['unauthenticated', 'disabled settings']) {
  test(`${reason} prevents kickoff, polling, and automatic skipping`, async () => {
    const h = harness({ initialSegments: ready({ enabled: false }), currentTime: 12, readyState: 1 });
    await h.audio.play();
    h.emit('timeupdate');
    await settle();
    assert.equal(h.requests.length, 0);
    assert.equal(h.timers.size, 0);
    assert.equal(h.audio.currentTime, 12);
    assert.equal(h.elements.get('#podcast-segments-tab').hidden, true);
    h.cleanup();
  });
}

test('a cached active analysis polls with GET and shows preparation and scanning phases', async () => {
  const h = harness({ initialSegments: { enabled: true, status: 'running', phase: 'preparing', segments: [] }, responses: [{ status: 'running', phase: 'scanning', segments: [] }, ready({ segments: [] })] });
  assert.match(h.text(), /Preparing episode transcript/);
  await settle();
  assert.match(h.text(), /Scanning/);
  assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), false);
  await h.poll();
  assert.deepEqual(h.requests.map((request) => request.method), ['GET', 'GET']);
  assert.match(h.text(), /No selected episode segments/);
  assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), true);
  h.cleanup();
});

test('cached ready segments wait for first-playing validation while audio keeps playing', async () => {
  const validation = deferred();
  const h = harness({ initialSegments: ready(), responses: [validation.promise], currentTime: 12, readyState: 1 });
  h.emit('loadedmetadata');
  h.emit('durationchange');
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  assert.equal(h.requests.length, 0);
  assert.match(h.text(), /Sponsor start/);
  assert.match(h.text(), /after the episode audio is checked/);
  h.emit('canplay');
  await h.audio.play();
  assert.equal(h.audio.paused, false);
  assert.equal(h.elements.get('#podcast-play-toggle').disabled, false);
  assert.match(h.text(), /Checking episode audio/);
  h.audio.currentTime = 13;
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 13);
  validation.resolve(ready());
  await settle();
  assert.equal(h.audio.currentTime, 20);
  assert.equal(h.audio.paused, false);
  assert.deepEqual(h.requests.map((request) => request.method), ['POST']);
  h.cleanup();
});

test('GET cache polling cannot validate cached segments for automatic skipping', async () => {
  const h = harness({ initialSegments: { enabled: true, status: 'running', phase: 'scanning', segments: [] }, responses: [ready(), ready()], currentTime: 12 });
  await settle();
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  assert.deepEqual(h.requests.map((request) => request.method), ['GET']);
  await h.audio.play();
  await settle();
  assert.equal(h.audio.currentTime, 20);
  assert.deepEqual(h.requests.map((request) => request.method), ['GET', 'POST']);
  h.cleanup();
});

test('failed validation keeps cached sponsor times from skipping and preserves playback', async () => {
  for (const response of [{ status: 'failed', error: 'source_validation_failed', segments: [sponsor] }, { status: 'unavailable', error: 'audio_unavailable', segments: [sponsor] }, { ok: false, status: 502 }, new Error('validation offline')]) {
    const validation = deferred();
    const h = harness({ initialSegments: ready(), responses: [validation.promise], currentTime: 12, readyState: 1 });
    await h.audio.play();
    h.emit('timeupdate');
    assert.equal(h.audio.currentTime, 12);
    validation.resolve(response);
    await settle();
    h.emit('timeupdate');
    h.emit('durationchange');
    assert.equal(h.audio.currentTime, 12);
    assert.equal(h.audio.paused, false);
    assert.doesNotMatch(h.text(), /No selected episode segments/);
    assert.equal(h.timers.size, 0);
    h.audio.pause();
    await h.audio.play();
    await settle();
    assert.deepEqual(h.requests.map((request) => request.method), ['POST']);
    h.cleanup();
  }
});

test('a pending replacement job suppresses old segments until its current result is ready', async () => {
  const h = harness({ initialSegments: ready(), responses: [{ status: 'running', phase: 'preparing', segments: [sponsor] }, ready({ segments: [{ ...sponsor, start_ms: 11000, end_ms: 25000 }] })], currentTime: 12 });
  await h.audio.play();
  await settle();
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  assert.equal(h.audio.paused, false);
  await h.poll();
  assert.equal(h.audio.currentTime, 25);
  assert.deepEqual(h.requests.map((request) => request.method), ['POST', 'GET']);
  h.cleanup();
});

for (const status of ['failed', 'ready']) {
  test(`first playing POSTs once when the cached analysis is ${status}`, async () => {
    const h = harness({ initialSegments: ready({ status, segments: [] }), responses: [ready({ segments: [] })] });
    assert.equal(h.requests.length, 0);
    h.emit('playing');
    h.emit('playing');
    await settle();
    assert.deepEqual(h.requests.map((request) => request.method), ['POST']);
    h.cleanup();
  });
}

test('failed and unavailable analyses are distinct from a completed empty result', () => {
  for (const [status, pattern] of [['failed', /analysis failed/], ['unavailable', /analysis is unavailable/], ['ready', /No selected episode segments/]]) {
    const h = harness({ initialSegments: ready({ status, error: 'transcript_fetch_failed', segments: [] }) });
    assert.match(h.text(), pattern);
    assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), true);
    assert.equal(h.elements.get('[data-podcast-segment-count]').classList.contains('hidden'), status !== 'ready');
    h.cleanup();
  }
});

test('HTTP and network errors stop the spinner and report analysis failure', async () => {
  for (const response of [{ ok: false, status: 500 }, new Error('network offline')]) {
    const h = harness({ responses: [response] });
    h.emit('playing');
    await settle();
    assert.match(h.text(), /Could not load episode segment analysis/);
    assert.doesNotMatch(h.text(), /No selected episode segments/);
    assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), true);
    assert.equal(h.timers.size, 0);
    h.cleanup();
  }
});

test('validated generated segments use their timestamps regardless of reported total duration', async () => {
  for (const [duration, transcriptDuration] of [[600, 600000], [602, 600000], [598, 600000], [720, 600000], [600, 0], [600, NaN], [NaN, 600000], [Infinity, 600000]]) {
    const state = ready({ duration_ms: transcriptDuration });
    const h = harness({ initialSegments: state, responses: [state], duration, currentTime: 12 });
    await h.audio.play();
    await settle();
    h.emit('timeupdate');
    assert.equal(h.audio.currentTime, 20, `audio ${duration}, transcript ${transcriptDuration}`);
    assert.match(h.text(), /Sponsor start/);
    assert.doesNotMatch(h.text(), /Automatic skipping is paused/);
    h.cleanup();
  }
});

test('duration changes update the timeline without blocking skipping or discarding segments', async () => {
  const h = harness({ initialSegments: ready(), responses: [ready()], duration: 603, currentTime: 12 });
  await h.audio.play();
  await settle();
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 20);
  assert.doesNotMatch(h.text(), /audio length differs/);
  h.audio.duration = 600;
  h.emit('durationchange');
  assert.equal(h.audio.currentTime, 20);
  assert.doesNotMatch(h.text(), /Automatic skipping is paused/);
  h.audio.duration = 603;
  h.audio.currentTime = 12;
  h.emit('durationchange');
  assert.equal(h.audio.currentTime, 20);
  assert.match(h.text(), /Sponsor start/);
  h.cleanup();
});

test('validated publisher segments do not need a transcript duration for automatic skipping', async () => {
  const state = ready({ source: 'publisher', duration_ms: 0 });
  const h = harness({ initialSegments: state, responses: [state], duration: 603, currentTime: 12 });
  await h.audio.play();
  await settle();
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 20);
  assert.doesNotMatch(h.text(), /Automatic skipping is paused/);
  h.cleanup();
});

test('manual forward seeking into a sponsor still automatically skips it', async () => {
  const h = harness({ initialSegments: ready(), responses: [ready()] });
  await h.audio.play();
  await settle();
  const progress = h.elements.get('#podcast-progress');
  progress.value = '12';
  progress.dispatchEvent(new Event('input'));
  progress.dispatchEvent(new Event('change'));
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 20);
  h.cleanup();
});

test('rewinding into a sponsor disables skipping only for that segment and player session', async () => {
  const later = { ...sponsor, start_ms: 30000, end_ms: 40000 };
  const state = ready({ segments: [sponsor, later] });
  const h = harness({ initialSegments: state, responses: [state, state], currentTime: 12 });
  await h.audio.play();
  await settle();
  assert.equal(h.audio.currentTime, 20);
  const progress = h.elements.get('#podcast-progress');
  progress.value = '12';
  progress.dispatchEvent(new Event('input'));
  progress.dispatchEvent(new Event('change'));
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  h.audio.currentTime = 21;
  h.emit('timeupdate');
  h.audio.pause();
  await h.audio.play();
  h.audio.currentTime = 12;
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  h.audio.currentTime = 31;
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 40);
  h.reopen();
  await h.audio.play();
  await settle();
  progress.value = '12';
  progress.dispatchEvent(new Event('input'));
  progress.dispatchEvent(new Event('change'));
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 20);
  h.cleanup();
});

test('rewind controls opt out only when they land inside the segment', async () => {
  const h = harness({ initialSegments: ready(), responses: [ready()], currentTime: 12 });
  await h.audio.play();
  await settle();
  h.skipButtons[0].dispatchEvent(new Event('click'));
  assert.equal(h.audio.currentTime, 5);
  h.audio.currentTime = 12;
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 20);
  h.audio.currentTime = 28;
  h.skipButtons[0].dispatchEvent(new Event('click'));
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 13);
  h.audio.currentTime = 25;
  h.emit('timeupdate');
  h.audio.currentTime = 12;
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  h.cleanup();
});

test('a rewind override survives refreshed segment objects', async () => {
  const validation = deferred();
  const h = harness({ initialSegments: ready(), responses: [validation.promise], currentTime: 25 });
  await h.audio.play();
  await settle();
  const progress = h.elements.get('#podcast-progress');
  progress.value = '12';
  progress.dispatchEvent(new Event('input'));
  progress.dispatchEvent(new Event('change'));
  validation.resolve(ready({ segments: [{ ...sponsor }] }));
  await settle();
  assert.equal(h.audio.currentTime, 12);
  h.audio.currentTime = 21;
  h.emit('timeupdate');
  h.audio.currentTime = 12;
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  h.cleanup();
});

test('autoplay before initialization still validates and enables automatic skipping', async () => {
  const h = harness({ initialSegments: ready(), responses: [ready()], currentTime: 12, readyState: 3, paused: false });
  await settle();
  assert.deepEqual(h.requests.map((request) => request.method), ['POST']);
  assert.equal(h.audio.currentTime, 20);
  h.emit('playing');
  await settle();
  assert.equal(h.requests.length, 1);
  h.cleanup();
});

test('continuous forward scrubbing does not register a rewind override', async () => {
  const h = harness({ initialSegments: ready(), responses: [ready()] });
  await h.audio.play();
  await settle();
  const progress = h.elements.get('#podcast-progress');
  for (const time of [12, 13]) {
    progress.value = String(time);
    progress.dispatchEvent(new Event('input'));
    h.emit('timeupdate');
    assert.equal(h.audio.currentTime, time);
  }
  progress.dispatchEvent(new Event('change'));
  assert.equal(h.audio.currentTime, 20);
  h.cleanup();
});

test('segments extending past the actual audio end cannot cause repeated seeks', async () => {
  const h = harness({ initialSegments: ready(), responses: [ready()], duration: 15, currentTime: 12 });
  const seeks = [];
  let position = h.audio.currentTime;
  Object.defineProperty(h.audio, 'currentTime', {
    get: () => position,
    set: (time) => { seeks.push(time); position = Math.min(time, h.audio.duration); },
  });
  await h.audio.play();
  await settle();
  assert.equal(h.audio.currentTime, 15);
  for (let i = 0; i < 3; i++) h.emit('timeupdate');
  assert.deepEqual(seeks, [15]);
  h.cleanup();
});

test('a stale GET cannot replace the newer POST result or schedule another poll', async () => {
  const stale = deferred();
  const h = harness({ initialSegments: { enabled: true, status: 'running', phase: 'preparing', segments: [] }, responses: [stale.promise, ready()] });
  h.emit('playing');
  await settle();
  assert.match(h.text(), /Sponsor start/);
  stale.resolve({ status: 'running', phase: 'preparing', segments: [] });
  await settle();
  assert.match(h.text(), /Sponsor start/);
  assert.deepEqual(h.requests.map((request) => request.method), ['GET', 'POST']);
  assert.equal(h.timers.size, 0);
  h.cleanup();
});

test('cleanup aborts browser requests and prevents late responses and further polling', async () => {
  const pending = deferred();
  const h = harness({ responses: [pending.promise], currentTime: 12 });
  h.emit('playing');
  assert.equal(h.requests.length, 1);
  h.cleanup();
  assert.equal(h.requests[0].signal.aborted, true);
  pending.resolve(ready());
  await settle();
  h.emit('playing');
  assert.equal(h.requests.length, 1);
  assert.equal(h.audio.currentTime, 12);
  assert.equal(h.timers.size, 0);

  const polling = harness({ initialSegments: { enabled: true, status: 'running', phase: 'scanning', segments: [] } });
  await settle();
  const timer = [...polling.timers.values()][0];
  polling.cleanup();
  assert.equal(polling.timers.size, 0);
  timer.callback();
  await settle();
  assert.equal(polling.requests.length, 1);
});
