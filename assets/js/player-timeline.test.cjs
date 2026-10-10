const test = require('node:test');
const assert = require('node:assert/strict');
const { normalizeSponsors, blendTimeline, chapterVTT, thumbnailVTT, formatTimelineTime } = require('./feedlr-player.js');

const chapter = (start, end, title) => ({ start, end, title });
const sponsor = (start, end, category = 'sponsor') => ({ start, end, category });
const spans = (items) => items.map(({ start, end, title, sponsor }) => ({ start, end, title, sponsor }));

test('sponsor ranges merge overlap and adjacency with sponsored labels taking priority', () => {
  const input = [sponsor(18, 25, 'unknown'), sponsor(7, 12, 'unknown'), sponsor(12, 20), sponsor(30, 40, 'unknown')];
  const before = structuredClone(input);
  const result = normalizeSponsors(input, 60);
  assert.deepEqual(result.map(({ start, end, title }) => ({ start, end, title })), [
    { start: 7, end: 25, title: 'Sponsored segment' },
    { start: 30, end: 40, title: 'SponsorBlock segment' },
  ]);
  assert.deepEqual(new Set(result[0].categories), new Set(['unknown', 'sponsor']));
  assert.deepEqual(input, before, 'normalization must preserve metadata used for skip behavior');
});

test('sponsor normalization clamps endpoints and drops invalid or empty intervals', () => {
  const result = normalizeSponsors([
    null, {}, sponsor(NaN, 4), sponsor(2, Infinity), sponsor(8, 4), sponsor(7, 7),
    sponsor(-5, 2.125), sponsor(9.75, 20), sponsor(11, 12),
  ], 10);
  assert.deepEqual(result.map(({ start, end }) => [start, end]), [[0, 2.125], [9.75, 10]]);
  assert.deepEqual(normalizeSponsors(null), []);
  assert.deepEqual(normalizeSponsors([]), []);
  assert.deepEqual(normalizeSponsors([sponsor(2, 3)], 0), []);
  assert.deepEqual(normalizeSponsors([sponsor(2, 3)]).map(({ start, end }) => [start, end]), [[2, 3]]);
});

test('sponsors spanning chapter boundaries remain one interval and override chapter titles', () => {
  const result = blendTimeline([
    chapter(0, 10, 'Opening'), chapter(10, 20, 'Discussion'), chapter(20, 30, 'Closing'),
  ], [sponsor(8, 13), sponsor(12, 22)], 30);
  assert.deepEqual(spans(result), [
    { start: 0, end: 8, title: 'Opening', sponsor: false },
    { start: 8, end: 22, title: 'Sponsored segment', sponsor: true },
    { start: 22, end: 30, title: 'Closing', sponsor: false },
  ]);
  assert.equal(result[0].chapterIndex, 0);
  assert.equal(result[2].chapterIndex, 2);
});

test('a sponsor in the middle of a chapter splits content and retains its chapter identity', () => {
  const result = blendTimeline([chapter(0, 60, 'Discussion')], [sponsor(20, 25)], 60);
  assert.deepEqual(spans(result), [
    { start: 0, end: 20, title: 'Discussion', sponsor: false },
    { start: 20, end: 25, title: 'Sponsored segment', sponsor: true },
    { start: 25, end: 60, title: 'Discussion', sponsor: false },
  ]);
  assert.deepEqual(result.filter((item) => !item.sponsor).map((item) => item.chapterIndex), [0, 0]);
});

test('sponsors exactly matching chapters produce no zero-length content slices', () => {
  const result = blendTimeline([chapter(0, 10, 'First'), chapter(10, 20, 'Second'), chapter(20, 30, 'Third')], [sponsor(10, 20)], 30);
  assert.deepEqual(spans(result), [
    { start: 0, end: 10, title: 'First', sponsor: false },
    { start: 10, end: 20, title: 'Sponsored segment', sponsor: true },
    { start: 20, end: 30, title: 'Third', sponsor: false },
  ]);
});

test('chapter-only gaps remain explicit while sponsor timelines label unchaptered content', () => {
  assert.deepEqual(spans(blendTimeline([chapter(5, 10, 'First'), chapter(20, 25, 'Second')], [], 30)), [
    { start: 5, end: 10, title: 'First', sponsor: false },
    { start: 20, end: 25, title: 'Second', sponsor: false },
  ]);
  assert.deepEqual(spans(blendTimeline([], [sponsor(5, 10), sponsor(20, 25, 'unknown')], 30)), [
    { start: 0, end: 5, title: 'Video', sponsor: false },
    { start: 5, end: 10, title: 'Sponsored segment', sponsor: true },
    { start: 10, end: 20, title: 'Video', sponsor: false },
    { start: 20, end: 25, title: 'SponsorBlock segment', sponsor: true },
    { start: 25, end: 30, title: 'Video', sponsor: false },
  ]);
  assert.deepEqual(blendTimeline([], [], 30), []);
});

test('timeline clamps to duration, retains fractions, and ignores invalid chapter intervals', () => {
  const result = blendTimeline([
    null, chapter(0, 0, 'Empty'), chapter(8, 4, 'Reversed'), chapter(NaN, 10, 'Invalid'),
    chapter(0, Infinity, 'Unbounded'), chapter(-2, 12, 'Discussion'), chapter(20, 25, 'After the end'),
  ], [sponsor(2.125, 3.875), sponsor(8.5, 20)], 10);
  assert.deepEqual(spans(result), [
    { start: 0, end: 2.125, title: 'Discussion', sponsor: false },
    { start: 2.125, end: 3.875, title: 'Sponsored segment', sponsor: true },
    { start: 3.875, end: 8.5, title: 'Discussion', sponsor: false },
    { start: 8.5, end: 10, title: 'Sponsored segment', sponsor: true },
  ]);
  for (const duration of [0, -1, NaN, Infinity]) {
    assert.deepEqual(blendTimeline([chapter(0, 10, 'Content')], [sponsor(3, 4)], duration), []);
  }
  assert.deepEqual(blendTimeline(null, null, 20), []);
});

test('multiple sponsor ranges fully covering a chapter remove its content entries', () => {
  const result = blendTimeline([chapter(4, 9, 'Fully covered')], [sponsor(2, 6), sponsor(6, 12)], 20);
  assert.deepEqual(spans(result), [
    { start: 0, end: 2, title: 'Video', sponsor: false },
    { start: 2, end: 12, title: 'Sponsored segment', sponsor: true },
    { start: 12, end: 20, title: 'Video', sponsor: false },
  ]);
});

test('unnamed chapters do not create blank timeline menu entries', () => {
  const result = blendTimeline([
    chapter(0, 10, ''), chapter(10, 20, '   '), { start: 20, end: 30 }, chapter(30, 40, 'Named'),
  ], [sponsor(5, 7)], 40);
  assert.deepEqual(spans(result), [
    { start: 0, end: 5, title: 'Video', sponsor: false },
    { start: 5, end: 7, title: 'Sponsored segment', sponsor: true },
    { start: 7, end: 30, title: 'Video', sponsor: false },
    { start: 30, end: 40, title: 'Named', sponsor: false },
  ]);
  assert.equal(result[3].chapterIndex, 3, 'ignored chapters cannot change thumbnail indices');
  assert.ok(result.filter((item) => item.title === 'Video').every((item) => item.chapterIndex === undefined));
});

test('a sponsor ending before video duration always has a nonsponsor tail cue', () => {
  for (const chapters of [[], [chapter(0, 15, 'Opening')]]) {
    const result = blendTimeline(chapters, [sponsor(10, 20)], 30);
    const last = result.at(-1);
    assert.equal(last.start, 20);
    assert.equal(last.end, 30);
    assert.equal(last.sponsor, false, 'Shaka extends its last hover cue beyond its end time');
    assert.equal(last.title, 'Video');
    assert.equal(last.chapterIndex, undefined);
    assert.ok(chapterVTT(result).includes('00:00:20.000 --> 00:00:30.000\nVideo'));
    for (let i = 1; i < result.length; i++) {
      assert.equal(result[i].start, result[i - 1].end, 'the entire sponsor timeline must have contiguous cues');
    }
  }
});

test('chapter WebVTT preserves millisecond timing and safely escapes title markup', () => {
  const result = chapterVTT([{ start: 2.125, end: 3661.875, title: 'A & <b>unsafe</b> --> title', sponsor: false }]);
  assert.ok(result.startsWith('WEBVTT\n\n'));
  assert.ok(result.includes('00:00:02.125 --> 01:01:01.875'));
  assert.ok(result.includes('A &amp; &lt;b&gt;unsafe&lt;/b&gt; --&gt; title'));
  assert.ok(!result.includes('<b>'));
  assert.equal(chapterVTT([]).trim(), 'WEBVTT');
});

test('thumbnail VTT keeps previews continuous across sponsors using cached chapter URLs', () => {
  const chapters = [
    { ...chapter(0, 10, 'No image'), thumbnails: [] },
    { ...chapter(10, 30, 'Discussion'), thumbnails: [{ url: 'https://i.ytimg.com/preview.jpg', width: 160, height: 90 }] },
  ];
  const items = blendTimeline(chapters, [sponsor(15, 20)], 30);
  const result = thumbnailVTT(items, chapters, 'dQw4w9WgXcQ', 'https://feedlr.example');
  assert.ok(result.startsWith('WEBVTT\n\n'));
  assert.ok(result.includes('00:00:10.000 --> 00:00:30.000'));
  const urls = result.split('\n').filter((line) => line.startsWith('https://'));
  assert.equal(urls.length, 1, 'sponsor splits must not duplicate image cues');
  assert.ok(urls.every((url) => url === 'https://feedlr.example/api/videos/dQw4w9WgXcQ/chapters/1/thumbnail?v=3#xywh=0,0,160,90'));
  assert.ok(!result.includes('i.ytimg.com'), 'browser tracks must use the authorized thumbnail proxy');
  assert.ok(!result.includes('/chapters/0/'));
});

test('storyboard previews share a sheet request while selecting distinct chapter frames', () => {
  const sheet = 'https://i.ytimg.com/sb/video/M0.jpg';
  const image = (x, y) => ({url:sheet,width:800,height:450,sprite:{x,y,width:160,height:90}});
  const chapters = [
    { ...chapter(0, 10, 'First'), thumbnails: [image(0, 0)] },
    { ...chapter(10, 20, 'Second'), thumbnails: [image(160, 0)] },
    { ...chapter(20, 30, 'Third'), thumbnails: [image(320, 90)] },
  ];
  const result = thumbnailVTT(blendTimeline(chapters, [], 30), chapters, 'video', 'https://feedlr.example');
  const urls = result.split('\n').filter(line => line.startsWith('https://'));
  assert.equal(new Set(urls.map(url => url.split('#')[0])).size, 1, 'one cached proxy URL per image sheet');
  assert.deepEqual(urls.map(url => new URL(url).hash), ['#xywh=0,0,160,90', '#xywh=160,0,160,90', '#xywh=320,90,160,90']);
  assert.ok(urls.every(url => url.includes('/chapters/0/thumbnail?v=3')));
});

test('standalone previews carry dimensions matching the proxy image selection', () => {
  const chapters = [{ ...chapter(0, 30, 'Content'), thumbnails: [
    {url:'https://i.ytimg.com/small.jpg',width:160,height:90},
    {url:'https://i.ytimg.com/large.jpg',width:336,height:188},
  ] }];
  const result = thumbnailVTT(blendTimeline(chapters, [], 30), chapters, 'video', 'https://feedlr.example');
  assert.ok(result.includes('#xywh=0,0,336,188'), 'Shaka must receive a positive image aspect ratio');
});

test('opening cover and higher resolution storyboard frames use separate image sources', () => {
  const sheet = 'https://i.ytimg.com/sb/video/M0.jpg';
  const chapters = [
    { ...chapter(0, 10, 'Opening'), thumbnails: [{url:'https://i.ytimg.com/vi/video/cover.jpg',width:336,height:188}] },
    { ...chapter(10, 20, 'Middle'), thumbnails: [{url:sheet,width:960,height:540,sprite:{x:320,y:0,width:320,height:180}}] },
    { ...chapter(20, 30, 'End'), thumbnails: [{url:sheet,width:960,height:540,sprite:{x:640,y:0,width:320,height:180}}] },
  ];
  const result = thumbnailVTT(blendTimeline(chapters, [], 30), chapters, 'video', 'https://feedlr.example');
  const urls = result.split('\n').filter(line => line.startsWith('https://'));
  assert.deepEqual(urls, [
    'https://feedlr.example/api/videos/video/chapters/0/thumbnail?v=3#xywh=0,0,336,188',
    'https://feedlr.example/api/videos/video/chapters/1/thumbnail?v=3#xywh=320,0,320,180',
    'https://feedlr.example/api/videos/video/chapters/1/thumbnail?v=3#xywh=640,0,320,180',
  ]);
});

test('sponsor preview images change at the original chapter boundary', () => {
  const chapters = [
    { ...chapter(0, 10, 'Before'), thumbnails: [{url:'https://i.ytimg.com/before.jpg'}] },
    { ...chapter(10, 20, 'After'), thumbnails: [{url:'https://i.ytimg.com/after.jpg'}] },
  ];
  const items = blendTimeline(chapters, [sponsor(5, 15)], 20);
  assert.equal(items.filter(item => item.sponsor).length, 1, 'sponsor priority keeps one continuous label');
  const result = thumbnailVTT(items, chapters, 'video', 'https://feedlr.example');
  assert.ok(result.includes('00:00:00.000 --> 00:00:10.000\nhttps://feedlr.example/api/videos/video/chapters/0/thumbnail'));
  assert.ok(result.includes('00:00:10.000 --> 00:00:20.000\nhttps://feedlr.example/api/videos/video/chapters/1/thumbnail'));
});

test('thumbnail VTT gracefully omits unavailable thumbnail metadata', () => {
  const items = [
    { start: 0, end: 10, title: 'Missing', chapterIndex: 0 },
    { start: 10, end: 20, title: 'Out of range', chapterIndex: 8 },
  ];
  assert.equal(thumbnailVTT(items, [chapter(0, 10, 'Missing')], 'video', 'https://feedlr.example').trim(), 'WEBVTT');
  assert.equal(thumbnailVTT([], [], 'video', 'https://feedlr.example').trim(), 'WEBVTT');
});

test('timeline time labels keep seconds padded and include hours only when needed', () => {
  for (const [seconds, label] of [[0, '0:00'], [9, '0:09'], [65.875, '1:05'], [3599, '59:59'], [3600, '1:00:00'], [3661, '1:01:01']]) {
    assert.equal(formatTimelineTime(seconds), label);
  }
});
