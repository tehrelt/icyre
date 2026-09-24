/**
 * Audio engines hide the media element from the rest of the app.
 * Only the player controller talks to an engine; UI components never do.
 */
export type EngineEvent =
  | { type: 'time'; position: number }
  | { type: 'duration'; duration: number }
  | { type: 'buffered'; buffered: number }
  | { type: 'waiting' }
  | { type: 'playing' }
  | { type: 'ended' }
  | { type: 'error'; message: string };

export type EngineListener = (e: EngineEvent) => void;

export interface AudioEngine {
  /** Loads a source. `src` null means no stream is available. */
  load(src: string | null, durationHint: number): void;
  play(): Promise<void>;
  pause(): void;
  seek(seconds: number): void;
  /** volume 0–1 */
  setVolume(volume: number, muted: boolean): void;
  subscribe(listener: EngineListener): () => void;
  destroy(): void;
}

/** Shared listener bookkeeping. */
export abstract class BaseEngine implements AudioEngine {
  private listeners = new Set<EngineListener>();

  subscribe(listener: EngineListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  protected emit(e: EngineEvent): void {
    this.listeners.forEach((l) => l(e));
  }

  destroy(): void {
    this.listeners.clear();
  }

  abstract load(src: string | null, durationHint: number): void;
  abstract play(): Promise<void>;
  abstract pause(): void;
  abstract seek(seconds: number): void;
  abstract setVolume(volume: number, muted: boolean): void;
}
