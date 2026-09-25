/**
 * Seeds Catalog Service with the content of the ICYRE product canvas
 * (Nova Hale — Prism Hours, the Home shelves) through its REST API.
 *
 *   bun scripts/seed-catalog.ts [http://localhost:8081]
 *
 * Talks to Catalog directly: the gateway exposes only reads to clients.
 * Idempotent enough for a dev stand: skips seeding when albums exist.
 */
const base = (process.argv[2] ?? process.env.CATALOG_URL ?? 'http://localhost:8081').replace(/\/$/, '');

interface Created {
  id: string;
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${base}/api/v1${path}`, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path} → ${res.status}: ${text}`);
  return JSON.parse(text) as T;
}

type TrackSeed = [title: string, seconds: number, opts?: { explicit?: boolean; feat?: string; status?: 'READY' | 'BLOCKED' | 'DRAFT' }];

interface AlbumSeed {
  title: string;
  artist: string;
  type: 'ALBUM' | 'EP' | 'SINGLE';
  date: string;
  genres?: string[];
  tracks: TrackSeed[];
}

const artists = ['Nova Hale', 'Kai Frost', 'Mira Solen', 'Aster Vale', 'Halcyon Ward', 'Juno Reyes', 'Kestrel Nine', 'Tove Marin', 'Lumen Park', 'Sable Ono', 'Ines Mora', 'Palefield'];

const albums: AlbumSeed[] = [
  {
    title: 'Prism Hours', artist: 'Nova Hale', type: 'ALBUM', date: '2026-03-06', genres: ['ambient-pop', 'downtempo'],
    tracks: [
      ['Frozen Choir', 240], ['Glass Tides', 227], ['Mirror Weather', 198, { explicit: true, feat: 'Kai Frost' }], ['Pearl Static', 254],
      ['Slow Aurora', 311], ['Thin Film', 182], ['Harbour Lights', 266], ['Vapour Trail', 174],
      ['Northern Index', 219, { status: 'BLOCKED' }], ['Rime', 108], ['Prism Hours', 387],
    ],
  },
  { title: 'Winter Index', artist: 'Nova Hale', type: 'ALBUM', date: '2024-11-15', genres: ['ambient'], tracks: [['Winter Index', 245], ['Snowline', 201], ['Cold Open', 188]] },
  { title: 'Close Weather', artist: 'Nova Hale', type: 'EP', date: '2023-05-19', tracks: [['Close Weather', 232], ['Low Front', 205]] },
  { title: 'Salt Lanterns', artist: 'Mira Solen', type: 'ALBUM', date: '2026-02-20', genres: ['ambient-folk', 'field-recordings'], tracks: [['Lanterns Out', 214], ['Frozen Harbour', 263], ['Bowed Light', 241]] },
  { title: 'Hollow Signal', artist: 'Kai Frost', type: 'ALBUM', date: '2025-09-12', genres: ['electronic'], tracks: [['Hollow Signal', 222, { explicit: true }], ['Supernova Drift', 205]] },
  { title: 'Lowlight Atlas', artist: 'Halcyon Ward', type: 'ALBUM', date: '2026-09-19', tracks: [['Undertow Choir', 276], ['Atlas', 230]] },
  { title: 'Paper Moons', artist: 'Juno Reyes', type: 'ALBUM', date: '2026-09-18', tracks: [['Paper Moons', 243], ['Nova Lines', 219]] },
  { title: 'Soft Engines', artist: 'Kestrel Nine', type: 'ALBUM', date: '2026-09-17', genres: ['electronic'], tracks: [['Soft Engines', 187, { explicit: true }]] },
  { title: 'Cold Harbour', artist: 'Sable Ono', type: 'ALBUM', date: '2025-12-05', tracks: [['Frost Letters', 201, { feat: 'Aster Vale' }]] },
  { title: 'Quiet Machines', artist: 'Lumen Park', type: 'ALBUM', date: '2024-06-21', tracks: [['Quiet Machines', 232]] },
  { title: 'Afterglass', artist: 'Tove Marin', type: 'SINGLE', date: '2026-08-28', tracks: [['Afterglass', 214]] },
  { title: 'Sleet', artist: 'Ines Mora', type: 'EP', date: '2026-07-10', tracks: [['Sleet', 199], ['Thaw', 223]] },
];

const existing = await call<{ data: unknown[] }>('GET', '/albums?limit=1');
if (existing.data.length > 0) {
  console.log('catalog already has albums — skipping seed');
  process.exit(0);
}

const genres = await call<{ data: Array<{ id: string; slug: string }> }>('GET', '/genres');
const genreId = new Map(genres.data.map((g) => [g.slug, g.id]));

const artistId = new Map<string, string>();
for (const name of artists) {
  artistId.set(name, (await call<Created>('POST', '/artists', { name })).id);
}

let tracks = 0;
for (const a of albums) {
  const album = await call<Created>('POST', '/albums', {
    title: a.title,
    albumType: a.type,
    releaseDate: a.date,
    artistIds: [artistId.get(a.artist)],
    genreIds: (a.genres ?? []).map((g) => genreId.get(g)).filter(Boolean),
  });
  for (const [i, [title, seconds, opts]] of a.tracks.entries()) {
    const credits = [artistId.get(a.artist), ...(opts?.feat ? [artistId.get(opts.feat)] : [])];
    const track = await call<Created>('POST', '/tracks', {
      albumId: album.id,
      artistIds: credits,
      title,
      durationMs: seconds * 1000,
      trackNumber: i + 1,
      explicit: opts?.explicit ?? false,
    });
    // Walk the status machine the media pipeline will drive: DRAFT → PROCESSING → READY (→ BLOCKED).
    const target = opts?.status ?? 'READY';
    if (target !== 'DRAFT') {
      await call('PATCH', `/tracks/${track.id}`, { status: 'PROCESSING' });
      await call('PATCH', `/tracks/${track.id}`, { status: 'READY' });
      if (target === 'BLOCKED') await call('PATCH', `/tracks/${track.id}`, { status: 'BLOCKED' });
    }
    tracks++;
  }
}
console.log(`seeded ${artists.length} artists, ${albums.length} albums, ${tracks} tracks`);
