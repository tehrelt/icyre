import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { useRecentSearches } from '@/features/search-history/recentSearches';
import { usePlayerStore } from '@/features/player';
import { mockFaults } from '@/shared/api/mock/server';
import { renderApp } from '@/test/render';

beforeEach(() => {
  useRecentSearches.setState({ items: ['salt lanterns', 'ambient folk'] });
  mockFaults.clear();
});

describe('Search screen', () => {
  it('shows browse content before a query', async () => {
    renderApp('/search');
    expect(screen.getByRole('heading', { level: 1, name: 'Search' })).toBeInTheDocument();
    expect(screen.getByRole('searchbox', { name: 'Search ICYRE' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Recent searches' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: /Hip-hop\s*12,655 releases/ })).toHaveAttribute('href', '/genre/hip-hop');
    expect(screen.getByRole('button', { name: 'Slow morning' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Collections from ICYRE' })).toBeInTheDocument();
  });

  it('searches as you type and syncs the URL', async () => {
    const { router } = renderApp('/search');
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search ICYRE' }), 'nova');

    const top = await screen.findByRole('heading', { name: 'Top result' });
    expect(router.state.location.search).toBe('?q=nova');
    const topCard = top.closest('section')!;
    expect(within(topCard).getByRole('link', { name: 'Nova Hale' })).toHaveAttribute('href', '/artist/art-nova-hale');
    expect(within(topCard).getByText('Verified')).toBeInTheDocument();
    expect(screen.getByRole('table', { name: 'Matching tracks' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Artists' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /Artists/ })).toHaveAttribute('aria-selected', 'false');
    expect(screen.queryByRole('heading', { level: 1, name: 'Search' })).not.toBeInTheDocument();
  });

  it('switches result tabs', async () => {
    const { router } = renderApp('/search?q=nova');
    await screen.findByRole('heading', { name: 'Top result' });
    await userEvent.click(screen.getByRole('tab', { name: /Playlists/ }));

    await waitFor(() => expect(router.state.location.search).toBe('?q=nova&type=playlists'));
    expect(await screen.findByText('This is Nova Hale')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Top result' })).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /Playlists/ })).toHaveAttribute('aria-selected', 'true');
  });

  it('plays the top result', async () => {
    renderApp('/search?q=nova');
    const top = (await screen.findByRole('heading', { name: 'Top result' })).closest('section')!;
    await userEvent.click(within(top).getByRole('button', { name: 'Play Nova Hale' }));
    await waitFor(() => expect(usePlayerStore.getState().source).toEqual({ kind: 'artist', id: 'art-nova-hale' }));
  });

  it('suggests a correction when nothing matches', async () => {
    renderApp('/search?q=nvoa%20hlae');
    expect(await screen.findByRole('heading', { name: 'No results for “nvoa hlae”' })).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Search for “nova hale”' }));
    expect(await screen.findByRole('heading', { name: 'Top result' })).toBeInTheDocument();
    expect(useRecentSearches.getState().items[0]).toBe('nova hale');
  });

  it('shows the error state with recent searches and retries', async () => {
    mockFaults.add('/search');
    renderApp('/search?q=nova');
    expect(await screen.findByRole('alert')).toHaveTextContent("Search isn't responding");
    expect(screen.getByRole('heading', { name: 'Recent searches' })).toBeInTheDocument();

    mockFaults.clear();
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByRole('heading', { name: 'Top result' })).toBeInTheDocument();
  });

  it('manages recent searches', async () => {
    const { router } = renderApp('/search');
    await userEvent.click(screen.getByRole('button', { name: 'Remove ambient folk from recent searches' }));
    expect(useRecentSearches.getState().items).toEqual(['salt lanterns']);

    await userEvent.click(screen.getByRole('button', { name: 'Search for salt lanterns' }));
    await waitFor(() => expect(router.state.location.search).toBe('?q=salt+lanterns'));
  });

  it('focuses the field with the "/" shortcut', async () => {
    renderApp('/search');
    act(() => (document.activeElement as HTMLElement | null)?.blur());
    await userEvent.keyboard('/');
    expect(screen.getByRole('searchbox', { name: 'Search ICYRE' })).toHaveFocus();
  });
});
