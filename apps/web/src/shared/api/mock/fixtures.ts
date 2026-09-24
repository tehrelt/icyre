/**
 * Development fixtures served by the mock API until the BFF endpoints exist.
 * Content mirrors the ICYRE product canvas (Home, Album "Prism Hours").
 * Shapes follow the public API contracts; UI code never imports this file.
 */

type Json = Record<string, unknown>;

const album = (id: string, title: string, artistName: string, year: number, art: number, extra: Json = {}) => ({
  kind: 'album' as const,
  id,
  title,
  artistName,
  year,
  art,
  coverUrl: null,
  explicit: false,
  ...extra,
});

const playlist = (id: string, title: string, owner: string, trackCount: number, art: number) => ({
  kind: 'playlist' as const,
  id,
  title,
  owner,
  trackCount,
  art,
  coverUrl: null,
});

const artist = (id: string, name: string, listeners: string, art: number) => ({
  kind: 'artist' as const,
  id,
  name,
  listeners,
  art,
  coverUrl: null,
});

const track = (id: string, title: string, artistName: string, albumId: string, albumTitle: string, durationSec: number, art: number, extra: Json = {}) => ({
  id,
  title,
  artistName,
  albumId,
  albumTitle,
  durationSec,
  art,
  coverUrl: null,
  explicit: false,
  liked: false,
  available: true,
  ...extra,
});

export const currentUser = { id: 'usr-rin', displayName: 'Rin Aoki', avatarUrl: null, art: 7 };

export const librarySummary = {
  savedCount: 128,
  likedTracksCount: 342,
  playlists: [
    playlist('pl-late-night', 'Late night drives', 'Rin Aoki', 48, 7),
    playlist('pl-deep-focus', 'Deep focus', 'Rin Aoki', 112, 5),
    playlist('pl-run-club', 'Run club', 'Rin Aoki', 36, 4),
    playlist('pl-glass-grain', 'Glass & grain', 'ICYRE', 45, 2),
    playlist('pl-archive-2025', 'Archive 2025', 'Rin Aoki', 204, 3),
  ],
};

/** Album "Prism Hours" by Nova Hale — the album from the canvas. */
export const prismHoursTracks = [
  track('trk-frozen-choir', 'Frozen Choir', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 240, 0, { plays: 1904210 }),
  track('trk-glass-tides', 'Glass Tides', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 227, 0, { liked: true, plays: 4431118 }),
  track('trk-mirror-weather', 'Mirror Weather', 'Nova Hale · Kai Frost', 'alb-prism-hours', 'Prism Hours', 198, 0, { explicit: true, plays: 2076554 }),
  track('trk-pearl-static', 'Pearl Static', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 254, 0, { plays: 988402 }),
  track('trk-slow-aurora', 'Slow Aurora', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 311, 0, { liked: true, plays: 1310776 }),
  track('trk-thin-film', 'Thin Film', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 182, 0, { plays: 742090 }),
  track('trk-harbour-lights', 'Harbour Lights', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 266, 0, { plays: 655318 }),
  track('trk-vapour-trail', 'Vapour Trail', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 174, 0, { plays: 512907 }),
  track('trk-northern-index', 'Northern Index', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 219, 0, { available: false, plays: 0 }),
  track('trk-rime', 'Rime', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 108, 0, { plays: 403261 }),
  track('trk-prism-hours', 'Prism Hours', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 387, 0, { plays: 1127843 }),
];

const trendingToday = [
  track('trk-lanterns-out', 'Lanterns Out', 'Mira Solen', 'alb-salt-lanterns', 'Salt Lanterns', 214, 2, { badge: 'trending', liked: true }),
  track('trk-soft-engines', 'Soft Engines', 'Kestrel Nine', 'alb-soft-engines', 'Soft Engines', 187, 4, { explicit: true }),
  track('trk-glass-tides', 'Glass Tides', 'Nova Hale', 'alb-prism-hours', 'Prism Hours', 227, 0, { liked: true }),
  track('trk-paper-moons', 'Paper Moons', 'Juno Reyes', 'alb-paper-moons', 'Paper Moons', 243, 1, { badge: 'new' }),
  track('trk-undertow-choir', 'Undertow Choir', 'Halcyon Ward', 'alb-lowlight-atlas', 'Lowlight Atlas', 276, 6),
  track('trk-frost-letters', 'Frost Letters', 'Sable Ono · Aster Vale', 'alb-cold-harbour', 'Cold Harbour', 201, 5),
];

const trendingWeek = [
  trendingToday[2],
  trendingToday[0],
  track('trk-quiet-machines', 'Quiet Machines', 'Lumen Park', 'alb-quiet-machines', 'Quiet Machines', 232, 3),
  trendingToday[1],
  track('trk-pearl-static-tm', 'Pearl Static', 'Tove Marin', 'alb-pearl-static', 'Pearl Static', 205, 7),
  trendingToday[4],
];

export const homeFeed = {
  recentlyPlayed: [
    album('alb-prism-hours', 'Prism Hours', 'Nova Hale', 2026, 0),
    playlist('pl-late-night', 'Late night drives', 'Rin Aoki', 48, 7),
    album('alb-hollow-signal', 'Hollow Signal', 'Kai Frost', 2025, 4, { explicit: true }),
    playlist('pl-deep-focus', 'Deep focus', 'Rin Aoki', 112, 5),
    artist('art-aster-vale', 'Aster Vale', '96K', 1),
    album('alb-night-index', 'Night Index', 'Lumen Park', 2025, 6),
  ],
  albumOfTheWeek: {
    album: album('alb-salt-lanterns', 'Salt Lanterns', 'Mira Solen', 2026, 2, { badge: 'featured' }),
    serial: 'No. 027',
    description:
      'Mira Solen records a coastline in February: bowed guitar, field recordings from a frozen harbour and a voice that barely rises above the wind.',
    label: 'Coldframe Records',
    releaseDate: '2026-02-20',
    format: '24-bit · 96 kHz',
    catalogNumber: 'CF-027',
    trackCount: 9,
    durationSec: 38 * 60,
    tags: ['Ambient folk', 'Field recordings'],
    hiRes: true,
  },
  recommended: {
    kicker: 'Because you played Nova Hale',
    items: [
      album('alb-hollow-signal', 'Hollow Signal', 'Kai Frost', 2025, 4, { explicit: true }),
      playlist('pl-glass-grain', 'Glass & grain', 'ICYRE', 45, 2),
      album('alb-pearl-static', 'Pearl Static', 'Tove Marin', 2026, 7),
      artist('art-aster-vale', 'Aster Vale', '96K', 1),
      album('alb-quiet-machines', 'Quiet Machines', 'Lumen Park', 2024, 3),
      album('alb-cold-harbour', 'Cold Harbour', 'Sable Ono', 2025, 5),
    ],
  },
  newReleases: {
    kicker: 'Out this week',
    items: [
      album('alb-lowlight-atlas', 'Lowlight Atlas', 'Halcyon Ward', 2026, 6, { badge: 'new' }),
      album('alb-paper-moons', 'Paper Moons', 'Juno Reyes', 2026, 1, { badge: 'new' }),
      album('alb-soft-engines', 'Soft Engines', 'Kestrel Nine', 2026, 4, { badge: 'new', explicit: true }),
      album('alb-white-noise-garden', 'White Noise Garden', 'Palefield', 2026, 5),
      album('alb-sleet', 'Sleet', 'Ines Mora', 2026, 0),
      album('alb-afterglass', 'Afterglass', 'Tove Marin', 2026, 7),
    ],
  },
  trending: { today: trendingToday, week: trendingWeek },
  madeForYou: {
    featured: {
      ...playlist('pl-daily-mix-07', 'Daily mix 07', 'Made for Rin', 50, 7),
      serial: 'No. 07',
      artistsLine: 'Nova Hale, Kai Frost, Ines Mora and more',
      durationSec: 192 * 60,
    },
    playlists: [
      playlist('pl-weekly-thaw', 'Weekly thaw', 'Made for Rin', 30, 0),
      playlist('pl-new-frost', 'New frost', 'Made for Rin', 25, 4),
      playlist('pl-on-repeat', 'On repeat', 'Made for Rin', 50, 3),
      playlist('pl-daily-mix-03', 'Daily mix 03', 'Made for Rin', 50, 5),
    ],
  },
  followedArtists: [
    artist('art-nova-hale', 'Nova Hale', '1.2M', 3),
    artist('art-kai-frost', 'Kai Frost', '640K', 4),
    artist('art-mira-solen', 'Mira Solen', '212K', 2),
    artist('art-ines-mora', 'Ines Mora', '88K', 0),
    artist('art-lumen-park', 'Lumen Park', '304K', 6),
    artist('art-tove-marin', 'Tove Marin', '157K', 7),
  ],
};

const TITLE_WORDS = ['Glass', 'Frost', 'Harbour', 'Pearl', 'Signal', 'Aurora', 'Lantern', 'Static', 'Rime', 'Vapour', 'Tide', 'Index'];

/** Deterministic track list for any collection without hand-written fixtures. */
export function generatedTracks(collectionId: string, title: string, artistName: string, art: number, count = 8) {
  const seed = [...collectionId].reduce((a, c) => a + c.charCodeAt(0), 0);
  return Array.from({ length: count }, (_, i) => {
    const w1 = TITLE_WORDS[(seed + i) % TITLE_WORDS.length];
    const w2 = TITLE_WORDS[(seed + i * 7 + 3) % TITLE_WORDS.length];
    return track(`${collectionId}-t${i + 1}`, `${w1} ${w2}`, artistName, collectionId, title, 150 + ((seed * (i + 3)) % 180), art);
  });
}

/** Album page (canvas Album.dc.html): "Prism Hours" by Nova Hale. */
export const prismHoursPage = {
  album: {
    id: 'alb-prism-hours',
    title: 'Prism Hours',
    albumType: 'ALBUM',
    serial: 'No. 014',
    year: 2026,
    releaseDate: '2026-03-06',
    trackCount: 11,
    durationSec: 42 * 60,
    tags: ['Ambient pop', 'Downtempo'],
    hiRes: true,
    copyright: '© ℗ 2026 Coldframe Records',
    coverUrl: null,
    art: 0,
  },
  artist: { id: 'art-nova-hale', name: 'Nova Hale', avatarUrl: null, art: 3 },
  tracks: prismHoursTracks,
  moreByArtist: [
    album('alb-winter-index', 'Winter Index', 'Nova Hale', 2024, 3, { albumType: 'ALBUM' }),
    album('alb-close-weather', 'Close Weather', 'Nova Hale', 2023, 5, { albumType: 'EP' }),
    album('alb-glass-tides-remix', 'Glass Tides (Aster Vale remix)', 'Nova Hale', 2026, 2, { albumType: 'SINGLE', badge: 'new' }),
    album('alb-salt-and-static', 'Salt & Static', 'Nova Hale', 2021, 6, { albumType: 'ALBUM' }),
    album('alb-first-frost', 'First Frost', 'Nova Hale', 2019, 1, { albumType: 'EP' }),
    album('alb-rime-live', 'Rime (Live at Hallgrím)', 'Nova Hale', 2025, 7, { albumType: 'SINGLE' }),
  ],
};

/** Album page for any other fixture album, generated deterministically. */
export function generatedAlbumPage(id: string, title: string, artistName: string, art: number, year = 2025) {
  const tracks = generatedTracks(id, title, artistName, art, 9).map((t, i) => ({ ...t, plays: 900_000 - i * 73_417 }));
  const durationSec = tracks.reduce((a, t) => a + t.durationSec, 0);
  return {
    album: {
      id,
      title,
      albumType: 'ALBUM',
      year,
      releaseDate: `${year}-01-17`,
      trackCount: tracks.length,
      durationSec,
      tags: [],
      hiRes: false,
      copyright: `© ℗ ${year} ${artistName}`,
      coverUrl: null,
      art,
    },
    artist: { id: `art-${id.replace(/^alb-/, '')}`, name: artistName, avatarUrl: null, art: (art + 3) % 8 },
    tracks,
    moreByArtist: [],
  };
}
