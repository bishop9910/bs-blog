/**
 * Music player store.
 *
 * The <audio> element lives here rather than inside the player component, so
 * playback survives route changes, the panel can collapse without interrupting
 * the track, and exactly one element owns the stream. Everything else in the UI
 * (waveform, lyrics, playlist) is a view over this state.
 *
 * Web Audio is wired lazily on the first play, because an AudioContext may only
 * be resumed from a user gesture. It is also strictly optional: if the graph
 * cannot be built the element keeps playing and the waveform falls back to its
 * idle animation.
 */
import { computed, readonly, ref, shallowRef, watch } from 'vue'
import { getMusicLibrary, getTrackLyrics, mediaUrl } from '@/api/music'
import { findActiveLine, parseLyrics, type LrcLine } from '@/utils/lrc'
import type { MusicLibrary, MusicTrack } from '@/types/music'

export type PlayMode = 'order' | 'loop' | 'single' | 'shuffle'

export const PLAY_MODES: PlayMode[] = ['order', 'loop', 'single', 'shuffle']

export const PLAY_MODE_LABEL: Record<PlayMode, string> = {
  order: '顺序播放',
  loop: '列表循环',
  single: '单曲循环',
  shuffle: '随机播放',
}

const STORAGE_KEY = 'bs-blog.music'

/* -------------------------------------------------------------------------- */
/* Persisted preferences                                                      */
/* -------------------------------------------------------------------------- */

interface StoredSession {
  volume?: number
  muted?: boolean
  mode?: PlayMode
  expanded?: boolean
  trackId?: string
}

function readStored(): StoredSession {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed: unknown = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as StoredSession) : {}
  } catch {
    return {}
  }
}

const stored = readStored()

/* -------------------------------------------------------------------------- */
/* State                                                                      */
/* -------------------------------------------------------------------------- */

const library = ref<MusicLibrary | null>(null)
const loading = ref(false)
const loadError = ref('')
const loaded = ref(false)

const queue = ref<string[]>([])
const currentId = ref<string>(stored.trackId ?? '')

const playing = ref(false)
const buffering = ref(false)
const playError = ref('')

/** Seconds. Mirrors the element rather than the other way round. */
const position = ref(0)
const duration = ref(0)

const volume = ref(typeof stored.volume === 'number' ? stored.volume : 0.8)
const muted = ref(stored.muted ?? false)
const mode = ref<PlayMode>(PLAY_MODES.includes(stored.mode as PlayMode) ? (stored.mode as PlayMode) : 'loop')

/** Panel state travels with the store so a route change does not re-collapse it. */
const expanded = ref(stored.expanded ?? false)
const showPlaylist = ref(false)

const lyrics = shallowRef<LrcLine[]>([])
const plainLyrics = shallowRef<string[]>([])
const lyricsLoading = ref(false)

/** 0…1 bass energy, published for the animated panel background. */
const level = ref(0)

/**
 * Decorations that react to the audio (the cover halo) subscribe here instead
 * of running a requestAnimationFrame of their own, so the spectrum is read once
 * per frame no matter how many of them are on screen.
 */
const levelListeners = new Set<(level: number) => void>()

/**
 * Subscribe to the bass energy the player publishes each frame. Returns the
 * unsubscribe function. The listener also receives 0 when playback stops, so a
 * reaction can settle back to its resting state.
 */
export function onMusicLevel(listener: (level: number) => void): () => void {
  levelListeners.add(listener)
  listener(level.value)
  return () => {
    levelListeners.delete(listener)
  }
}

/** Single writer for `level`, so ref and subscribers can never disagree. */
function publishLevel(next: number): void {
  level.value = next
  for (const listener of levelListeners) listener(next)
}

const tracks = computed<MusicTrack[]>(() => library.value?.tracks ?? [])

const current = computed<MusicTrack | null>(
  () => tracks.value.find((track) => track.id === currentId.value) ?? null,
)

const currentIndex = computed(() => queue.value.indexOf(currentId.value))

const activeLyricIndex = computed(() =>
  lyrics.value.length > 0 ? findActiveLine(lyrics.value, position.value * 1000) : -1,
)

/* -------------------------------------------------------------------------- */
/* The element                                                                */
/* -------------------------------------------------------------------------- */

let audio: HTMLAudioElement | null = null
let context: AudioContext | null = null
let analyser: AnalyserNode | null = null
let spectrum: Parameters<AnalyserNode['getByteFrequencyData']>[0] = new Uint8Array(0)
let frame = 0
/** Consecutive failed loads; a broken library must not loop forever. */
let failures = 0

type AudioContextConstructor = new (options?: AudioContextOptions) => AudioContext

function audioContextCtor(): AudioContextConstructor | null {
  const legacy = window as unknown as { webkitAudioContext?: AudioContextConstructor }
  return window.AudioContext ?? legacy.webkitAudioContext ?? null
}

function trackSeconds(track: MusicTrack | null): number {
  return track && track.durationMs > 0 ? track.durationMs / 1000 : 0
}

function streamUrlFor(id: string): string {
  const track = tracks.value.find((item) => item.id === id)
  // Prefer the URL the server handed out: it keeps working if the route shape
  // ever changes. The fallback only exists for a track that just vanished.
  return mediaUrl(track?.streamUrl ?? `/api/v1/music/stream/${id}`)
}

/* -------------------------------------------------------------------------- */
/* Reactive plumbing driven by the element                                    */
/* -------------------------------------------------------------------------- */

function startFrame(): void {
  if (frame || !audio) return
  const tick = () => {
    const el = audio
    if (el && !el.paused) {
      position.value = el.currentTime
      publishLevel(readLevel())
    }
    frame = requestAnimationFrame(tick)
  }
  frame = requestAnimationFrame(tick)
}

function stopFrame(): void {
  if (frame) cancelAnimationFrame(frame)
  frame = 0
  publishLevel(0)
}

/** Average of the low bins — the part that reads as "bass" to the eye. */
function readLevel(): number {
  if (!analyser) return 0
  analyser.getByteFrequencyData(spectrum)
  const bins = Math.min(12, spectrum.length)
  if (bins === 0) return 0
  let total = 0
  for (let i = 1; i < bins; i += 1) total += spectrum[i] ?? 0
  return total / ((bins - 1) * 255)
}

/**
 * Live frequency bytes, or null when Web Audio is unavailable.
 *
 * Callers draw straight from this buffer and must therefore consume it inside
 * their own animation frame rather than keeping a reference.
 */
export function readSpectrum(): Uint8Array | null {
  if (!analyser) return null
  analyser.getByteFrequencyData(spectrum)
  return spectrum
}

export function hasAnalyser(): boolean {
  return analyser !== null
}

function startAnalyser(): void {
  if (analyser || !audio) return

  try {
    const Ctor = audioContextCtor()
    if (!Ctor) return

    const ctx = new Ctor()
    const source = ctx.createMediaElementSource(audio)
    const node = ctx.createAnalyser()
    node.fftSize = 256
    node.smoothingTimeConstant = 0.8
    node.minDecibels = -85
    source.connect(node)
    node.connect(ctx.destination)

    spectrum = new Uint8Array(node.frequencyBinCount)
    context = ctx
    analyser = node
    // Safe to call here: this only ever runs from a play request, which the
    // browser counts as the required user gesture.
    void ctx.resume()
  } catch {
    /* Decoration only — playback continues through the element. */
  }
}

export function activeAudioContext(): AudioContext | null {
  return context
}

function ensureAudio(): HTMLAudioElement {
  if (audio) return audio

  const el = new Audio()
  el.preload = 'metadata'
  el.volume = volume.value
  el.muted = muted.value

  el.addEventListener('play', () => {
    playing.value = true
    buffering.value = false
    startFrame()
    syncMediaSessionState()
  })
  el.addEventListener('pause', () => {
    playing.value = false
    stopFrame()
    syncMediaSessionState()
  })
  el.addEventListener('waiting', () => {
    buffering.value = true
  })
  el.addEventListener('playing', () => {
    buffering.value = false
    playError.value = ''
    failures = 0
  })
  el.addEventListener('loadedmetadata', () => {
    if (Number.isFinite(el.duration)) duration.value = el.duration
  })
  el.addEventListener('durationchange', () => {
    if (Number.isFinite(el.duration) && el.duration > 0) duration.value = el.duration
  })
  el.addEventListener('timeupdate', () => {
    position.value = el.currentTime
  })
  el.addEventListener('volumechange', () => {
    muted.value = el.muted
  })
  el.addEventListener('ended', handleEnded)
  el.addEventListener('error', handlePlaybackError)

  audio = el
  return el
}

/* -------------------------------------------------------------------------- */
/* Playback                                                                   */
/* -------------------------------------------------------------------------- */

async function start(): Promise<void> {
  if (!currentId.value) {
    const first = queue.value[0]
    if (first) select(first, false)
  }

  const el = ensureAudio()
  startAnalyser()

  try {
    await el.play()
  } catch (cause) {
    playing.value = false
    // An autoplay rejection is a policy answer, not a broken file: stay paused
    // and let the visitor press play, rather than skipping the whole library.
    if (!(cause instanceof DOMException && cause.name === 'NotAllowedError')) {
      playError.value = '无法播放这首曲目'
    }
  }
}

function pause(): void {
  audio?.pause()
}

function toggle(): void {
  if (playing.value) pause()
  else void start()
}

/**
 * Loads a track. `autoplay` is false when restoring a session or previewing a
 * row, so the browser is never asked to start audio on its own.
 */
function select(id: string, autoplay = true): void {
  const el = ensureAudio()
  const track = tracks.value.find((item) => item.id === id)
  if (!track) return

  currentId.value = id
  playError.value = ''
  failures = 0
  position.value = 0
  duration.value = trackSeconds(track)
  void loadLyrics(id)
  updateMediaSession()

  el.src = streamUrlFor(id)
  el.load()
  persist()

  if (autoplay) void start()
  else if (playing.value) void start()
}

function stop(): void {
  const el = audio
  if (!el) return
  el.pause()
  el.removeAttribute('src')
  el.load()
  playing.value = false
  position.value = 0
  duration.value = 0
  stopFrame()
}

function seek(seconds: number): void {
  const el = ensureAudio()
  const total = duration.value || trackSeconds(current.value)
  const next = Math.min(Math.max(seconds, 0), total > 0 ? total : seconds)
  if (Number.isFinite(next)) {
    el.currentTime = next
    position.value = next
  }
}

function seekRatio(ratio: number): void {
  const total = duration.value || trackSeconds(current.value)
  seek(ratio * total)
}

function next(manual = true): void {
  const list = queue.value
  if (list.length === 0) return

  if (mode.value === 'shuffle' && list.length > 1) {
    const others = list.filter((id) => id !== currentId.value)
    const target = others[Math.floor(Math.random() * others.length)]
    if (target) select(target)
    return
  }

  const index = currentIndex.value
  const last = index >= list.length - 1

  // Auto-advance in order mode stops at the end of the queue; a manual press
  // wraps, because a click that does nothing reads as a bug.
  if (last && !manual && mode.value === 'order') {
    pause()
    seek(0)
    return
  }

  const target = list[(index + 1) % list.length]
  if (target) select(target)
}

function prev(): void {
  const list = queue.value
  if (list.length === 0) return

  // Standard player behaviour: restart the track unless we are near its start.
  if ((audio?.currentTime ?? 0) > 3) {
    seek(0)
    return
  }

  if (mode.value === 'shuffle' && list.length > 1) {
    const others = list.filter((id) => id !== currentId.value)
    const target = others[Math.floor(Math.random() * others.length)]
    if (target) select(target)
    return
  }

  const index = currentIndex.value
  const target = list[(index - 1 + list.length) % list.length]
  if (target) select(target)
}

function handleEnded(): void {
  if (mode.value === 'single') {
    seek(0)
    void start()
    return
  }
  next(false)
}

function handlePlaybackError(): void {
  playing.value = false
  stopFrame()

  // A single unreadable file (unsupported codec, deleted mid-session) should
  // cost one track, not the queue; three in a row means the server is unhappy.
  if (failures >= 3) {
    playError.value = '连续多首播放失败，请检查音乐目录'
    return
  }

  failures += 1
  playError.value = `无法播放《${current.value?.title ?? '未知曲目'}》，已跳到下一首`
  next(false)
}

/* -------------------------------------------------------------------------- */
/* Lyrics                                                                     */
/* -------------------------------------------------------------------------- */

const lyricCache = new Map<string, { lines: LrcLine[]; plain: string[] }>()

function applyLyrics(id: string, parsed: { lines: LrcLine[]; plain: string[] }): void {
  if (currentId.value !== id) return
  lyrics.value = parsed.lines
  plainLyrics.value = parsed.plain
}

async function loadLyrics(id: string): Promise<void> {
  const track = tracks.value.find((item) => item.id === id)
  lyrics.value = []
  plainLyrics.value = []

  if (!track?.hasLyrics) return

  const cached = lyricCache.get(id)
  if (cached) {
    applyLyrics(id, cached)
    return
  }

  lyricsLoading.value = true
  try {
    const response = await getTrackLyrics(id)
    const parsed = parseLyrics(response.lyrics)
    lyricCache.set(id, parsed)
    applyLyrics(id, parsed)
  } catch {
    /* Lyrics are optional; a failed fetch leaves the panel on its placeholder. */
  } finally {
    if (currentId.value === id) lyricsLoading.value = false
  }
}

/* -------------------------------------------------------------------------- */
/* Volume, mode, panel                                                        */
/* -------------------------------------------------------------------------- */

function setVolume(next: number): void {
  const el = ensureAudio()
  const value = Math.min(Math.max(next, 0), 1)
  volume.value = value
  el.volume = value
  // Dragging the slider up is an implicit unmute; dragging to zero is not an
  // explicit mute, so the two stay separate notions.
  if (value > 0 && el.muted) el.muted = false
  persist()
}

function toggleMute(): void {
  const el = ensureAudio()
  el.muted = !el.muted
  muted.value = el.muted
  persist()
}

function setMode(next: PlayMode): void {
  mode.value = next
  persist()
}

function cycleMode(): PlayMode {
  const index = PLAY_MODES.indexOf(mode.value)
  const next = PLAY_MODES[(index + 1) % PLAY_MODES.length] ?? 'loop'
  setMode(next)
  return next
}

function setExpanded(next: boolean): void {
  expanded.value = next
  if (!next) showPlaylist.value = false
  persist()
}

function toggleExpanded(): void {
  setExpanded(!expanded.value)
}

function togglePlaylist(): void {
  showPlaylist.value = !showPlaylist.value
}

/* -------------------------------------------------------------------------- */
/* Queue                                                                      */
/* -------------------------------------------------------------------------- */

/** Replace the play order — used by the "play this album" action. */
function setQueue(ids: string[], startId?: string): void {
  queue.value = [...ids]
  const first = startId && ids.includes(startId) ? startId : ids[0]
  if (first) {
    select(first)
    setExpanded(true)
  }
}

function playTrack(track: MusicTrack, ids?: string[]): void {
  if (ids && ids.length > 0) queue.value = [...ids]
  else if (!queue.value.includes(track.id)) queue.value = [...queue.value, track.id]

  select(track.id)
  setExpanded(true)
}

function shuffleAll(): void {
  const ids = [...tracks.value.map((track) => track.id)]
  // Fisher–Yates on a copy: the library order stays intact for the album view.
  for (let i = ids.length - 1; i > 0; i -= 1) {
    const j = Math.floor(Math.random() * (i + 1))
    const swap = ids[i]
    ids[i] = ids[j] ?? ''
    ids[j] = swap ?? ''
  }
  setMode('shuffle')
  setQueue(ids.filter(Boolean))
}

/* -------------------------------------------------------------------------- */
/* Media session (lock screen / OS media keys)                                */
/* -------------------------------------------------------------------------- */

function syncMediaSessionState(): void {
  if (!('mediaSession' in navigator)) return
  try {
    navigator.mediaSession.playbackState = playing.value ? 'playing' : 'paused'
  } catch {
    /* some browsers refuse the setter before metadata exists */
  }
}

function updateMediaSession(): void {
  if (!('mediaSession' in navigator)) return
  const track = current.value
  if (!track) return

  try {
    navigator.mediaSession.metadata = new MediaMetadata({
      title: track.title,
      artist: track.artist,
      album: track.album,
      artwork:
        track.hasCover && track.coverUrl
          ? [{ src: mediaUrl(track.coverUrl), sizes: '512x512', type: 'image/jpeg' }]
          : [],
    })

    navigator.mediaSession.setActionHandler('play', () => void start())
    navigator.mediaSession.setActionHandler('pause', () => pause())
    navigator.mediaSession.setActionHandler('previoustrack', () => prev())
    navigator.mediaSession.setActionHandler('nexttrack', () => next())
    navigator.mediaSession.setActionHandler('seekto', (details) => {
      if (typeof details.seekTime === 'number') seek(details.seekTime)
    })
  } catch {
    /* MediaSession is a bonus; unsupported actions throw on older browsers. */
  }
}

/* -------------------------------------------------------------------------- */
/* Library                                                                    */
/* -------------------------------------------------------------------------- */

let inflight: Promise<void> | null = null

/**
 * Loads the library once per session. The player mounts on every public page,
 * so a second call is expected and must not refetch.
 */
async function load(force = false): Promise<void> {
  if (loaded.value && !force) return
  if (inflight) return inflight

  loading.value = true
  loadError.value = ''

  inflight = getMusicLibrary()
    .then((data) => {
      library.value = data
      loaded.value = true

      const ids = data.tracks.map((track) => track.id)
      const known = new Set(ids)

      // A rescan can drop tracks; a stale queue id would stall "next" on a
      // missing file, so the queue is rebuilt from what the server just sent.
      queue.value = queue.value.length > 0 ? queue.value.filter((id) => known.has(id)) : ids
      if (queue.value.length === 0) queue.value = ids

      if (!currentId.value) {
        currentId.value = stored.trackId && known.has(stored.trackId) ? stored.trackId : (ids[0] ?? '')
      } else if (!known.has(currentId.value)) {
        currentId.value = ids[0] ?? ''
      }

      // Prepare (but never auto-play) the restored track so the panel shows its
      // cover, title and duration before the visitor asks for sound.
      if (currentId.value) select(currentId.value, false)
      else duration.value = 0
    })
    .catch((cause: unknown) => {
      loadError.value = cause instanceof Error ? cause.message : '音乐库加载失败'
    })
    .finally(() => {
      loading.value = false
      inflight = null
    })

  return inflight
}

/* -------------------------------------------------------------------------- */
/* Persistence                                                                */
/* -------------------------------------------------------------------------- */

function persist(): void {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        volume: volume.value,
        muted: muted.value,
        mode: mode.value,
        expanded: expanded.value,
        trackId: currentId.value,
      } satisfies StoredSession),
    )
  } catch {
    /* private mode — preferences simply do not survive the session */
  }
}

watch([volume, muted, mode, expanded], persist)

/* -------------------------------------------------------------------------- */

export function useMusic() {
  return {
    // Library
    tracks,
    library: readonly(library),
    loading: readonly(loading),
    loaded: readonly(loaded),
    loadError: readonly(loadError),
    // Current track
    current,
    currentIndex,
    currentId: readonly(currentId),
    queue: readonly(queue),
    playError: readonly(playError),
    // Transport
    playing: readonly(playing),
    buffering: readonly(buffering),
    position: readonly(position),
    duration: readonly(duration),
    level: readonly(level),
    // Preferences
    volume: readonly(volume),
    muted: readonly(muted),
    mode: readonly(mode),
    expanded: readonly(expanded),
    showPlaylist: readonly(showPlaylist),
    // Lyrics
    lyrics: readonly(lyrics),
    plainLyrics: readonly(plainLyrics),
    lyricsLoading: readonly(lyricsLoading),
    activeLyricIndex,
    // Actions
    load,
    select,
    playTrack,
    setQueue,
    shuffleAll,
    start,
    pause,
    toggle,
    stop,
    next,
    prev,
    seek,
    seekRatio,
    setVolume,
    toggleMute,
    setMode,
    cycleMode,
    setExpanded,
    toggleExpanded,
    togglePlaylist,
  }
}
