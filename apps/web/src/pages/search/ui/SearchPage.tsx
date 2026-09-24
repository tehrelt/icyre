import { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useRecentSearches } from '@/features/search-history/recentSearches';
import { useDebouncedValue } from '@/shared/hooks/useDebouncedValue';
import { Button, SearchInput, Tabs, type TabItem } from '@/shared/ui';
import { EmptyState } from '@/shared/ui/patterns/EmptyState';
import { InlineAlert } from '@/shared/ui/patterns/InlineAlert';
import { PageToolbar } from '@/widgets/page-toolbar/PageToolbar';

import { SEARCH_TYPES, useSearch, type SearchResult, type SearchType } from '../api/search';

import { RecentSearches } from './RecentSearches';
import { SearchBrowse } from './SearchBrowse';
import styles from './SearchPage.module.css';
import { SearchResults, SearchSkeleton } from './SearchResults';

const DEBOUNCE_MS = 250;
/** A query that stayed on screen this long counts as a search the listener made. */
const RECORD_AFTER_MS = 1500;

const TAB_LABELS: Record<SearchType, string> = { all: 'All', tracks: 'Tracks', artists: 'Artists', albums: 'Albums', playlists: 'Playlists' };

function parseType(v: string | null): SearchType {
  return (SEARCH_TYPES as readonly string[]).includes(v ?? '') ? (v as SearchType) : 'all';
}

/** Search — canvas SearchEmpty / SearchResults / SearchLoading / SearchNoResults / SearchError. */
export function SearchPage() {
  const [params, setParams] = useSearchParams();
  const urlQuery = params.get('q') ?? '';
  const type = parseType(params.get('type'));
  const [input, setInput] = useState(urlQuery);
  const inputRef = useRef<HTMLInputElement>(null);
  const addRecent = useRecentSearches((s) => s.add);

  // Keep the field in sync when the URL changes from outside (back/forward,
  // recent search) — but never overwrite what is being typed right now.
  const pushed = useRef(urlQuery);
  useEffect(() => {
    if (urlQuery !== pushed.current) {
      pushed.current = urlQuery;
      setInput(urlQuery);
    }
  }, [urlQuery]);

  const setQuery = useCallback(
    (q: string, nextType: SearchType = type) => {
      pushed.current = q.trim();
      const next = new URLSearchParams();
      if (q.trim()) next.set('q', q.trim());
      if (q.trim() && nextType !== 'all') next.set('type', nextType);
      setParams(next, { replace: true });
    },
    [setParams, type],
  );

  // Debounced typing → URL (the URL is the source of truth for the query).
  const debounced = useDebouncedValue(input, DEBOUNCE_MS);
  useEffect(() => {
    if (debounced.trim() !== urlQuery) setQuery(debounced);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only react to typing
  }, [debounced]);

  // "/" focuses the field (the shortcut shown inside it).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const typing = target?.closest('input, textarea, select, [contenteditable="true"]');
      if (e.key === '/' && !typing && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const pick = useCallback(
    (q: string) => {
      setInput(q);
      setQuery(q, 'all');
      addRecent(q);
    },
    [addRecent, setQuery],
  );

  const search = useSearch(urlQuery, type);
  const data = search.data;

  useEffect(() => {
    if (!data || data.query !== urlQuery || total(data) === 0) return;
    const t = setTimeout(() => addRecent(urlQuery), RECORD_AFTER_MS);
    return () => clearTimeout(t);
  }, [data, urlQuery, addRecent]);

  const hasQuery = urlQuery.length > 0;
  const tabs: TabItem<SearchType>[] = SEARCH_TYPES.map((id) => ({
    id,
    label: TAB_LABELS[id],
    count: id !== 'all' && data && data.query === urlQuery ? data.counts[id] : undefined,
  }));

  return (
    <>
      <PageToolbar />
      <div className={hasQuery ? styles.pageResults : styles.pageBrowse}>
        <section className={styles.head}>
          {!hasQuery && <h1 className="t-h1">Search</h1>}
          <div className={styles.field}>
            <SearchInput
              ref={inputRef}
              size="lg"
              label="Search ICYRE"
              placeholder="Artists, tracks, albums or playlists"
              shortcut="/"
              value={input}
              onChange={setInput}
              onSubmit={(q) => {
                setQuery(q);
                addRecent(q);
              }}
            />
          </div>
          {hasQuery && <Tabs<SearchType> size="lg" label="Result type" items={tabs} value={type} onChange={(t) => setQuery(urlQuery, t)} />}
        </section>

        {!hasQuery && <SearchBrowse onPick={pick} />}

        {hasQuery && search.isPending && <SearchSkeleton type={type} />}

        {hasQuery && search.isError && (
          <>
            <InlineAlert
              title="Search isn't responding"
              description="ICYRE couldn't reach the catalogue. Your library and downloads still work."
              onRetry={() => void search.refetch()}
            />
            <RecentSearches onPick={pick} />
          </>
        )}

        {hasQuery && data && !search.isError && (
          total(data) === 0 && !search.isPlaceholderData ? (
            <EmptyState
              title={`No results for “${urlQuery}”`}
              description="Check the spelling, or search for an artist, track or album name."
              action={
                data.didYouMean ? (
                  <Button variant="secondary" iconStart="search" onClick={() => pick(data.didYouMean!)}>
                    Search for “{data.didYouMean}”
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <SearchResults data={data} type={type} onTab={(t) => setQuery(urlQuery, t)} stale={search.isPlaceholderData} />
          )
        )}
      </div>
    </>
  );
}

function total(r: SearchResult): number {
  return r.counts.tracks + r.counts.artists + r.counts.albums + r.counts.playlists;
}
