/* Feedlr's shared native / YouTube controller. No expiring URLs belong in page HTML. */
(function (global) {
  "use strict";

  const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
  const finite = (value, fallback = 0) => Number.isFinite(Number(value)) ? Number(value) : fallback;
  const read = (storage, key) => { try { return storage.getItem(key); } catch (_) { return null; } };
  const write = (storage, key, value) => { try { storage.setItem(key, String(value)); } catch (_) {} };
  const qualityKey = "feedlr-player-quality";
  const usesDeviceVolume = () => Boolean(global.navigator?.userAgentData?.mobile ||
    /Android|iPhone|iPad|iPod/i.test(global.navigator?.userAgent || "") ||
    (global.navigator?.platform === "MacIntel" && global.navigator?.maxTouchPoints > 1) ||
    global.matchMedia?.("(hover: none) and (pointer: coarse)").matches);
  const qualityPreference = (value) => value === "audio" ? "audio" : /^\d+$/.test(value) && Number(value) > 0 ? String(Number(value)) : "auto";
  function qualityHeight(track) {
    let height = track.height || 0;
    let width = track.width || 0;
    // Match familiar YouTube resolution names for portrait and ultrawide video.
    if (width > 0 && height > width) [width, height] = [height, width];
    if (height > 0 && width / height > 16 / 9) height = Math.round(width * 9 / 16);
    return height;
  }
  const qualityNames = { 1440: "2K", 2160: "4K", 2880: "5K", 4320: "8K", 8640: "16K" };
  function qualityLabel(height) { return qualityNames[height] || `${height}p`; }
  function qualityFromLabel(label) {
    const named = Object.entries(qualityNames).find(([, name]) => name === label);
    return named ? named[0] : /^\d+p$/.test(label) ? label.slice(0, -1) : null;
  }
  function initialVolume(saved, server) {
    // Older progress requests omitted volume and left a zero database default.
    // New pages start audibly; in-page switches preserve their exact snapshot.
    return clamp(finite(saved) > 0 ? finite(saved) : finite(server) > 0 ? finite(server) : 100, 0, 100);
  }
  function usableResolution(result, now = Date.now()) {
    if (result.mode !== "native") return false;
    try {
      const url = new URL(result.manifestUrl, global.location.origin);
      return url.origin === global.location.origin && url.pathname.startsWith("/api/") && Date.parse(result.expiresAt) > now;
    } catch (_) { return false; }
  }
  function expirationError(error, expiresAt, now = Date.now()) {
    return now >= expiresAt || (error?.category === 1 && (error.data || []).some((value) => [401, 403, 404, 410].includes(value)));
  }

  // Iframe commands and Shaka's initial seek are asynchronous. Keep each
  // restored value until the adapter acknowledges it, rather than saving the
  // temporary zero/default state emitted during initialization.
  function restoredState(actual, pending) {
    const state = { ...actual };
    for (const key of Object.keys(pending)) {
      const tolerance = key === "position" ? 2 : key === "volume" ? 1 : 0;
      const acknowledged = typeof pending[key] === "number"
        ? Number.isFinite(actual[key]) && Math.abs(actual[key] - pending[key]) <= tolerance
        : actual[key] === pending[key];
      if (acknowledged) delete pending[key];
      else state[key] = pending[key];
    }
    return state;
  }

  // Count only time during which the browser can actually play. Visibility,
  // connectivity, pause and autoplay policy suspend both deadlines.
  class PlaybackWatchdog {
    constructor() { this.reset(); }
    reset() { this.firstFrame = false; this.startup = 0; this.stall = 0; this.position = null; }
    tick(seconds, active, position, decoded) {
      if (!active) return null;
      const moved = this.position !== null && Math.abs(position - this.position) > 0.01;
      this.position = position;
      if (decoded || moved) this.firstFrame = true;
      if (!this.firstFrame) {
        this.startup += seconds;
        return this.startup >= 10 ? "startup_timeout" : null;
      }
      this.stall = moved ? 0 : this.stall + seconds;
      return this.stall >= 15 ? "playback_stall" : null;
    }
  }

  function applyQuality(player, quality) {
    // Clear both kinds of restrictions when returning to Auto, including any
    // previous manual selection or defaults supplied by a UI integration.
    const unrestricted = { minWidth: 0, maxWidth: Infinity, minHeight: 0, maxHeight: Infinity, minPixels: 0, maxPixels: Infinity, minBandwidth: 0, maxBandwidth: Infinity, minFrameRate: 0, maxFrameRate: Infinity };
    if (quality === "auto") {
      player.configure({ abr: { enabled: true, restrictions: unrestricted }, restrictions: unrestricted });
      return "auto";
    }
    const variants = player.getVideoTracks ? player.getVideoTracks() : player.getVariantTracks();
    const heights = variants.map(qualityHeight).filter((height) => height > 0);
    const below = heights.filter((height) => height <= Number(quality));
    // If every track exceeds the preference, the smallest is the only playable fallback.
    const height = below.length ? Math.max(...below) : Math.min(...heights);
    const tracks = variants.filter((track) => qualityHeight(track) === height);
    const active = tracks.find((track) => track.active);
    const track = active || tracks.sort((a, b) => finite(b.bandwidth) - finite(a.bandwidth))[0];
    if (!track) return applyQuality(player, "auto");
    player.configure({ abr: { enabled: false } });
    if (player.selectVideoTrack) player.selectVideoTrack(track, true, 2);
    else player.selectVariantTrack(track, true, 2);
    return String(height);
  }

  // Companion marks original audio with the DASH role "main", independently
  // of its language or YouTube's default track. Keep stereo as a preference,
  // but never let a stereo dub beat an original with a different channel count.
  const originalAudioPreferences = [{ role: "main", channelCount: 2 }, { role: "main" }];
  function audioPreference(track) {
    if (!track) return null;
    return {
      language: track.language,
      role: ["main", "dub", "description", "enhanced-audio-intelligibility", "alternate"].find((role) => track.roles?.includes(role)) || "",
      label: track.label || "", channelCount: track.channelsCount || 0, spatialAudio: track.spatialAudio,
    };
  }
  function selectInitialAudio(player, preference) {
    const tracks = player.getAudioTracks();
    const isOriginal = (track) => track.roles?.includes("main") && !track.roles.some((role) => ["dub", "description", "alternate"].includes(role));
    const matches = (track) => (!preference.language || track.language === preference.language) &&
      (!preference.role || track.roles?.includes(preference.role)) &&
      (preference.role !== "main" || isOriginal(track)) &&
      (!preference.label || track.label === preference.label) &&
      (!preference.channelCount || track.channelsCount === preference.channelCount) &&
      (preference.spatialAudio === undefined || track.spatialAudio === preference.spatialAudio);
    let candidates = preference ? tracks.filter(matches) : [];
    if (!candidates.length) candidates = tracks.filter(isOriginal);
    // Ordinary single-track Companion manifests can omit roles entirely.
    // Missing original metadata in a multilingual/alternate manifest is unsafe
    // to guess from track order, labels, or the viewer's language.
    if (!candidates.length && tracks.every((track) => !track.roles?.length) && new Set(tracks.map((track) => JSON.stringify([track.language, track.label || ""]))).size <= 1) candidates = tracks;
    const track = candidates.find((track) => track.active) || candidates[0];
    if (!track) throw new Error("unsupported_audio");
    if (!track.active) player.selectAudioTrack(track);
  }

  const scripts = new Map();
  function loadScript(src) {
    if (scripts.has(src)) return scripts.get(src);
    const promise = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      const timeout = global.setTimeout(() => { script.remove(); reject(new Error("script_timeout")); }, 10000);
      script.src = src;
      script.onload = () => { global.clearTimeout(timeout); resolve(); };
      script.onerror = () => { global.clearTimeout(timeout); script.remove(); reject(new Error("script_failed")); };
      document.head.append(script);
    }).catch((error) => { scripts.delete(src); throw error; });
    scripts.set(src, promise);
    return promise;
  }
  async function loadShaka() {
    if (!document.querySelector('link[data-feedlr-player]')) {
      const css = document.createElement("link");
      css.rel = "stylesheet";
      css.href = "/assets/css/player.css";
      css.dataset.feedlrPlayer = "true";
      document.head.append(css);
    }
    if (!global.shaka?.ui) {
      if (!document.querySelector('link[data-feedlr-shaka]')) {
        const css = document.createElement("link");
        css.rel = "stylesheet";
        css.href = "/assets/vendor/controls.css";
        css.dataset.feedlrShaka = "true";
        document.head.append(css);
      }
      await loadScript("/assets/vendor/shaka-player.ui.js");
    }
    global.shaka.polyfill.installAll();
    if (!global.shaka.Player.isBrowserSupported()) throw new Error("unsupported_browser");
  }
  let youtubeReady;
  function loadYouTube() {
    if (global.YT?.Player) return Promise.resolve();
    if (youtubeReady) return youtubeReady;
    youtubeReady = new Promise((resolve, reject) => {
      const prior = global.onYouTubeIframeAPIReady;
      const timeout = global.setTimeout(() => reject(new Error("youtube_api_timeout")), 12000);
      global.onYouTubeIframeAPIReady = () => {
        global.clearTimeout(timeout);
        try { prior?.(); } finally { resolve(); }
      };
      loadScript("https://www.youtube.com/iframe_api").catch((error) => { global.clearTimeout(timeout); reject(error); });
    }).catch((error) => { youtubeReady = null; throw error; });
    return youtubeReady;
  }

  const iframeMenuActions = new WeakMap();
  let iframeMenuRegistered = false;
  function registerIframeMenu() {
    if (iframeMenuRegistered) return;
    class YouTubeMenuButton extends global.shaka.ui.Element {
      constructor(parent, controls) {
        super(parent, controls);
        this.button = document.createElement("button");
        this.button.type = "button";
        this.button.className = "feedlr-youtube-menu-button";
        this.button.setAttribute("aria-label", "Use YouTube player");
        this.button.setAttribute("role", "menuitem");
        const icon = document.getElementById("feedlr-iframe-menu-icon")?.content.cloneNode(true);
        if (icon) this.button.append(icon);
        const label = document.createElement("span");
        label.className = "shaka-overflow-button-label shaka-overflow-menu-only shaka-simple-overflow-button-label-inline";
        label.textContent = "Use YouTube player";
        this.button.append(label);
        parent.append(this.button);
        this.eventManager.listen(this.button, "click", () => {
          controls.hideSettingsMenus();
          iframeMenuActions.get(controls.getLocalPlayer())?.();
        });
        this.checkAvailability();
      }
      checkAvailability() {
        this.button.classList.toggle("shaka-hidden", this.isSubMenuOpened);
      }
      release() {
        this.button.remove();
        super.release();
      }
    }
    global.shaka.ui.OverflowMenu.registerElement("feedlr_youtube", {
      create: (parent, controls) => new YouTubeMenuButton(parent, controls),
    }, false);
    iframeMenuRegistered = true;
  }

  class Controller {
    constructor(options) {
      this.options = options;
      this.root = document.getElementById("player");
      this.loading = document.getElementById("player-loading");
      this.abort = new AbortController();
      this.generation = 0;
      this.disposed = false;
      this.ready = false;
      this.switching = false;
      this.quality = qualityPreference(read(global.localStorage, qualityKey));
      this.availableQualities = [];
      this.audioOnly = false;
      // Scoped to this video/page; a different video always starts original.
      this.audioPreference = null;
      this.deviceVolume = usesDeviceVolume();
      this.mode = options.authenticated ? "native" : "iframe";
      this.lastProgress = -1;
      this.progressWrites = new Set();
      this.tvButton = options.authenticated ? document.getElementById("send-to-tv-btn") : null;
      this.tvOnline = false;
      this.tvSending = false;
      this.tvHandedOff = false;
      this.expiresAt = Infinity;
      this.renewAt = Infinity;
      this.expiryRetryUsed = false;
      this.watchdog = new PlaybackWatchdog();
      const savedVolume = read(global.localStorage, `player-volume-${options.channel}`) ?? read(global.localStorage, "player-volume");
      this.state = { position: finite(options.progress), playing: true, volume: this.deviceVolume ? 100 : initialVolume(savedVolume, options.volume), muted: false, rate: 1 };
      this.listen(document.getElementById("close-button"), "click", () => this.cleanup());
      this.listen(this.tvButton, "click", () => this.sendToTV());
      this.listen(document, "keydown", (event) => this.hotkey(event));
      this.listen(document, "htmx:beforeSwap", (event) => {
        if (event.detail?.target?.contains(this.root)) this.cleanup();
      });
      this.listen(document, "htmx:historyRestore", () => {
        if (document.getElementById("player") !== this.root) this.cleanup();
      });
      this.listen(global, "pagehide", (event) => {
        this.stopTVStatus();
        if (event.persisted) {
          this.historyState = this.snapshot();
          this.saveProgress();
          this.adapter?.pause();
        } else this.cleanup();
      });
      this.listen(global, "pageshow", (event) => {
        if (event.persisted && !this.disposed) this.switchPlayer(this.mode, this.historyState || this.snapshot(), "history_restore");
        this.startTVStatus();
      });
      this.listen(document, "visibilitychange", () => {
        if (!document.hidden) { this.checkRenewal(); this.startTVStatus(); }
        else { this.saveProgress(); this.stopTVStatus(); }
      });
      this.listen(global, "online", () => { this.checkRenewal(); this.startTVStatus(); });
      this.listen(global, "offline", () => this.stopTVStatus());
      this.lastTick = performance.now();
      this.timer = global.setInterval(() => this.tick(), 500);
      this.progressTimer = options.withProgress ? global.setInterval(() => this.saveProgress(), 10000) : null;
      this.start = this.switchPlayer(this.mode, null, "initial");
      this.startTVStatus();
    }

    listen(target, event, callback) { target?.addEventListener(event, callback, { signal: this.abort.signal }); }
    updateTVButton() {
      if (!this.tvButton) return;
      this.tvButton.hidden = !this.tvOnline || this.disposed;
      this.tvButton.disabled = !this.tvOnline || this.tvSending || !this.ready || this.switching || this.disposed;
      const label = this.tvScreenName ? `Send to TV (${this.tvScreenName})` : "Send to TV";
      this.tvButton.title = label;
      this.tvButton.setAttribute("aria-label", label);
      this.tvButton.setAttribute("aria-busy", String(this.tvSending));
    }
    startTVStatus() {
      if (!this.tvButton || this.disposed || document.hidden || global.navigator.onLine === false) return;
      if (!this.tvTimer) this.tvTimer = global.setInterval(() => this.refreshTVStatus(), 15000);
      this.refreshTVStatus();
    }
    stopTVStatus() {
      global.clearInterval(this.tvTimer);
      this.tvTimer = null;
      global.clearTimeout(this.tvStatusTimeout);
      this.tvStatusAbort?.abort();
      this.tvStatusAbort = null;
      this.tvOnline = false;
      this.updateTVButton();
    }
    async refreshTVStatus() {
      if (!this.tvButton || this.disposed || document.hidden || global.navigator.onLine === false || this.tvStatusAbort) return;
      const request = this.tvStatusAbort = new AbortController();
      const timeout = this.tvStatusTimeout = global.setTimeout(() => request.abort(), 6000);
      try {
        const response = await fetch("/api/tv/status", { credentials: "same-origin", cache: "no-store", signal: request.signal });
        if (!response.ok) throw new Error("tv_status_failed");
        const status = await response.json();
        if (this.disposed || request.signal.aborted || this.tvStatusAbort !== request) return;
        this.tvOnline = status.online === true;
        this.tvScreenName = typeof status.screenName === "string" ? status.screenName : "";
      } catch (_) {
        if (this.tvStatusAbort === request) this.tvOnline = false;
      } finally {
        global.clearTimeout(timeout);
        if (this.tvStatusAbort === request) {
          this.tvStatusAbort = null;
          this.updateTVButton();
        }
      }
    }
    async sendToTV() {
      if (!this.tvButton || !this.tvOnline || this.tvSending || !this.ready || this.switching || this.disposed) return;
      this.tvSending = true;
      this.updateTVButton();
      let complete;
      this.tvSendCompletion = new Promise((resolve) => { complete = resolve; });
      const request = this.tvSendAbort = new AbortController();
      const timeout = this.tvSendTimeout = global.setTimeout(() => request.abort(), 15000);
      try {
        // Complete older progress writes before handing off. From here until
        // local playback resumes, pause/timer/cleanup must not overwrite the TV.
        await Promise.race([
          Promise.all(this.progressWrites),
          new Promise((_, reject) => request.signal.addEventListener("abort", () => reject(new Error("tv_send_cancelled")), { once: true })),
        ]);
        if (this.disposed || request.signal.aborted) return;
        const response = await fetch(`/api/videos/${encodeURIComponent(this.options.video)}/tv`, {
          method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ position: this.snapshot().position }), signal: request.signal,
        });
        if (response.status !== 204 || response.redirected) {
          const result = await response.json().catch(() => ({}));
          throw new Error(typeof result.error === "string" ? result.error : "Could not send video to TV. Try again.");
        }
        if (this.disposed || request.signal.aborted) return;
        this.tvHandedOff = true;
        this.lastProgress = Math.floor(this.snapshot().position);
        this.state.playing = false;
        if (this.historyState) this.historyState.playing = false;
        this.adapter?.pause();
        this.notice("Sent to TV");
      } catch (error) {
        if (!this.disposed) {
          this.notice(request.signal.aborted ? "Could not send video to TV. Try again." : error.message || "Could not send video to TV. Try again.");
          this.refreshTVStatus();
        }
      } finally {
        global.clearTimeout(timeout);
        request.abort();
        this.tvSendAbort = null;
        this.tvSending = false;
        this.tvSendCompletion = null;
        complete();
        if (!this.disposed) this.updateTVButton();
      }
    }
    metric(event, reason, seconds) {
      if (!this.options.authenticated) return;
      fetch(`/api/videos/${encodeURIComponent(this.options.video)}/playback/events`, {
        method: "POST", credentials: "same-origin", keepalive: true,
        headers: { "Content-Type": "application/json" }, body: JSON.stringify({ event, reason, seconds }),
      }).catch(() => {});
    }
    snapshot() {
      if (this.adapter && this.ready) {
        const current = this.adapter.snapshot();
        this.state = { ...this.state, ...current, position: Math.max(0, finite(current.position, this.state.position)) };
      }
      return { ...this.state };
    }
    showLoading(loading) { this.loading?.classList.toggle("hidden", !loading); }
    notice(message) {
      const toast = document.getElementById("notification-toast");
      const text = toast?.querySelector("span");
      if (text) text.textContent = message;
      toast?.classList.remove("opacity-0");
      global.clearTimeout(this.toastTimer);
      this.toastTimer = global.setTimeout(() => toast?.classList.add("opacity-0"), 2500);
    }
    async resolve(mode, signal) {
      const response = await fetch(`/api/videos/${encodeURIComponent(this.options.video)}/playback`, {
        method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode, audioOnly: this.quality === "audio" }), signal,
      });
      if (!response.ok) throw new Error("resolution_failed");
      return response.json();
    }
    async destroyAdapter() {
      // Detach listeners before pause/destroy: teardown must never save a zero.
      this.playerAbort?.abort();
      this.playerCleanup?.();
      this.playerCleanup = null;
      const adapter = this.adapter;
      if (this.ready && adapter?.player) {
        // Shaka's language menu changes preferredAudio on an explicit choice.
        // Do not turn an automatically selected, unmarked legacy track into a
        // language preference that could outrank original audio on renewal.
        const configured = adapter.player.getConfiguration().preferredAudio[0];
        this.audioPreference = configured.role === "main" && !configured.language && !configured.label ? null :
          audioPreference(adapter.player.getAudioTracks().find((track) => track.active));
      }
      this.adapter = null;
      this.ready = false;
      const previous = this.teardown || Promise.resolve();
      this.teardown = previous.then(async () => { if (adapter) { try { await adapter.destroy(); } catch (_) {} } });
      await this.teardown;
    }
    async switchPlayer(requestedMode, preservedState, reason) {
      if (this.tvSendCompletion) {
        // A replacement must not retain an autoplay request from before the
        // handoff. Keep the current adapter until the send has settled, then
        // capture its latest position and desired playback state.
        await this.tvSendCompletion;
        if (preservedState) preservedState = this.snapshot();
      }
      if (this.disposed) return;
      this.saveProgress();
      const generation = ++this.generation;
      const current = () => !this.disposed && generation === this.generation;
      this.resolveAbort?.abort();
      this.resolveAbort = new AbortController();
      const requestAbort = this.resolveAbort;
      this.switching = true;
      this.updateTVButton();
      if (reason === "renewal") this.metric("renewal", "scheduled");
      if (reason === "manual_switch") this.metric("switch", requestedMode);
      if (preservedState) this.state = { ...preservedState };
      if (this.deviceVolume) this.state.volume = 100;
      this.mode = requestedMode;
      this.blocked = false;
      this.showLoading(true);
      await this.destroyAdapter();
      if (!current()) return;
      this.root.replaceChildren();
      let resolution = { mode: "iframe", reason: "guest", progress: this.state.position };
      if (this.options.authenticated) {
        const timeout = global.setTimeout(() => requestAbort.abort(), 9000);
        try { resolution = await this.resolve(requestedMode, requestAbort.signal); }
        catch (_) { resolution = { mode: "iframe", reason: "resolution_failed", progress: this.state.position }; }
        finally { global.clearTimeout(timeout); }
        if (!current()) return;
      }
      if (!preservedState) this.state.position = Math.max(0, finite(resolution.progress, this.state.position));
      if (requestedMode === "native" && resolution.mode === "native" && !usableResolution(resolution)) {
        resolution = { ...resolution, mode: "iframe", reason: "invalid_resolution" };
      }
      this.mode = requestedMode === "native" ? resolution.mode : "iframe";
      try {
        if (this.mode === "native") {
          this.audioOnly = resolution.audioOnly === true;
          if (Array.isArray(resolution.qualities)) {
            this.availableQualities = [...new Set(resolution.qualities.map(qualityHeight).filter((height) => height > 0))].sort((a, b) => b - a);
          }
          this.expiresAt = Date.parse(resolution.expiresAt);
          this.renewAt = Math.max(Date.now() + 30000, this.expiresAt - 120000);
          this.watchdog.reset();
          this.started = false;
          this.resolvedAt = performance.now();
          this.monitoring = true;
          await this.createNative(resolution, current);
        } else {
          this.monitoring = false;
          if (requestedMode === "native" || reason === "fallback") {
            this.notice("Switched to YouTube player");
            if (reason !== "fallback") this.metric("fallback", resolution.reason || "server_fallback");
          }
          await this.createIframe(current);
        }
      } catch (error) {
        if (!current()) return;
        if (this.mode === "native") {
          await this.nativeError(error);
          return;
        }
        this.notice("YouTube player could not load. Reload to try again.");
        this.showLoading(false);
        this.metric("media_error", "iframe_failed");
      }
      if (!current()) return;
      this.switching = false;
      this.updateTVButton();
    }

    async createNative(resolution, current) {
      await loadShaka();
      if (!current()) return;
      const box = document.createElement("div");
      box.style.cssText = "position:absolute;inset:0;background:#000";
      const video = document.createElement("video");
      video.style.cssText = "width:100%;height:100%;object-fit:contain";
      video.playsInline = true;
      video.controls = false;
      // Check Shaka's playable audio choices before allowing autoplay. A role
      // preference alone falls back to a dub if the original is unsupported.
      video.autoplay = false;
      video.volume = this.state.volume / 100;
      video.muted = this.state.muted;
      box.append(video);
      let waveform;
      if (this.audioOnly) {
        video.style.opacity = "0";
        waveform = document.createElement("div");
        waveform.className = "feedlr-audio-visual";
        waveform.dataset.playing = "false";
        waveform.setAttribute("aria-label", "Audio only");
        const bars = document.createElement("div");
        bars.className = "feedlr-audio-wave";
        bars.setAttribute("aria-hidden", "true");
        for (let i = 0; i < 24; i++) {
          const bar = document.createElement("span");
          bar.style.setProperty("--bar", String(i));
          bars.append(bar);
        }
        waveform.append(bars);
        box.append(waveform);
      }
      this.root.append(box);
      const player = new global.shaka.Player();
      let destroyed = false;
      registerIframeMenu();
      iframeMenuActions.set(player, () => {
        if (current() && !destroyed && this.ready && !this.switching) {
          this.switchPlayer("iframe", this.snapshot(), "manual_switch");
        }
      });
      const ui = new global.shaka.ui.Overlay(player, box, video);
      ui.configure({
        overflowMenuButtons: ["quality", "language", "playback_rate", "picture_in_picture", "feedlr_youtube"],
        trackLabelFormat: global.shaka.ui.Overlay.TrackLabelFormat.LABEL_OR_LANGUAGE,
        controlPanelElements: [...(this.deviceVolume ? [] : ["play_pause"]), "time_and_duration", "spacer", ...(this.deviceVolume ? [] : ["mute", "volume"]), "overflow_menu", "fullscreen"],
        alwaysShowVolumeBar: true,
        singleClickForPlayAndPause: !this.deviceVolume,
        doubleClickForFullscreen: !this.deviceVolume,
        seekOnTaps: this.deviceVolume,
        tapSeekDistance: 10,
        enableFullscreenOnRotation: false,
        bigButtons: this.deviceVolume ? ["play_pause_buffering"] : [],
        qualityMarks: { 720: "", 1080: "", 1440: "", 2160: "", 4320: "" },
        customTrackLabel: (label, track, type) => type === "video" && track.height ? qualityLabel(qualityHeight(track)) : label,
      });
      ui.setEnabled(true);
      this.playerAbort = new AbortController();
      const signal = this.playerAbort.signal;
      const listen = (event, callback) => video.addEventListener(event, callback, { signal });
      const pending = { ...this.state };
      this.adapter = {
        video, player, waveform,
        snapshot: () => restoredState({ position: video.currentTime, playing: !video.paused && !video.ended, volume: video.volume * 100, muted: video.muted, rate: player.getPlaybackRate?.() || this.state.rate }, pending),
        play: () => { pending.playing = true; return this.playNative(video, pending); }, pause: () => { pending.playing = false; video.pause(); },
        seek: (time) => { pending.position = clamp(time, 0, Number.isFinite(video.duration) ? video.duration : Infinity); video.currentTime = pending.position; },
        volume: (value) => { pending.volume = this.deviceVolume ? 100 : clamp(value, 0, 100); video.volume = pending.volume / 100; },
        destroy: async () => {
          destroyed = true;
          iframeMenuActions.delete(player);
          video.pause();
          // Overlay.destroy owns and destroys its Shaka Player as well.
          await ui.destroy();
          video.removeAttribute("src");
          video.load();
        },
      };
      const onError = (event) => {
        if (current() && !destroyed && event.detail?.severity === 2) this.nativeError(event.detail);
      };
      player.addEventListener("error", onError);
      const controls = ui.getControls();
      const renderQualityMenu = () => {
        const menu = box.querySelector(".shaka-resolutions");
        if (!menu || !current() || destroyed) return;
        for (const button of menu.querySelectorAll("[data-feedlr-quality]")) button.remove();
        if (this.audioOnly) {
          // Shaka's default audio quality entries are bitrates. This menu keeps
          // the same video choices so listening can switch back to watching.
          for (const button of menu.querySelectorAll(".explicit-resolution, .shaka-enable-abr-button")) button.remove();
        } else {
          const seen = new Map();
          for (const button of menu.querySelectorAll(".explicit-resolution")) {
            const label = button.querySelector("span")?.textContent;
            const previous = seen.get(label);
            if (previous && button.getAttribute("aria-checked") !== "true") button.remove();
            else { previous?.remove(); seen.set(label, button); }
          }
          const active = player.getVariantTracks().find((track) => track.active);
          if (active?.height) {
            for (const label of box.querySelectorAll(".shaka-current-auto-quality")) label.textContent = qualityLabel(qualityHeight(active));
          }
        }
        const addChoice = (value, label, selected = false) => {
          const button = document.createElement("button");
          button.type = "button";
          button.dataset.feedlrQuality = value;
          button.setAttribute("role", "menuitemradio");
          button.setAttribute("aria-checked", String(selected));
          const text = document.createElement("span");
          text.textContent = label;
          button.append(text);
          if (selected) {
            const check = document.createElement("span");
            check.textContent = "✓";
            check.setAttribute("aria-hidden", "true");
            button.append(check);
          }
          menu.append(button);
        };
        if (this.audioOnly) {
          for (const height of this.availableQualities) addChoice(String(height), qualityLabel(height));
          addChoice("auto", "Auto");
          const button = box.querySelector(".shaka-resolution-button");
          button?.setAttribute("shaka-status", "Audio only");
          const selection = button?.querySelector(".shaka-current-selection-span");
          if (selection) selection.textContent = "Audio only";
          for (const label of box.querySelectorAll(".shaka-current-auto-quality")) label.style.display = "none";
        }
        addChoice("audio", "Audio only", this.audioOnly);
      };
      let menuUpdatePending = false;
      const updateQualityLabels = () => {
        if (menuUpdatePending) return;
        menuUpdatePending = true;
        // Shaka writes its own labels after dispatching the menu event.
        Promise.resolve().then(() => { menuUpdatePending = false; renderQualityMenu(); });
      };
      controls.addEventListener("resolutionselectionupdated", updateQualityLabels);
      player.addEventListener("adaptation", updateQualityLabels);
      box.addEventListener("click", (event) => {
        const button = event.target.closest?.("[data-feedlr-quality], .explicit-resolution, .shaka-enable-abr-button");
        if (!button?.closest(".shaka-resolutions")) return;
        const custom = button.dataset.feedlrQuality;
        if (custom) {
          event.preventDefault();
          event.stopImmediatePropagation();
          this.selectQuality(custom);
        } else if (button.classList.contains("shaka-enable-abr-button")) {
          this.selectQuality("auto");
        } else {
          // Record the clicked preference, not the old, still-active track.
          const selected = qualityFromLabel(button.querySelector("span")?.textContent);
          if (selected) {
            this.quality = selected;
            write(global.localStorage, qualityKey, selected);
          }
        }
      }, { capture: true, signal });
      this.playerCleanup = () => {
        player.removeEventListener("error", onError);
        player.removeEventListener("adaptation", updateQualityLabels);
        controls.removeEventListener("resolutionselectionupdated", updateQualityLabels);
      };
      listen("play", () => { pending.playing = true; this.state.playing = true; this.tvHandedOff = false; this.blocked = false; this.checkRenewal(); });
      listen("pause", () => { if (this.ready) { pending.playing = false; this.state.playing = false; this.saveProgress(); } });
      listen("ended", () => { pending.playing = false; this.state.playing = false; this.saveProgress(); });
      listen("playing", () => { this.showLoading(false); });
      listen("volumechange", () => this.saveVolume());
      listen("error", () => { if (current() && !destroyed) this.nativeError({ category: 3, code: video.error?.code }); });
      player.configure({ preferredAudio: [...(this.audioPreference ? [this.audioPreference] : []), ...originalAudioPreferences], streaming: { bufferingGoal: 30, rebufferingGoal: 2, retryParameters: { maxAttempts: 2, timeout: 10000, connectionTimeout: 5000, stallTimeout: 5000 } }, manifest: { retryParameters: { maxAttempts: 1, timeout: 8000 } } });
      await player.attach(video);
      if (!current()) return;
      await player.load(resolution.manifestUrl, this.state.position);
      if (!current()) return;
      selectInitialAudio(player, this.audioPreference);
      // Shaka 5.2 only includes labels in audio-menu deduplication in LABEL
      // mode. Keep human and AI dubs in the same language separately selectable.
      ui.configure({ trackLabelFormat: player.getAudioTracks().every((track) => track.label) ? global.shaka.ui.Overlay.TrackLabelFormat.LABEL : global.shaka.ui.Overlay.TrackLabelFormat.LABEL_OR_LANGUAGE });
      video.autoplay = this.state.playing;
      const heights = [...new Set(player.getVariantTracks().map((track) => track.height).filter((height) => height > 0))].sort((a, b) => b - a);
      if (!this.audioOnly) {
        if (!heights.length) throw new Error("unsupported_codec");
        this.availableQualities = [...new Set(player.getVariantTracks().map(qualityHeight).filter((height) => height > 0))].sort((a, b) => b - a);
      }
      // Loading clears Shaka's media session. Restore metadata for every new
      // player, including URL renewals and switches to/from audio-only playback.
      const mediaSession = controls.getMediaSession();
      mediaSession.setupTitle(this.options.title || "");
      mediaSession.setupArtist(this.options.channelTitle || "");
      mediaSession.setupPoster(new URL(`/thumb/video/${encodeURIComponent(this.options.video)}/hqdefault`, global.location.origin).href);
      applyQuality(player, this.audioOnly ? "auto" : this.quality);
      updateQualityLabels();
      video.volume = this.state.volume / 100;
      video.muted = this.state.muted;
      try { player.trickPlay(this.state.rate, false); } catch (_) { pending.rate = 1; player.trickPlay(1, false); }
      this.ready = true;
      this.showLoading(false);
      if (video.requestVideoFrameCallback) video.requestVideoFrameCallback(() => { if (current()) this.watchdog.firstFrame = true; });
      if (this.state.playing) await this.playNative(video, pending);
      this.snapshot();
    }
    selectQuality(value) {
      const quality = qualityPreference(value);
      this.quality = quality;
      write(global.localStorage, qualityKey, quality);
      if ((quality === "audio") !== this.audioOnly) {
        return this.switchPlayer("native", this.snapshot(), "quality_switch");
      }
      if (!this.audioOnly && this.adapter?.player) applyQuality(this.adapter.player, quality);
      return Promise.resolve();
    }
    async playNative(video, pending) {
      this.state.playing = true;
      try { await video.play(); }
      catch (error) {
        if (error.name === "NotAllowedError") {
          this.blocked = true;
          pending.playing = false;
          this.state.playing = false;
          this.showLoading(false);
        } else if (error.name !== "AbortError") await this.nativeError(error);
      }
    }

    async createIframe(current) {
      await loadYouTube();
      if (!current()) return;
      const frame = document.createElement("div");
      frame.style.cssText = "position:absolute;inset:0;width:100%;height:100%";
      this.root.append(frame);
      await new Promise((resolve, reject) => {
        let destroyed = false;
        const pending = { ...this.state };
        const timer = global.setTimeout(() => reject(new Error("iframe_timeout")), 15000);
        const player = new global.YT.Player(frame, {
          height: "100%", width: "100%", videoId: this.options.video,
          playerVars: { start: Math.floor(this.state.position), autoplay: this.state.playing ? 1 : 0, playsinline: 1, rel: 0, enablejsapi: 1, iv_load_policy: 3, origin: global.location.origin },
          events: {
            onReady: () => {
              if (!current() || destroyed) return;
              global.clearTimeout(timer);
              player.setVolume(this.state.volume);
              if (this.state.muted) player.mute(); else player.unMute();
              const rates = player.getAvailablePlaybackRates();
              if (rates.includes(this.state.rate)) player.setPlaybackRate(this.state.rate);
              else pending.rate = 1;
              player.seekTo(this.state.position, true);
              if (this.state.playing) player.playVideo(); else player.pauseVideo();
              try { player.unloadModule("captions"); } catch (_) {}
              this.ready = true;
              this.showLoading(false);
              resolve();
            },
            onStateChange: (event) => {
              if (!current() || destroyed || !this.ready) return;
              if ([0, 1, 2].includes(event.data)) this.state.playing = event.data === 1;
              if (event.data === 1) this.tvHandedOff = false;
              this.saveProgress();
            },
            onError: () => { if (current() && !destroyed) { global.clearTimeout(timer); reject(new Error("iframe_failed")); } },
            onAutoplayBlocked: () => { if (current()) { pending.playing = false; this.state.playing = false; } },
          },
        });
        this.adapter = {
          snapshot: () => restoredState({ position: player.getCurrentTime(), playing: [1, 3].includes(player.getPlayerState()), volume: player.getVolume(), muted: player.isMuted(), rate: player.getPlaybackRate() }, pending),
          play: () => { pending.playing = true; player.playVideo(); }, pause: () => { pending.playing = false; player.pauseVideo(); },
          seek: (time) => { pending.position = Math.max(0, time); player.seekTo(pending.position, true); }, volume: (value) => { pending.volume = this.deviceVolume ? 100 : clamp(value, 0, 100); player.setVolume(pending.volume); },
          destroy: async () => { destroyed = true; global.clearTimeout(timer); player.destroy(); resolve(); },
        };
      });
    }

    async nativeError(error) {
      if (this.mode !== "native" || this.disposed || this.handlingError) return;
      this.handlingError = true;
      const state = this.snapshot();
      const expired = expirationError(error, this.expiresAt);
      const reason = expired ? "expired_media" : (["unsupported_browser", "unsupported_codec"].includes(error.message) ? error.message : "native_error");
      this.metric("media_error", reason);
      if (expired && !this.expiryRetryUsed) {
        this.expiryRetryUsed = true;
        // Release before awaiting so a failed fresh player can itself fall back.
        this.handlingError = false;
        await this.switchPlayer("native", state, "expiry_retry");
      } else {
        this.handlingError = false;
        await this.fallback(reason, state);
      }
    }
    fallback(reason, state = this.snapshot()) {
      if (this.mode !== "native" || this.disposed) return Promise.resolve();
      this.monitoring = false;
      this.metric("fallback", reason);
      return this.switchPlayer("iframe", state, "fallback");
    }
    checkRenewal() {
      if (this.mode !== "native" || !this.ready || this.switching || this.tvSending || this.disposed || document.hidden || global.navigator.onLine === false) return;
      if (this.snapshot().playing && Date.now() >= this.renewAt) {
        this.renewAt = Infinity;
        this.switchPlayer("native", this.snapshot(), "renewal");
      }
    }
    tick() {
      const now = performance.now();
      // Discard scheduler suspension rather than treating a sleeping device as a stall.
      const elapsed = Math.min(1, (now - this.lastTick) / 1000);
      this.lastTick = now;
      if (this.disposed) return;
      const state = this.snapshot();
      if (this.mode === "native" && this.monitoring) {
        const active = state.playing && !this.blocked && !document.hidden && global.navigator.onLine !== false;
        const hadFrame = this.started;
        const reason = this.watchdog.tick(elapsed, active, state.position, this.watchdog.firstFrame);
        if (this.watchdog.firstFrame && !hadFrame) {
          this.started = true;
          this.metric("startup", "native", (performance.now() - this.resolvedAt) / 1000);
        }
        if (reason) { this.fallback(reason); return; }
        this.checkRenewal();
      }
      if (this.ready && state.playing) {
        const segment = (this.options.segments || []).find((item) => state.position >= item.start && state.position < item.end);
        if (segment) { this.adapter.seek(segment.end); this.notice("SponsorBlock skipped a video segment"); }
      }
    }
    saveVolume() {
      if (this.deviceVolume || !this.ready || !this.adapter) return;
      const volume = Math.round(this.adapter.snapshot().volume);
      write(global.localStorage, "player-volume", volume);
      write(global.localStorage, `player-volume-${this.options.channel}`, volume);
    }
    saveProgress() {
      if (!this.ready || !this.adapter || this.disposed) return;
      this.saveVolume();
      if (!this.options.withProgress || this.tvSending || this.tvHandedOff) return;
      const position = Math.floor(this.snapshot().position);
      if (position <= 0 || position === this.lastProgress) return;
      this.lastProgress = position;
      const request = fetch(`/api/videos/${encodeURIComponent(this.options.video)}/progress?progress=${position}&volume=${Math.round(this.snapshot().volume)}`, { method: "POST", credentials: "same-origin", keepalive: true }).catch(() => {}).finally(() => this.progressWrites.delete(request));
      this.progressWrites.add(request);
    }
    hotkey(event) {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.target?.closest?.("input,textarea,select,button,a,[contenteditable=true]")) return;
      if (event.key === "Escape") {
        event.preventDefault();
        this.cleanup();
        global.location.href = this.options.returnURL;
        return;
      }
      if (!this.ready || !this.adapter) return;
      if (this.deviceVolume && ["ArrowUp", "ArrowDown"].includes(event.key)) return;
      const state = this.snapshot();
      const actions = {
        " ": () => state.playing ? this.adapter.pause() : this.adapter.play(),
        ArrowLeft: () => this.adapter.seek(state.position - 5), ArrowRight: () => this.adapter.seek(state.position + 5),
        ArrowUp: () => this.adapter.volume(state.volume + 5), ArrowDown: () => this.adapter.volume(state.volume - 5),
      };
      if (actions[event.key]) { event.preventDefault(); actions[event.key](); }
    }
    cleanup() {
      if (this.disposed) return;
      this.saveProgress();
      this.disposed = true;
      this.generation++;
      this.abort.abort();
      this.resolveAbort?.abort();
      this.stopTVStatus();
      this.tvSendAbort?.abort();
      global.clearTimeout(this.tvSendTimeout);
      global.clearInterval(this.timer);
      global.clearInterval(this.progressTimer);
      global.clearTimeout(this.toastTimer);
      this.destroyAdapter();
      if (global.feedlr_player === this) delete global.feedlr_player;
    }
    // The existing integration surface remains available for other page scripts.
    playVideo() { this.adapter?.play(); }
    pauseVideo() { this.adapter?.pause(); }
    seekTo(time) { this.adapter?.seek(time); }
    getCurrentTime() { return this.snapshot().position; }
    getPlayerState() { return this.snapshot().playing ? 1 : 2; }
    getVolume() { return this.snapshot().volume; }
    setVolume(value) { this.adapter?.volume(value); }
  }

  function mount(options) {
    global.feedlr_player?.cleanup?.();
    global.feedlrPodcastPlayer?.cleanup?.();
    const controller = new Controller(options);
    global.feedlr_player = controller;
    return controller;
  }
  const api = { mount, Controller, PlaybackWatchdog, usableResolution, expirationError, applyQuality, restoredState, initialVolume, qualityLabel, qualityPreference, qualityHeight, qualityFromLabel };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else {
    global.FeedlrPlayer = api;
    const pending = global.feedlrPendingPlayer;
    delete global.feedlrPendingPlayer;
    // Do not revive an obsolete page if navigation completed during the load.
    if (pending?.root && pending.root === document.getElementById("player")) mount(pending.options);
  }
})(typeof window !== "undefined" ? window : globalThis);
