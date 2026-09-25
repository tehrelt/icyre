import { search } from './searchIndex';

describe('mock search', () => {
  it('ranks the canvas "nova" results', () => {
    const r = search('nova', 'all', 20);
    expect(r.topResult).toMatchObject({ item: { kind: 'artist', name: 'Nova Hale' }, verified: true, monthlyListeners: '1.2M' });
    expect(r.artists.map((a) => a.name)).toContain('Novaline');
    expect(r.tracks).toHaveLength(4);
    expect(r.albums.slice(0, 2).map((a) => a.title)).toEqual(['Prism Hours', 'Winter Index']);
    expect(r.didYouMean).toBeNull();
  });

  it('suggests a spelling fix when nothing matches', () => {
    const r = search('nvoa hlae', 'all', 20);
    expect(r.counts).toEqual({ tracks: 0, artists: 0, albums: 0, playlists: 0 });
    expect(r.didYouMean).toBe('nova hale');
  });

  it('returns a single list for a type tab', () => {
    const r = search('nova', 'playlists', 50);
    expect(r.topResult).toBeNull();
    expect(r.tracks).toHaveLength(0);
    expect(r.playlists.length).toBe(r.counts.playlists);
  });
});
