/* Shared podcast runtime: media state, validated analysis and cancellation remain outside React. */
(function (window) {
"use strict";
function mount(options) {
const { episode, title, channel, progress = 0, withProgress = false, initialSegments } = options;
const root = options.root || document.getElementById("podcast-player-page")
const audio = options.audio || document.getElementById("podcast-player")
if (!root || !audio) return

window.feedlr_player?.cleanup?.()
window.feedlrPodcastPlayer?.cleanup?.()
for (const player of document.querySelectorAll("audio[data-feedlr-podcast-player]")) {
	if (player !== audio) {
		player.pause()
		player.removeAttribute("src")
		player.load()
	}
}

const controller = new AbortController()
let lastProgress = -1
let progressTimer
let disposed = false
let pageSuspended = false
let historyPlaying = false
let loading = audio.readyState < HTMLMediaElement.HAVE_FUTURE_DATA
let blocked = false
let snapshot
let snapshotSegmentsSource
const subscribers = new Set()
const readPreference = (key) => { try { return window.localStorage.getItem(key) } catch (_) { return null } }
const writePreference = (key, value) => { try { window.localStorage.setItem(key, String(value)) } catch (_) {} }
let playbackFailed = false
let segmentsEnabled = initialSegments?.enabled === true
let segments = segmentSnapshot(initialSegments?.segments)
let analysisStatus = initialSegments?.status || "idle"
let analysisPhase = initialSegments?.phase || ""
let analysisError = initialSegments?.error || ""
let transcriptReady = initialSegments?.transcript_ready
let hasConfirmedSegments = initialSegments?.has_segments === true || segments.length > 0
let analysisStarted = false
let playbackValidationAccepted = false
let statusRequestGeneration = 0
const manuallyRewoundSegments = new Set()
let scrubbing = false
let pollTimer
let scanHoldActive = false
let scanHoldFinished = false
let scanHoldResume = false
let scanHoldPauseEvents = 0
let scanHoldDeadline = 0
let scanHoldTimer
let progressInput
const scanWaitNotice = root.querySelector("#podcast-scan-wait")
const scanWaitSkip = root.querySelector("#podcast-scan-wait-skip")
const segmentTab = root.querySelector("#podcast-segments-tab")
const notesTab = root.querySelector("#podcast-notes-tab")
const notesPanel = root.querySelector("#podcast-notes-panel")
const segmentsPanel = root.querySelector("#podcast-segments-panel")
const segmentCount = root.querySelector("[data-podcast-segment-count]")
const segmentSpinner = root.querySelector("[data-podcast-segment-spinner]")
const segmentMarkers = root.querySelector("#podcast-segment-markers")
audio.addEventListener("playing", startSegmentAnalysis, { once: true, signal: controller.signal })
function element(tag, className, value) { const node = document.createElement(tag); if (className) node.className = className; if (value) node.textContent = value; return node }
function segmentSnapshot(values) {
	const snapshot = new Map()
	for (const segment of values || []) snapshot.set(JSON.stringify([segment.category, segment.start_ms, segment.end_ms, segment.brand || ""]), segment)
	return [...snapshot.values()]
}
function publish() {
    const previous = snapshot
    const values = {
        position: Math.max(0, Number.isFinite(audio.currentTime) ? audio.currentTime : 0),
        duration: Number.isFinite(audio.duration) ? audio.duration : 0,
        playing: scanHoldActive ? scanHoldResume : !audio.paused && !audio.ended,
        volume: Math.round(audio.volume * 100), rate: audio.playbackRate,
        loading, blocked, error: playbackFailed ? "This episode could not load. Try again." : "",
        scanHold: scanHoldActive, segmentsEnabled, analysisStatus, analysisPhase, analysisError,
        validated: playbackValidationAccepted,
        segments: snapshotSegmentsSource === segments && previous ? previous.segments : Object.freeze(segments.map((segment) => Object.freeze({ ...segment }))),
    }
    if (previous && Object.keys(values).every((key) => Object.is(previous[key], values[key]))) return
    snapshot = Object.freeze(values)
    snapshotSegmentsSource = segments
    for (const listener of subscribers) listener()
}
function invalidatesSegmentValidation(error) { return ["source_changed", "source_validation_failed", "configuration_changed"].includes(error) }
function updateSegmentTrack() {
	if (options.externalUI) return
	if (!progressInput) return
	const duration = Number.isFinite(audio.duration) ? audio.duration : 0
	const ranges = segmentsEnabled && duration > 0 ? segments.filter((segment) => segment.skippable).map((segment) => ({ start: Math.max(0, Math.min(100, segment.start_ms / 1000 / duration * 100)), end: Math.max(0, Math.min(100, segment.end_ms / 1000 / duration * 100)) })).filter((range) => range.end > range.start).sort((a, b) => a.start - b.start) : []
	segmentMarkers.replaceChildren()
	for (const range of ranges) { const marker = document.createElement("span"); marker.className = "absolute h-full rounded-full bg-accent/80"; marker.style.left = `${range.start}%`; marker.style.width = `${range.end - range.start}%`; segmentMarkers.append(marker) }
}
function setActivePanel(panel) {
	const notesActive = panel === "notes"
	notesPanel.classList.toggle("hidden", !notesActive)
	segmentsPanel.classList.toggle("hidden", notesActive)
	notesTab.classList.toggle("ui-tab-active", notesActive)
	segmentTab.classList.toggle("ui-tab-active", !notesActive)
	notesTab.setAttribute("aria-selected", String(notesActive))
	segmentTab.setAttribute("aria-selected", String(!notesActive))
}
function panelMessage(message, pending) {
	const messagePanel = element("div", "glass-panel flex items-center gap-3 p-4 text-sm text-text-secondary")
	if (pending) messagePanel.append(element("span", "ui-spinner size-4 border-2"))
	messagePanel.append(element("span", "", message))
	segmentsPanel.replaceChildren(messagePanel)
}
function analysisFailureMessage() {
	if (segments.length) return analysisError === "request_failed" ? "Could not refresh episode segment analysis. The scan may be incomplete; confirmed segments remain available." : "Episode segment scanning stopped before completion. Confirmed segments remain available."
	if (analysisError === "request_failed") return "Could not load episode segment analysis. Playback will continue."
	if (analysisStatus === "unavailable") return "Episode segment analysis is unavailable for this episode."
	return "Episode segment analysis failed. Playback will continue."
}
function renderSegments() {
	if (options.externalUI) { publish(); return }
	updateSegmentTrack()
	segmentTab.hidden = !segmentsEnabled
	const active = analysisStatus === "pending" || analysisStatus === "running"
	segmentSpinner.classList.toggle("hidden", !segmentsEnabled || !active)
	segmentCount.classList.toggle("hidden", !segmentsEnabled || (!segments.length && analysisStatus !== "ready"))
	if (!segmentsEnabled) { segmentsPanel.replaceChildren(); return }
	segmentCount.textContent = String(segments.length)
	segmentsPanel.replaceChildren()
	if (analysisStatus === "idle") { panelMessage("Episode segment analysis starts when playback begins.", false); return }
	if (active) panelMessage(analysisPhase === "validating" ? "Checking episode audio…" : analysisPhase === "scanning" ? "Scanning the episode for sponsors…" : analysisPhase === "preparing" || analysisPhase === "preparation" || analysisPhase === "transcribing" ? "Preparing episode transcript…" : "Finding episode segments…", false)
	if (analysisStatus === "failed" || analysisStatus === "unavailable") panelMessage(analysisFailureMessage(), false)
	if (!segments.length) { if (analysisStatus === "ready") panelMessage("No selected episode segments identified.", false); return }
	const warning = !playbackValidationAccepted ? "Automatic skipping starts after the episode audio is checked when playback begins." : ""
	if (warning) segmentsPanel.append(element("p", "mb-3 text-sm text-text-secondary", warning))
	const list = element("div", "flex flex-col gap-2")
	for (const segment of segments) {
		const row = element("div", "glass-panel w-full p-3")
		const header = element("div", "flex items-center justify-between gap-3")
		const labels = element("div", "flex min-w-0 items-center gap-2")
		labels.append(element("span", "ui-badge ui-badge-accent capitalize", segment.category))
		header.append(labels)
		const duration = Math.max(0, Math.round((segment.end_ms - segment.start_ms) / 1000))
		header.append(element("span", "shrink-0 text-xs tabular-nums text-text-secondary", `${formatTime(segment.start_ms/1000)}–${formatTime(segment.end_ms/1000)} · ${duration}s`))
		row.append(header)
		row.append(element("p", "mt-2 line-clamp-2 text-sm leading-5 text-text-secondary", `${segment.start_text}${segment.end_text !== segment.start_text ? " … "+segment.end_text : ""}`))
		list.append(row)
	}
	segmentsPanel.append(list)
}
function applyStatus(payload, allowHold = false) {
	if (typeof payload.enabled === "boolean") segmentsEnabled = payload.enabled
	analysisStatus = payload.status || "idle"
	analysisPhase = payload.phase || ""
	analysisError = payload.error || ""
	if (typeof payload.transcript_ready === "boolean") transcriptReady = payload.transcript_ready
	segments = segmentsEnabled && !invalidatesSegmentValidation(analysisError) ? segmentSnapshot(payload.segments) : []
	hasConfirmedSegments = !invalidatesSegmentValidation(analysisError) && (payload.has_segments === true || segments.length > 0)
	if (invalidatesSegmentValidation(analysisError)) playbackValidationAccepted = false
	if (scanHoldActive && (!segmentsEnabled || analysisError || hasConfirmedSegments || !["pending", "running"].includes(analysisStatus))) finishScanHold()
	else if (allowHold && playbackValidationAccepted) beginScanHold()
	renderSegments()
	skipSegmentAtCurrentTime()
}
async function loadSegmentStatus(method) {
	if (disposed || !segmentsEnabled) return
	if (pollTimer) window.clearTimeout(pollTimer)
	pollTimer = undefined
	const generation = ++statusRequestGeneration
	try {
		const response = await fetch(`/api/v1/media/${encodeURIComponent(episode)}/segments`, { method, credentials: "same-origin", signal: controller.signal })
		if (disposed || generation !== statusRequestGeneration) return
		if (!response.ok) throw new Error("segment_request_failed")
		const payload = await response.json()
		if (disposed || generation !== statusRequestGeneration) return
		if (method === "POST") playbackValidationAccepted = !invalidatesSegmentValidation(payload.error) && (payload.status === "ready" || payload.status === "pending" || payload.status === "running" || (payload.status === "failed" && payload.segments?.length > 0))
		applyStatus(payload, method === "POST")
		if (segmentsEnabled && (payload.status === "pending" || payload.status === "running")) pollTimer = window.setTimeout(() => { pollTimer = undefined; loadSegmentStatus("GET") }, Number(payload.poll_after_ms) > 0 ? Number(payload.poll_after_ms) : 2000)
	} catch (error) {
		if (disposed || generation !== statusRequestGeneration || error.name === "AbortError") return
		applyStatus({ status: "failed", error: "request_failed", segments: playbackValidationAccepted ? segments : [] })
	}
}
function startSegmentAnalysis() {
	if (disposed || !segmentsEnabled || analysisStarted) return
	const canHoldInitially = transcriptReady === false && !analysisError && !["failed", "unavailable"].includes(analysisStatus) && !hasConfirmedSegments
	analysisStarted = true
	scanHoldDeadline = Date.now() + 30000
	playbackValidationAccepted = false
	analysisPhase = analysisStatus === "ready" ? "validating" : "preparing"
	analysisStatus = "pending"
	analysisError = ""
	setLoading(false)
	if (canHoldInitially) beginScanHold()
	renderSegments()
	loadSegmentStatus("POST")
}
function pauseForScanHold() {
	if (!audio.paused) { scanHoldPauseEvents++; audio.pause() }
}
function beginScanHold() {
	if (disposed || scanHoldActive || scanHoldFinished || playbackFailed || !analysisStarted || !segmentsEnabled || transcriptReady !== false || analysisError || hasConfirmedSegments || !["pending", "running"].includes(analysisStatus)) return
	const remaining = scanHoldDeadline - Date.now()
	if (remaining <= 0) { scanHoldFinished = true; return }
	scanHoldActive = true
	scanHoldResume = !audio.paused && !audio.ended
	scanWaitNotice?.classList.remove("hidden")
	pauseForScanHold()
	updatePlayButton()
	scanHoldTimer = window.setTimeout(finishScanHold, remaining)
}
function finishScanHold() {
	if (disposed || !scanHoldActive) return
	scanHoldActive = false
	scanHoldFinished = true
	if (scanHoldTimer) window.clearTimeout(scanHoldTimer)
	scanHoldTimer = undefined
	scanWaitNotice?.classList.add("hidden")
	const resume = scanHoldResume && !audio.ended && !pageSuspended
	scanHoldResume = false
	updatePlayButton()
	if (resume) audio.play().catch((error) => { if (!disposed && error.name !== "AbortError") { blocked = true; loading = false; publish() } })
}
function requestPlay() {
	if (disposed) return
	if (pageSuspended) { historyPlaying = true; return }
	if (scanHoldActive) { scanHoldResume = true; updatePlayButton(); return }
	audio.play().catch((error) => { if (!disposed && error.name !== "AbortError") { blocked = true; loading = false; publish() } })
}
function requestPause() {
	if (disposed) return
	historyPlaying = false
	scanHoldResume = false
	audio.pause()
	updatePlayButton()
}
function togglePlayback() {
	if (scanHoldActive ? scanHoldResume : !audio.paused && !audio.ended) requestPause()
	else requestPlay()
}
scanWaitSkip?.addEventListener("click", () => { scanHoldResume = true; finishScanHold() }, { signal: controller.signal })
notesTab?.addEventListener("click",()=>setActivePanel("notes"),{signal:controller.signal})
segmentTab?.addEventListener("click",()=>setActivePanel("segments"),{signal:controller.signal})
renderSegments()
if (segmentsEnabled && (analysisStatus === "pending" || analysisStatus === "running")) loadSegmentStatus("GET")

function saveProgress() {
	if (!withProgress || disposed) return
	const currentTime = Math.floor(audio.currentTime)
	if (currentTime <= 0 || currentTime === lastProgress) return
	lastProgress = currentTime
	const volume = Math.round(audio.volume * 100)
	const writeProgress = options.persistProgress ? options.persistProgress(currentTime, volume) : fetch(`/api/v1/media/${encodeURIComponent(episode)}/state`, {
		method: "PATCH",
		credentials: "same-origin",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ progressSeconds: currentTime, volume }),
		keepalive: true,
	}).then((response) => { if (!response.ok) throw new Error("progress_write_failed") })
	Promise.resolve(writeProgress).catch(() => { if (lastProgress === currentTime) lastProgress = -1 })
}

function cleanup() {
	if (disposed) return
	saveProgress()
	disposed = true
	pageSuspended = false
	historyPlaying = false
	if (progressTimer) window.clearInterval(progressTimer)
	if (pollTimer) window.clearTimeout(pollTimer)
	if (scanHoldTimer) window.clearTimeout(scanHoldTimer)
	scanHoldActive = false
	scanHoldResume = false
	scanWaitNotice?.classList.add("hidden")
	controller.abort()
	subscribers.clear()
	audio.pause()
	audio.removeAttribute("src")
	audio.load()
	if (window.feedlrPodcastPlayer?.audio === audio) {
		try { for (const action of ["seekbackward", "seekforward", "play", "pause"]) navigator.mediaSession?.setActionHandler(action, null) } catch (_) {}
		delete window.feedlrPodcastPlayer
	}
}

const api = {
    audio, cleanup, subscribe: (listener) => { subscribers.add(listener); return () => subscribers.delete(listener) },
    getSnapshot: () => snapshot, play: requestPlay, pause: requestPause, toggle: togglePlayback,
    seekBy: seek, seekTo: (time) => { if (!disposed && Number.isFinite(time)) setManualPosition(Math.max(0, Math.min(time, Number.isFinite(audio.duration) ? audio.duration : Infinity))) },
    scrub: (time) => { if (!disposed) { scrubbing = true; api.seekTo(time) } },
    finishScrub: () => { if (!disposed) { scrubbing = false; skipSegmentAtCurrentTime(); publish() } },
    skipScanWait: () => { if (!disposed) { scanHoldResume = true; finishScanHold() } },
    setRate: (rate) => { if (!disposed && rate >= 0.5 && rate <= 3 && Number.isFinite(rate)) { audio.playbackRate = rate; writePreference("podcast-playback-rate", rate); publish() } },
    setVolume: (volume) => { if (!disposed && Number.isFinite(volume)) { audio.volume = Math.max(0, Math.min(1, volume / 100)); writePreference("player-volume", Math.round(audio.volume * 100)); publish() } },
    retry: () => { if (!disposed) { playbackFailed = false; blocked = false; setLoading(true); audio.load(); requestPlay() } },
}
window.feedlrPodcastPlayer = api
window.addEventListener("pagehide", (event) => {
	if (!event.persisted) { cleanup(); return }
	if (pageSuspended) return
	historyPlaying = scanHoldActive ? scanHoldResume : !audio.paused && !audio.ended
	pageSuspended = true
	saveProgress()
	scanHoldResume = false
	audio.pause()
	updatePlayButton()
}, { signal: controller.signal })
window.addEventListener("pageshow", (event) => {
	if (!event.persisted || !pageSuspended || disposed) return
	pageSuspended = false
	const resume = historyPlaying
	historyPlaying = false
	if (scanHoldActive && Date.now() >= scanHoldDeadline) finishScanHold()
	if (resume && !audio.ended) requestPlay()
	else updatePlayButton()
}, { signal: controller.signal })
document.addEventListener("visibilitychange", () => { if (document.hidden) saveProgress() }, { signal: controller.signal })

function seek(seconds) {
	const duration = Number.isFinite(audio.duration) ? audio.duration : Infinity
	setManualPosition(Math.max(0, Math.min(duration, audio.currentTime + seconds)))
}
if ("mediaSession" in navigator && "MediaMetadata" in window) {
	try {
		navigator.mediaSession.metadata = new MediaMetadata({
			title,
			artist: channel,
			album: channel,
			artwork: [{ src: new URL(`/thumb/video/${episode}/maxresdefault`, window.location.origin).href }],
		})
		navigator.mediaSession.setActionHandler("seekbackward", () => seek(-15))
		navigator.mediaSession.setActionHandler("seekforward", () => seek(15))
		navigator.mediaSession.setActionHandler("play", requestPlay)
		navigator.mediaSession.setActionHandler("pause", requestPause)
	} catch (_) {}
}
function formatTime(seconds) {
	const value = Math.max(0, Math.floor(Number.isFinite(seconds) ? seconds : 0))
	const hours = Math.floor(value / 3600)
	const minutes = Math.floor(value / 60) % 60
	const secs = value % 60
	return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}` : `${minutes}:${String(secs).padStart(2, "0")}`
}
progressInput = root.querySelector("#podcast-progress")
const elapsed = root.querySelector("#podcast-elapsed")
const remaining = root.querySelector("#podcast-remaining")
function updateTimeline() {
	if (options.externalUI) { publish(); return }
	const duration = Number.isFinite(audio.duration) ? audio.duration : 0
	progressInput.max = String(duration)
	progressInput.value = String(Math.min(audio.currentTime, duration))
	progressInput.setAttribute("aria-valuetext", `${formatTime(audio.currentTime)} of ${formatTime(duration)}`)
	elapsed.textContent = formatTime(audio.currentTime)
	remaining.textContent = `-${formatTime(duration - audio.currentTime)}`
	updateSegmentTrack()
}
function segmentID(segment) { return `${segment.start_ms}:${segment.end_ms}` }
function segmentAt(time) { return segments.find((segment) => segment.skippable && time >= segment.start_ms / 1000 && time < segment.end_ms / 1000) }
function setManualPosition(time) {
	if (time < audio.currentTime) {
		for (const segment of segments) {
			if (segment.skippable && time >= segment.start_ms / 1000 && time < segment.end_ms / 1000) manuallyRewoundSegments.add(segmentID(segment))
		}
	}
	audio.currentTime = time
	updateTimeline()
	skipSegmentAtCurrentTime()
}
function skipSegmentAtCurrentTime() {
	if (scanHoldActive || scrubbing || !segmentsEnabled || !playbackValidationAccepted || !["pending", "running", "ready", "failed"].includes(analysisStatus)) return
	const segment = segmentAt(audio.currentTime)
	if (!segment || manuallyRewoundSegments.has(segmentID(segment))) return
	const end = Number.isFinite(audio.duration) ? Math.min(segment.end_ms / 1000, audio.duration) : segment.end_ms / 1000
	if (end <= audio.currentTime) return
	audio.currentTime = end
	updateTimeline()
	saveProgress()
}
const playToggle = root.querySelector("#podcast-play-toggle")
const loadingIcon = root.querySelector("[data-podcast-loading-icon]")
function setLoading(value) {
	loading = value
	updatePlayButton()
}
function updatePlayButton() {
	if (options.externalUI) { publish(); return }
	const busy = loading || scanHoldActive
	const playing = scanHoldActive ? scanHoldResume : !audio.paused && !audio.ended
	playToggle.disabled = loading && !scanHoldActive
	playToggle.setAttribute("aria-busy", String(busy))
	loadingIcon.classList.toggle("hidden", !busy)
	root.querySelector("[data-podcast-play-icon]").classList.toggle("hidden", busy || playing)
	root.querySelector("[data-podcast-pause-icon]").classList.toggle("hidden", busy || !playing)
	playToggle.setAttribute("aria-label", busy && !scanHoldActive ? "Loading episode" : playing ? "Pause episode" : "Play episode")
}

function restoreProgress() {
	if (progress > 0 && Number.isFinite(audio.duration)) audio.currentTime = Math.min(progress, audio.duration)
	updateTimeline()
	skipSegmentAtCurrentTime()
}
if (audio.readyState >= HTMLMediaElement.HAVE_METADATA) restoreProgress()
else audio.addEventListener("loadedmetadata", restoreProgress, { once: true, signal: controller.signal })

const savedVolume = Number.parseInt(readPreference("player-volume"), 10)
if (Number.isInteger(savedVolume) && savedVolume >= 0 && savedVolume <= 100) audio.volume = savedVolume / 100
const savedRate = Number.parseFloat(readPreference("podcast-playback-rate"))
if (Number.isFinite(savedRate) && savedRate >= 0.5 && savedRate <= 3) audio.playbackRate = savedRate

const speedSelect = root.querySelector("#podcast-speed")
if (speedSelect) speedSelect.value = String(audio.playbackRate)
speedSelect?.addEventListener("change", () => {
	audio.playbackRate = Number.parseFloat(speedSelect.value)
	writePreference("podcast-playback-rate", speedSelect.value)
}, { signal: controller.signal })
const volumeInput = root.querySelector("#podcast-volume")
if (volumeInput) volumeInput.value = String(Math.round(audio.volume * 100))
volumeInput?.addEventListener("input", () => { audio.volume = Number(volumeInput.value) / 100 }, { signal: controller.signal })
playToggle?.addEventListener("click", togglePlayback, { signal: controller.signal })
root.querySelectorAll("[data-podcast-skip]").forEach((button) => {
	button.addEventListener("click", () => seek(Number.parseInt(button.dataset.podcastSkip, 10)), { signal: controller.signal })
})
progressInput?.addEventListener("input", () => {
	scrubbing = true
	setManualPosition(Number(progressInput.value))
}, { signal: controller.signal })
progressInput?.addEventListener("change", () => {
	scrubbing = false
	skipSegmentAtCurrentTime()
}, { signal: controller.signal })
audio.addEventListener("loadedmetadata", updateTimeline, { signal: controller.signal })
audio.addEventListener("loadedmetadata", renderSegments, { signal: controller.signal })
audio.addEventListener("durationchange", () => { updateTimeline(); renderSegments(); skipSegmentAtCurrentTime() }, { signal: controller.signal })
audio.addEventListener("loadstart", () => { playbackFailed = false; setLoading(true) }, { signal: controller.signal })
audio.addEventListener("canplay", () => setLoading(false), { signal: controller.signal })
audio.addEventListener("playing", () => { blocked = false; setLoading(false) }, { signal: controller.signal })
audio.addEventListener("error", () => { playbackFailed = true; scanHoldResume = false; finishScanHold(); setLoading(false) }, { signal: controller.signal })
audio.addEventListener("timeupdate", () => { updateTimeline(); skipSegmentAtCurrentTime() }, { signal: controller.signal })
audio.addEventListener("play", () => {
	if (scanHoldActive) { scanHoldResume = true; pauseForScanHold() }
	updatePlayButton()
}, { signal: controller.signal })
audio.addEventListener("pause", () => {
	if (scanHoldPauseEvents > 0) scanHoldPauseEvents--
	else if (scanHoldActive) scanHoldResume = false
	updatePlayButton()
}, { signal: controller.signal })
audio.addEventListener("ended", updatePlayButton, { signal: controller.signal })
audio.addEventListener("volumechange", () => {
	if (volumeInput) volumeInput.value = String(Math.round(audio.volume * 100))
	writePreference("player-volume", Math.round(audio.volume * 100))
	publish()
}, { signal: controller.signal })
audio.addEventListener("pause", saveProgress, { signal: controller.signal })
audio.addEventListener("ended", saveProgress, { signal: controller.signal })
;(options.externalUI ? null : document.getElementById("close-button"))?.addEventListener("click", cleanup, { signal: controller.signal })
document.addEventListener("keydown", (event) => {
	const target = event.target instanceof Element ? event.target : null
	if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || target?.closest("input, textarea, select, button, a, summary, dialog, [role=dialog], [role=menu], [role=tablist], [contenteditable=true]")) return
	if (event.key === " ") {
		event.preventDefault()
		togglePlayback()
	} else if (event.key === "ArrowLeft") {
		event.preventDefault()
		seek(-15)
	} else if (event.key === "ArrowRight") {
		event.preventDefault()
		seek(15)
	}
}, { signal: controller.signal })
if (withProgress) progressTimer = window.setInterval(saveProgress, 10000)
updateTimeline()
setLoading(loading)
if (!audio.paused && !audio.ended && audio.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) startSegmentAnalysis()
publish()
return api
}
const api = { mount };
if (typeof module !== "undefined" && module.exports) module.exports = api;
else window.FeedlrPodcastPlayer = api;
})(typeof window !== "undefined" ? window : globalThis);
