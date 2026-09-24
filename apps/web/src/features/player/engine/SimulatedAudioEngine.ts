import { BaseEngine } from './AudioEngine';

/**
 * A silent clock that behaves like an audio element. Used in mock mode,
 * where there is no media origin to stream from, so the whole player UX
 * (progress, next, repeat, queue end) works end to end.
 */
export class SimulatedAudioEngine extends BaseEngine {
  private position = 0;
  private duration = 0;
  private timer: ReturnType<typeof setInterval> | null = null;

  constructor(private readonly tickMs = 250) {
    super();
  }

  load(_src: string | null, durationHint: number): void {
    this.stopClock();
    this.position = 0;
    this.duration = durationHint;
    this.emit({ type: 'duration', duration: durationHint });
    this.emit({ type: 'time', position: 0 });
    this.emit({ type: 'buffered', buffered: durationHint });
  }

  async play(): Promise<void> {
    if (this.timer || this.duration <= 0) return;
    this.emit({ type: 'playing' });
    this.timer = setInterval(() => {
      this.position = Math.min(this.duration, this.position + this.tickMs / 1000);
      this.emit({ type: 'time', position: this.position });
      if (this.position >= this.duration) {
        this.stopClock();
        this.emit({ type: 'ended' });
      }
    }, this.tickMs);
  }

  pause(): void {
    this.stopClock();
  }

  seek(seconds: number): void {
    this.position = Math.max(0, Math.min(this.duration, seconds));
    this.emit({ type: 'time', position: this.position });
  }

  setVolume(): void {
    // Silent by design.
  }

  override destroy(): void {
    this.stopClock();
    super.destroy();
  }

  private stopClock(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }
}
