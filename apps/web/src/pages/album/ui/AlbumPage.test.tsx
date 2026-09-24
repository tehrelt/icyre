import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { usePlayerStore } from '@/features/player';
import { renderApp } from '@/test/render';

describe('Album screen', () => {
  it('renders the canvas album with its tracklist', async () => {
    renderApp('/album/alb-prism-hours');
    expect(await screen.findByRole('heading', { level: 1, name: 'Prism Hours' })).toBeInTheDocument();
    expect(screen.getByText('No. 014')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Nova Hale' })).toBeInTheDocument();
    expect(screen.getByText(/2026 · 11 tracks · 42 min/)).toBeInTheDocument();

    const tracks = screen.getByRole('table', { name: 'Tracks' });
    const rows = within(tracks).getAllByRole('row');
    expect(rows).toHaveLength(12); // header + 11
    expect(within(tracks).getByText('4,431,118')).toBeInTheDocument();
    expect(within(tracks).getByText('Not available in your region')).toBeInTheDocument();
    expect(screen.getByText('Released 6 March 2026')).toBeInTheDocument();

    expect(screen.getByRole('heading', { name: 'More by Nova Hale' })).toBeInTheDocument();
    expect(screen.getByText('EP · 2023')).toBeInTheDocument();

    const crumbs = screen.getByRole('navigation', { name: 'Breadcrumb' });
    expect(within(crumbs).getByText('Prism Hours')).toHaveAttribute('aria-current', 'page');
  });

  it('plays the album and toggles pause from the hero button', async () => {
    renderApp('/album/alb-prism-hours');
    await screen.findByRole('heading', { level: 1, name: 'Prism Hours' });
    const main = screen.getByRole('main');
    await userEvent.click(within(main).getByRole('button', { name: 'Play' }));

    expect(usePlayerStore.getState().source).toEqual({ kind: 'album', id: 'alb-prism-hours' });
    expect(usePlayerStore.getState().currentTrack?.id).toBe('trk-frozen-choir');
    const tracks = screen.getByRole('table', { name: 'Tracks' });
    await waitFor(() => expect(within(tracks).getAllByRole('row')[1]).toHaveClass('is-current'));

    await userEvent.click(within(main).getByRole('button', { name: 'Pause' }));
    expect(usePlayerStore.getState().isPlaying).toBe(false);
  });

  it('starts from a track row and skips the unavailable one', async () => {
    renderApp('/album/alb-prism-hours');
    const tracks = await screen.findByRole('table', { name: 'Tracks' });
    expect(within(tracks).queryByRole('button', { name: 'Play Northern Index' })).not.toBeInTheDocument();

    await userEvent.click(within(tracks).getByRole('button', { name: 'Play Vapour Trail' }));
    expect(usePlayerStore.getState().currentTrack?.id).toBe('trk-vapour-trail');
    usePlayerStore.getState().next();
    expect(usePlayerStore.getState().currentTrack?.id).toBe('trk-rime');
  });

  it('shows a not-found state for unknown albums', async () => {
    renderApp('/album/does-not-exist');
    expect(await screen.findByRole('heading', { name: 'Album not found' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Back to Home/ })).toBeInTheDocument();
  });

  it('opens from a Home card', async () => {
    const { router } = renderApp('/');
    const recent = await screen.findByRole('heading', { name: 'Recently played' });
    await userEvent.click(within(recent.closest('section')!).getByRole('link', { name: /^Prism Hours\s*Album/ }));
    expect(router.state.location.pathname).toBe('/album/alb-prism-hours');
    expect(await screen.findByRole('heading', { level: 1, name: 'Prism Hours' })).toBeInTheDocument();
  });
});
