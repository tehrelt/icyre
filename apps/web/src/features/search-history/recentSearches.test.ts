import { useRecentSearches } from './recentSearches';

beforeEach(() => useRecentSearches.setState({ items: [] }));

describe('recent searches', () => {
  it('normalises, de-duplicates and keeps the newest first', () => {
    const { add } = useRecentSearches.getState();
    add('Nova  Hale ');
    add('salt lanterns');
    add('nova hale');
    add('x');
    expect(useRecentSearches.getState().items).toEqual(['nova hale', 'salt lanterns']);
  });

  it('caps the list and supports remove/clear', () => {
    const s = useRecentSearches.getState();
    for (let i = 0; i < 12; i++) s.add(`query ${i}`);
    expect(useRecentSearches.getState().items).toHaveLength(8);
    useRecentSearches.getState().remove('query 11');
    expect(useRecentSearches.getState().items[0]).toBe('query 10');
    useRecentSearches.getState().clear();
    expect(useRecentSearches.getState().items).toEqual([]);
  });
});
