/**
 * Fills object storage with audio variants for every playable Catalog track,
 * standing in for the media pipeline (EPIC-019/020) on a dev stand.
 *
 *   bun scripts/seed-media.ts [--qualities 64,128,256] [--catalog http://localhost:8081]
 *
 * Each track gets a generated tone of its real duration (ffmpeg, AAC/ADTS),
 * uploaded to tracks/{trackId}/audio/{quality}.aac — the layout from
 * specs/data/object-storage.md. Existing objects are skipped (variants are
 * immutable), so the script is safe to re-run.
 *
 * Needs ffmpeg on PATH and MinIO from docker compose (S3_* env to override).
 */
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';

import { S3Client } from 'bun';

const arg = (name: string) => {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : undefined;
};

const catalog = (arg('catalog') ?? process.env.CATALOG_URL ?? 'http://localhost:8081').replace(/\/$/, '');
const qualities = (arg('qualities') ?? '64,128,256').split(',').map(Number);
if (qualities.some((q) => ![64, 128, 256].includes(q))) throw new Error('qualities must be 64, 128 and/or 256');

const s3 = new S3Client({
  endpoint: process.env.S3_ENDPOINT ?? 'http://localhost:9000',
  accessKeyId: process.env.S3_ACCESS_KEY ?? 'icyre',
  secretAccessKey: process.env.S3_SECRET_KEY ?? 'icyre-secret',
  bucket: process.env.S3_MEDIA_BUCKET ?? 'icyre-media',
});

interface Page<T> {
  data: T[];
  pagination?: { nextCursor?: string | null; hasMore: boolean };
}
interface Album {
  id: string;
  title: string;
}
interface Track {
  id: string;
  title: string;
  durationMs: number;
  status: string;
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${catalog}/api/v1${path}`);
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
  return (await res.json()) as T;
}

async function allAlbums(): Promise<Album[]> {
  const out: Album[] = [];
  let cursor: string | null | undefined;
  do {
    const page = await get<Page<Album>>(`/albums?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`);
    out.push(...page.data);
    cursor = page.pagination?.hasMore ? page.pagination.nextCursor : null;
  } while (cursor);
  return out;
}

/** Pitch of the generated drone, derived from the track ID (A2..G#3). */
function rootHz(trackId: string): number {
  let h = 0;
  for (const c of trackId) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return 110 * 2 ** ((h % 12) / 12);
}

/**
 * One ffmpeg run per track: the tone (root, fifth, octave with a slow swell)
 * is synthesised once and encoded to every requested bitrate.
 */
async function encodeAll(trackId: string, seconds: number, kbps: number[], dir: string): Promise<Map<number, string>> {
  const f = rootHz(trackId);
  const fade = Math.min(3, seconds / 4);
  const voices = [f, f * 1.5, f * 2];
  const inputs = voices.flatMap((hz) => ['-f', 'lavfi', '-i', `sine=f=${hz.toFixed(2)}:d=${seconds}:r=44100`]);
  const mix =
    `[0]volume=0.5[a];[1]volume=0.28[b];[2]volume=0.14[c];[a][b][c]amix=inputs=3:normalize=0,` +
    `tremolo=f=0.1:d=0.4,afade=t=in:d=${fade},afade=t=out:st=${seconds - fade}:d=${fade},` +
    `asplit=${kbps.length}${kbps.map((q) => `[o${q}]`).join('')}`;
  const outputs = new Map(kbps.map((q) => [q, `${dir}/${trackId}-${q}.aac`]));
  const args = [...outputs].flatMap(([q, file]) => ['-map', `[o${q}]`, '-ac', '2', '-c:a', 'aac', '-b:a', `${q}k`, '-f', 'adts', '-y', file]);
  const proc = Bun.spawn(['ffmpeg', '-hide_banner', '-loglevel', 'error', ...inputs, '-filter_complex', mix, ...args], { stderr: 'pipe' });
  const [err, code] = await Promise.all([new Response(proc.stderr).text(), proc.exited]);
  if (code !== 0) throw new Error(`ffmpeg failed for ${trackId}: ${err}`);
  return outputs;
}

const tracks: Track[] = [];
for (const album of await allAlbums()) {
  const page = await get<Page<Track>>(`/albums/${album.id}/tracks`);
  // Blocked tracks keep their media: blocking is a playback decision, not a deletion.
  tracks.push(...page.data.filter((t) => t.status === 'READY' || t.status === 'BLOCKED'));
}
console.log(`${tracks.length} tracks × ${qualities.join('/')} kbps`);

const dir = await mkdtemp(`${tmpdir()}/icyre-media-`);
let uploaded = 0;
let skipped = 0;
const queue = [...tracks];
const worker = async () => {
  for (let t = queue.shift(); t; t = queue.shift()) {
    const missing: number[] = [];
    for (const q of qualities) {
      if (await s3.exists(`tracks/${t.id}/audio/${q}.aac`)) skipped++;
      else missing.push(q);
    }
    if (missing.length === 0) continue;
    const files = await encodeAll(t.id, Math.max(1, Math.round(t.durationMs / 1000)), missing, dir);
    for (const [q, file] of files) {
      const body = Bun.file(file);
      await s3.write(`tracks/${t.id}/audio/${q}.aac`, body, { type: 'audio/aac' });
      uploaded++;
      console.log(`  tracks/${t.id}/audio/${q}.aac  ${(body.size / 1024 / 1024).toFixed(1)} MB  (${t.title})`);
      await body.delete();
    }
  }
};
await Promise.all(Array.from({ length: 4 }, worker));
await rm(dir, { recursive: true, force: true });
console.log(`done: ${uploaded} uploaded, ${skipped} already present`);
