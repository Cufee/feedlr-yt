const test = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const template = fs.readFileSync(path.join(__dirname, '../../internal/templates/pages/podcast.templ'), 'utf8');
const script = template.match(/script podcastPlayerInit\([^)]*\)\s*\{([\s\S]*)\n\}\s*$/);
assert.ok(script, 'podcastPlayerInit script exists in the template');

const sponsor = { category: 'sponsor', start_ms: 10000, end_ms: 20000, start_text: 'Sponsor start', end_text: 'Sponsor end', skippable: true };
const ready = (overrides = {}) => {
  const state = { enabled: true, status: 'ready', phase: '', duration_ms: 600000, source: 'generated', error: '', transcript_ready: true, segments: [sponsor], ...overrides };
  return { has_segments: state.segments.length > 0, ...state };
};

function harness({ initialSegments = { enabled: true, status: 'idle', segments: [] }, responses = [], duration = 600, currentTime = 0, readyState = 0, paused = true } = {}) {
  const requests = [];
  const timers = new Map();
  let nextTimer = 1;
  let now = 0;
  class ClockDate extends Date { static now() { return now; } }
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
    async play() { this.playCalls = (this.playCalls || 0) + 1; this.paused = false; this.dispatchEvent(new Event('play')); if (!this.paused) this.dispatchEvent(new Event('playing')); }
  }
  const root = new Element();
  const audio = Object.assign(new Element('audio'), { duration, currentTime, readyState, paused, ended: false, volume: 1, playbackRate: 1 });
  const selectors = [
    '#podcast-segments-tab', '#podcast-notes-tab', '#podcast-notes-panel', '#podcast-segments-panel',
    '[data-podcast-segment-count]', '[data-podcast-segment-spinner]', '#podcast-segment-markers',
    '#podcast-progress', '#podcast-elapsed', '#podcast-remaining', '#podcast-play-toggle',
    '[data-podcast-loading-icon]', '[data-podcast-play-icon]', '[data-podcast-pause-icon]',
    '#podcast-speed', '#podcast-volume',
    '#podcast-scan-wait', '#podcast-scan-wait-skip',
  ];
  const elements = new Map(selectors.map((selector) => [selector, new Element()]));
  elements.get('#podcast-scan-wait').classList.add('hidden');
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
    setTimeout: (callback, ms) => { const id = nextTimer++; timers.set(id, { callback, ms, due: now + ms, type: 'timeout' }); return id; },
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
    window: win, document: doc, navigator: {}, HTMLMediaElement: { HAVE_METADATA: 1, HAVE_FUTURE_DATA: 3 }, Element, AbortController, URL, Date: ClockDate, fetch,
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
    waiting: () => !elements.get('#podcast-scan-wait').classList.contains('hidden'),
    async advance(ms) {
      const target = now + ms;
      for (let count = 0; count < 100; count++) {
        const [id, timer] = [...timers].filter(([, value]) => value.type === 'timeout' && value.due <= target).sort((a, b) => a[1].due - b[1].due)[0] || [];
        if (!timer) break;
        now = timer.due;
        timers.delete(id);
        timer.callback();
        await settle();
      }
      now = target;
      await settle();
    },
    async poll() {
      const [id, timer] = [...timers].filter(([, value]) => value.type === 'timeout').sort((a, b) => a[1].due - b[1].due)[0] || [];
      assert.ok(timer, 'analysis poll is scheduled');
      now = timer.due;
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

const newScan = (overrides = {}) => ready({ status: 'running', phase: 'scanning', transcript_ready: false, segments: [], ...overrides });

test('known missing transcript waits only after first playing and keeps loading across canplay', async () => {
  const pending = deferred();
  const h = harness({ initialSegments: newScan({ status: 'idle' }), responses: [pending.promise] });
  assert.equal(h.waiting(), false);
  assert.equal(h.requests.length, 0);
  h.emit('canplay');
  await h.audio.play();
  assert.equal(h.waiting(), true);
  assert.equal(h.audio.paused, true);
  assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'true');
  assert.equal(h.elements.get('#podcast-play-toggle').disabled, false, 'pause intent remains available during the hold');
  h.emit('canplay');
  assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'true');
  assert.equal([...h.timers.values()].filter((timer) => timer.ms === 30000).length, 1);
  h.cleanup();
  pending.resolve(newScan());
  await settle();
});

test('a complete cached transcript bypasses waiting while its sponsor scan runs', async () => {
  for (const initialSegments of [{ enabled: true, status: 'idle', segments: [] }, newScan({ status: 'idle', transcript_ready: true })]) {
    const h = harness({ initialSegments, responses: [newScan({ transcript_ready: true })] });
    await h.audio.play();
    await settle();
    assert.equal(h.audio.paused, false);
    assert.equal(h.waiting(), false);
    assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'false');
    assert.equal([...h.timers.values()].some((timer) => timer.ms === 30000), false);
    h.cleanup();
  }
});

test('initial cache-only GET readiness can activate waiting on first playing without an earlier kickoff', async () => {
  const pending = deferred();
  const h = harness({ initialSegments: { enabled: true, status: 'running', segments: [] }, responses: [newScan(), pending.promise] });
  await settle();
  assert.equal(h.waiting(), false);
  assert.deepEqual(h.requests.map((request) => request.method), ['GET']);
  await h.audio.play();
  assert.equal(h.waiting(), true);
  assert.equal(h.audio.paused, true);
  assert.deepEqual(h.requests.map((request) => request.method), ['GET', 'POST']);
  h.cleanup();
  pending.resolve(newScan());
  await settle();
});

test('the first confirmed detection releases waiting, including an unselected category', async () => {
  for (const detected of [newScan({ segments: [sponsor] }), newScan({ has_segments: true })]) {
    const h = harness({ responses: [newScan(), detected] });
    await h.audio.play();
    await settle();
    assert.equal(h.waiting(), true);
    assert.equal(h.audio.paused, true);
    await h.poll();
    assert.equal(h.waiting(), false);
    assert.equal(h.audio.paused, false);
    assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'false');
    assert.equal(h.requests.filter((request) => request.method === 'POST').length, 1);
    assert.equal([...h.timers.values()].some((timer) => timer.ms === 30000), false);
    h.cleanup();
  }
});

test('a transcript completed during an active hold still waits for a detection or the deadline', async () => {
  const h = harness({ responses: [newScan(), newScan({ transcript_ready: true }), newScan({ transcript_ready: true, has_segments: true })] });
  await h.audio.play();
  await settle();
  await h.poll();
  assert.equal(h.waiting(), true);
  assert.equal(h.audio.paused, true);
  await h.poll();
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, false);
  h.cleanup();
});

test('waiting ends at thirty seconds from kickoff and cannot restart on resumed playback or polls', async () => {
  const h = harness({ responses: [newScan()] });
  await h.audio.play();
  await settle();
  await h.advance(29999);
  assert.equal(h.waiting(), true);
  assert.equal(h.audio.paused, true);
  await h.advance(1);
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, false);
  assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'false');
  await h.audio.play();
  h.emit('playing');
  await h.poll();
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, false);
  assert.equal(h.requests.filter((request) => request.method === 'POST').length, 1);
  h.cleanup();
});

test('POST latency consumes the same thirty-second wait budget', async () => {
  const pending = deferred();
  const h = harness({ responses: [pending.promise] });
  await h.audio.play();
  await h.advance(20000);
  assert.equal(h.audio.paused, false, 'unknown transcript state leaves playback independent until POST confirms eligibility');
  pending.resolve(newScan());
  await settle();
  assert.equal(h.waiting(), true);
  assert.equal([...h.timers.values()].some((timer) => timer.ms === 10000), true);
  await h.advance(9999);
  assert.equal(h.audio.paused, true);
  await h.advance(1);
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, false);
  h.cleanup();
});

test('a response after the thirty-second deadline cannot start a late hold', async () => {
  const pending = deferred();
  const h = harness({ responses: [pending.promise] });
  await h.audio.play();
  await h.advance(30001);
  pending.resolve(newScan());
  await settle();
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, false);
  h.cleanup();
});

test('Skip explicitly resumes playback and leaves the scan polling in the background', async () => {
  const h = harness({ responses: [newScan()] });
  await h.audio.play();
  await settle();
  h.elements.get('#podcast-play-toggle').dispatchEvent(new Event('click'));
  assert.equal(h.elements.get('#podcast-play-toggle')['aria-label'], 'Play episode');
  h.elements.get('#podcast-scan-wait-skip').dispatchEvent(new Event('click'));
  await settle();
  assert.equal(h.audio.paused, false, 'the explicit Skip action overrides the earlier pause intent');
  assert.equal(h.waiting(), false);
  assert.equal([...h.timers.values()].some((timer) => timer.ms === 30000), false);
  await h.poll();
  assert.equal(h.audio.paused, false);
  assert.equal(h.waiting(), false);
  assert.equal(h.requests.filter((request) => request.method === 'POST').length, 1);
  h.cleanup();
});

test('scan failures and terminal empty results release waiting without claiming sponsors were found', async () => {
  for (const response of [newScan({ status: 'failed', error: 'transcription_failed' }), newScan({ status: 'failed', error: 'source_changed' }), newScan({ status: 'unavailable', error: 'provider_disabled' }), new Error('offline'), { ok: false, status: 502 }, ready({ segments: [] })]) {
    const h = harness({ responses: [newScan(), response] });
    await h.audio.play();
    await settle();
    assert.equal(h.waiting(), true);
    await h.poll();
    assert.equal(h.waiting(), false);
    assert.equal(h.audio.paused, false);
    assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'false');
    assert.equal(h.timers.size, 0);
    h.cleanup();
  }
});

test('failed kickoff never activates a wait, and disabled settings release an active wait', async () => {
  const failed = harness({ initialSegments: newScan({ status: 'failed', error: 'transcription_failed' }), responses: [newScan({ status: 'failed', error: 'transcription_failed' })] });
  await failed.audio.play();
  await settle();
  assert.equal(failed.waiting(), false);
  assert.equal(failed.audio.paused, false);
  failed.cleanup();

  const h = harness({ responses: [newScan(), newScan({ enabled: false, status: 'disabled' })] });
  await h.audio.play();
  await settle();
  await h.poll();
  assert.equal(h.audio.paused, false);
  assert.equal(h.waiting(), false);
  assert.equal(h.timers.size, 0);
  h.cleanup();
});

for (const reason of ['unauthenticated', 'disabled settings']) {
  test(`${reason} cannot activate waiting for a missing transcript`, async () => {
    const h = harness({ initialSegments: newScan({ enabled: false }) });
    await h.audio.play();
    await h.advance(30000);
    assert.equal(h.waiting(), false);
    assert.equal(h.audio.paused, false);
    assert.equal(h.requests.length, 0);
    assert.equal(h.timers.size, 0);
    h.cleanup();
  });
}

test('user pause while waiting prevents automatic resume on a result or timeout', async () => {
  for (const result of [false, true]) {
    const h = harness({ responses: result ? [newScan(), newScan({ has_segments: true })] : [newScan()] });
    await h.audio.play();
    await settle();
    h.elements.get('#podcast-play-toggle').dispatchEvent(new Event('click'));
    assert.equal(h.elements.get('#podcast-play-toggle')['aria-label'], 'Play episode');
    const plays = h.audio.playCalls;
    if (result) await h.poll();
    else await h.advance(30000);
    assert.equal(h.waiting(), false);
    assert.equal(h.audio.paused, true);
    assert.equal(h.audio.playCalls, plays);
    h.cleanup();
  }
});

test('a user pause before POST resolves is preserved when the deadline releases the hold', async () => {
  const pending = deferred();
  const h = harness({ responses: [pending.promise] });
  await h.audio.play();
  h.audio.pause();
  pending.resolve(newScan());
  await settle();
  await h.advance(30000);
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, true);
  assert.equal(h.audio.playCalls, 1);
  h.cleanup();
});

test('play attempts during a hold keep loading and do not restart kickoff or the deadline', async () => {
  const h = harness({ responses: [newScan()] });
  await h.audio.play();
  await settle();
  await h.advance(5000);
  const deadline = [...h.timers.values()].find((timer) => timer.ms === 30000).due;
  await h.audio.play();
  h.emit('playing');
  assert.equal(h.audio.paused, true);
  assert.equal(h.waiting(), true);
  assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'true');
  assert.equal([...h.timers.values()].find((timer) => timer.ms === 30000).due, deadline);
  assert.equal(h.requests.filter((request) => request.method === 'POST').length, 1);
  await h.advance(25000);
  assert.equal(h.audio.paused, false);
  h.cleanup();
});

test('audio playback failure ends waiting without resuming, including a late POST', async () => {
  const pending = deferred();
  const h = harness({ initialSegments: newScan({ status: 'idle' }), responses: [pending.promise] });
  await h.audio.play();
  assert.equal(h.waiting(), true);
  h.emit('error');
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, true);
  assert.equal(h.elements.get('#podcast-play-toggle')['aria-busy'], 'false');
  pending.resolve(newScan());
  await settle();
  await h.advance(30000);
  assert.equal(h.waiting(), false);
  assert.equal(h.audio.paused, true);
  assert.equal(h.audio.playCalls, 1);
  h.cleanup();
});

test('cleanup cancels the wait deadline and ignores late results without resuming the old player', async () => {
  const pending = deferred();
  const h = harness({ initialSegments: newScan({ status: 'idle' }), responses: [pending.promise] });
  await h.audio.play();
  const deadline = [...h.timers.values()].find((timer) => timer.ms === 30000);
  h.cleanup();
  assert.equal(h.timers.size, 0);
  assert.equal(h.waiting(), false);
  deadline.callback();
  pending.resolve(newScan({ has_segments: true }));
  await settle();
  assert.equal(h.audio.paused, true);
  assert.equal(h.audio.playCalls, 1);
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0].signal.aborted, true);
});

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

test('a pending replacement job suppresses old segments until its current result is confirmed', async () => {
  const h = harness({ initialSegments: ready(), responses: [{ status: 'running', phase: 'preparing', segments: [] }, ready({ status: 'running', phase: 'scanning', segments: [{ ...sponsor, start_ms: 11000, end_ms: 25000 }] })], currentTime: 12 });
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

test('confirmed segments appear and skip while preparation and scanning continue', async () => {
  const later = { ...sponsor, start_ms: 30000, end_ms: 40000, start_text: 'Later sponsor' };
  const h = harness({ responses: [ready({ status: 'running', phase: 'preparing', segments: [] }), ready({ status: 'running', phase: 'scanning' }), ready({ status: 'running', phase: 'scanning', segments: [sponsor, later] }), ready({ segments: [sponsor, later] })], currentTime: 12 });
  await h.audio.play();
  await settle();
  assert.equal(h.audio.currentTime, 12);
  assert.match(h.text(), /Preparing episode transcript/);
  assert.doesNotMatch(h.text(), /No selected episode segments/);
  await h.poll();
  assert.equal(h.audio.currentTime, 20);
  assert.match(h.text(), /Scanning/);
  assert.match(h.text(), /Sponsor start/);
  assert.equal(h.elements.get('[data-podcast-segment-count]').textContent, '1');
  assert.equal(h.elements.get('[data-podcast-segment-count]').classList.contains('hidden'), false);
  assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), false);
  h.audio.currentTime = 31;
  await h.poll();
  assert.equal(h.audio.currentTime, 40);
  assert.equal(h.elements.get('[data-podcast-segment-count]').textContent, '2');
  await h.poll();
  assert.doesNotMatch(h.text(), /Scanning/);
  assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), true);
  assert.equal(h.timers.size, 0);
  h.cleanup();
});

test('later scan failure retains confirmed segments with an incomplete message', async () => {
  const h = harness({ responses: [ready({ status: 'running', phase: 'scanning' }), ready({ status: 'failed', phase: 'failed', error: 'transcription_failed' })] });
  await h.audio.play();
  await settle();
  h.audio.currentTime = 12;
  await h.poll();
  assert.equal(h.audio.currentTime, 20);
  assert.equal(h.audio.paused, false);
  assert.match(h.text(), /stopped before completion/);
  assert.match(h.text(), /Sponsor start/);
  assert.doesNotMatch(h.text(), /No selected episode segments/);
  assert.equal(h.elements.get('[data-podcast-segment-count]').textContent, '1');
  assert.equal(h.elements.get('[data-podcast-segment-spinner]').classList.contains('hidden'), true);
  assert.equal(h.timers.size, 0);
  h.cleanup();
});

test('a validated POST may return an already failed scan with usable confirmed intervals', async () => {
  const partial = ready({ status: 'failed', phase: 'failed', error: 'transcription_failed' });
  const h = harness({ initialSegments: partial, responses: [partial], currentTime: 12, readyState: 1 });
  assert.equal(h.audio.currentTime, 12);
  await h.audio.play();
  await settle();
  assert.equal(h.audio.currentTime, 20);
  assert.match(h.text(), /stopped before completion/);
  assert.match(h.text(), /Sponsor start/);
  h.cleanup();
});

for (const invalidation of ['source_changed', 'source_validation_failed', 'configuration_changed']) {
  test(`${invalidation} clears a validated running snapshot instead of skipping retained intervals`, async () => {
    const h = harness({ responses: [ready({ status: 'running', phase: 'scanning' }), ready({ status: 'failed', phase: 'failed', error: invalidation })] });
    await h.audio.play();
    await settle();
    assert.match(h.text(), /Sponsor start/);
    h.audio.currentTime = 12;
    await h.poll();
    h.emit('timeupdate');
    assert.equal(h.audio.currentTime, 12);
    assert.equal(h.audio.paused, false);
    assert.doesNotMatch(h.text(), /Sponsor start|confirmed segments remain available/i);
    assert.match(h.text(), /analysis failed/);
    assert.equal(h.elements.get('#podcast-segment-markers').children.length, 0);
    assert.equal(h.elements.get('[data-podcast-segment-count]').classList.contains('hidden'), true);
    assert.equal(h.timers.size, 0);
    assert.deepEqual(h.requests.map((request) => request.method), ['POST', 'GET']);
    h.cleanup();
  });

  test(`a failed ${invalidation} POST cannot validate retained cached intervals`, async () => {
    const h = harness({ initialSegments: ready(), currentTime: 12, responses: [ready({ status: 'failed', phase: 'failed', error: invalidation })] });
    await h.audio.play();
    await settle();
    h.emit('timeupdate');
    assert.equal(h.audio.currentTime, 12);
    assert.equal(h.audio.paused, false);
    assert.doesNotMatch(h.text(), /Sponsor start|confirmed segments remain available/i);
    assert.match(h.text(), /analysis failed/);
    assert.equal(h.elements.get('#podcast-segment-markers').children.length, 0);
    assert.equal(h.timers.size, 0);
    assert.deepEqual(h.requests.map((request) => request.method), ['POST']);
    h.cleanup();
  });
}

test('a failed status refresh retains the last validated confirmed snapshot', async () => {
  const h = harness({ responses: [ready({ status: 'running', phase: 'scanning' }), new Error('offline')] });
  await h.audio.play();
  await settle();
  h.audio.currentTime = 12;
  await h.poll();
  assert.equal(h.audio.currentTime, 20);
  assert.match(h.text(), /Could not refresh/);
  assert.match(h.text(), /scan may be incomplete/);
  assert.match(h.text(), /Sponsor start/);
  assert.equal(h.timers.size, 0);
  h.cleanup();
});

test('poll snapshots replace rows and deduplicate repeated confirmed intervals', async () => {
  const corrected = { ...sponsor, start_text: 'Corrected sponsor' };
  const later = { ...sponsor, start_ms: 30000, end_ms: 40000, start_text: 'Later sponsor' };
  const h = harness({ responses: [ready({ status: 'running', phase: 'scanning', segments: [sponsor, sponsor] }), ready({ status: 'running', phase: 'scanning', segments: [corrected, later, corrected] }), ready({ segments: [corrected] })] });
  await h.audio.play();
  await settle();
  assert.equal(h.elements.get('[data-podcast-segment-count]').textContent, '1');
  assert.equal(h.elements.get('#podcast-segment-markers').children.length, 1);
  await h.poll();
  assert.equal(h.elements.get('[data-podcast-segment-count]').textContent, '2');
  assert.equal(h.elements.get('#podcast-segment-markers').children.length, 2);
  assert.match(h.text(), /Corrected sponsor/);
  assert.doesNotMatch(h.text(), /Sponsor start/);
  await h.poll();
  assert.equal(h.elements.get('[data-podcast-segment-count]').textContent, '1');
  assert.equal(h.elements.get('#podcast-segment-markers').children.length, 1);
  assert.doesNotMatch(h.text(), /Later sponsor/);
  h.cleanup();
});

test('disabled settings suppress cached partial segments and stop an active poll', async () => {
  const disabled = harness({ initialSegments: ready({ enabled: false, status: 'running', phase: 'scanning' }), currentTime: 12 });
  await disabled.audio.play();
  disabled.emit('timeupdate');
  assert.equal(disabled.audio.currentTime, 12);
  assert.equal(disabled.requests.length, 0);
  disabled.cleanup();

  const h = harness({ responses: [ready({ status: 'running', phase: 'scanning' }), ready({ enabled: false, status: 'disabled' })] });
  await h.audio.play();
  await settle();
  h.audio.currentTime = 12;
  await h.poll();
  h.emit('timeupdate');
  assert.equal(h.audio.currentTime, 12);
  assert.equal(h.elements.get('#podcast-segments-tab').hidden, true);
  assert.equal(h.elements.get('#podcast-segment-markers').children.length, 0);
  assert.equal(h.timers.size, 0);
  assert.deepEqual(h.requests.map((request) => request.method), ['POST', 'GET']);
  h.cleanup();
});

test('partial confirmed segments skip despite injected-ad duration differences and scan failure', async () => {
  const h = harness({ duration: 603, currentTime: 12, responses: [ready({ status: 'running', phase: 'scanning' }), ready({ status: 'failed', error: 'transcription_failed' })] });
  await h.audio.play();
  await settle();
  assert.equal(h.audio.currentTime, 20);
  assert.doesNotMatch(h.text(), /audio length differs/);
  h.audio.currentTime = 12;
  await h.poll();
  assert.equal(h.audio.currentTime, 20);
  assert.match(h.text(), /stopped before completion/);
  assert.match(h.text(), /Sponsor start/);
  h.audio.duration = 600;
  h.emit('durationchange');
  assert.equal(h.audio.currentTime, 20);
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
  const h = harness({ initialSegments: ready(), responses: [ready({ status: 'running', phase: 'scanning' })] });
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
