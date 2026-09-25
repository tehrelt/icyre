import { BaseEngine } from './AudioEngine';

/**
 * Plays signed CDN URLs through one long-lived HTMLAudioElement. Audio bytes
 * go browser → CDN → object storage; they never pass through the backend.
 */
export class HtmlAudioEngine extends BaseEngine {
  private readonly audio: HTMLAudioElement;
  private readonly off: Array<() => void> = [];

  constructor(audio: HTMLAudioElement = new Audio()) {
    super();
    this.audio = audio;
    this.audio.preload = 'auto';

    const on = <K extends keyof HTMLMediaElementEventMap>(type: K, fn: () => void) => {
      this.audio.addEventListener(type, fn);
      this.off.push(() => this.audio.removeEventListener(type, fn));
    };
    on('timeupdate', () => this.emit({ type: 'time', position: this.audio.currentTime }));
    on('durationchange', () => this.emit({ type: 'duration', duration: this.audio.duration }));
    on('progress', () => {
      const b = this.audio.buffered;
      if (b.length > 0) this.emit({ type: 'buffered', buffered: b.end(b.length - 1) });
    });
    on('waiting', () => this.emit({ type: 'waiting' }));
    on('playing', () => this.emit({ type: 'playing' }));
    on('ended', () => this.emit({ type: 'ended' }));
    on('error', () => this.emit({ type: 'error', message: 'This track could not be played' }));
  }

  load(src: string | null): void {
    if (!src) {
      this.audio.removeAttribute('src');
      this.audio.load();
      this.emit({ type: 'error', message: 'Stream is not available' });
      return;
    }
    this.audio.src = src;
    this.audio.load();
  }

  async play(): Promise<void> {
    if (!this.audio.src) return;
    try {
      await this.audio.play();
    } catch (err) {
      const name = err instanceof DOMException ? err.name : '';
      // AbortError: a newer load() interrupted this play() — not a failure.
      if (name === 'AbortError') return;
      // NotAllowedError is the autoplay policy; anything else (e.g.
      // NotSupportedError: the browser cannot decode the format) is a
      // media failure, reported by the element's own error event too.
      this.emit({ type: 'error', message: name === 'NotAllowedError' ? 'Playback was blocked by the browser' : 'This track could not be played' });
    }
  }

  pause(): void {
    this.audio.pause();
  }

  seek(seconds: number): void {
    if (Number.isFinite(seconds)) this.audio.currentTime = seconds;
  }

  setVolume(volume: number, muted: boolean): void {
    this.audio.volume = Math.max(0, Math.min(1, volume));
    this.audio.muted = muted;
  }

  override destroy(): void {
    this.off.forEach((f) => f());
    this.audio.pause();
    this.audio.removeAttribute('src');
    super.destroy();
  }
}
