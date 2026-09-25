import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { usePlayerStore } from '@/features/player';
import { renderApp } from '@/test/render';

describe('Home screen', () => {
  it('renders the canvas sections from the home feed', async () => {
    renderApp('/');
    expect(await screen.findByRole('heading', { level: 1, name: /, Rin$/ })).toBeInTheDocument();
    for (const name of ['Recently played', 'Salt Lanterns', 'Recommended for you', 'New releases', 'Trending on ICYRE', 'Made for you', 'Artists you follow']) {
      expect(screen.getByRole('heading', { name })).toBeInTheDocument();
    }
    expect(screen.getByRole('table', { name: 'Trending tracks' })).toBeInTheDocument();
    expect(screen.getByText('CF-027')).toBeInTheDocument();
    // Sidebar is fed by the library summary.
    const playlists = screen.getByRole('navigation', { name: 'Playlists' });
    expect(await within(playlists).findByRole('link', { name: /Late night drives/ })).toBeInTheDocument();
  });

  it('plays the album of the week in the global player', async () => {
    renderApp('/');
    const player = screen.getByRole('region', { name: 'Player' });
    expect(within(player).getByText('Nothing playing')).toBeInTheDocument();

    await userEvent.click(await screen.findByRole('button', { name: 'Play album' }));

    await waitFor(() => expect(usePlayerStore.getState().source).toEqual({ kind: 'album', id: 'alb-salt-lanterns' }));
    expect(within(player).getByText('Mira Solen')).toBeInTheDocument();
    expect(within(player).getByRole('button', { name: 'Pause' })).toBeInTheDocument();
    // The CTA flips to pause while the album plays.
    const aotw = screen.getByRole('region', { name: 'Salt Lanterns' });
    expect(within(aotw).getByRole('button', { name: 'Pause' })).toHaveClass('ic-btn-iridescent');
  });

  it('plays a trending track and marks its row as current', async () => {
    renderApp('/');
    const table = await screen.findByRole('table', { name: 'Trending tracks' });
    await userEvent.click(within(table).getByRole('button', { name: 'Play Soft Engines' }));

    expect(usePlayerStore.getState().currentTrack?.id).toBe('trk-soft-engines');
    const row = within(table).getAllByRole('row')[1]!;
    expect(row).toHaveClass('is-current');
  });

  it('filters shelves with the chips', async () => {
    renderApp('/');
    await screen.findByRole('heading', { name: 'Recommended for you' });

    await userEvent.click(screen.getByRole('button', { name: 'Artists' }));
    expect(screen.getByRole('button', { name: 'Artists' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByRole('heading', { name: 'New releases' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Trending on ICYRE' })).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Artists you follow' })).toBeInTheDocument();
  });
});

describe('App shell', () => {
  it('keeps playback running across navigation', async () => {
    const { router } = renderApp('/');
    await userEvent.click(await screen.findByRole('button', { name: 'Play album' }));
    await waitFor(() => expect(usePlayerStore.getState().isPlaying).toBe(true));

    await router.navigate('/library');
    expect(await screen.findByRole('heading', { name: "This screen isn't built yet" })).toBeInTheDocument();
    const player = screen.getByRole('region', { name: 'Player' });
    expect(within(player).getByText('Mira Solen')).toBeInTheDocument();
    expect(usePlayerStore.getState().isPlaying).toBe(true);
  });

  it('marks the active sidebar item', async () => {
    renderApp('/');
    const nav = screen.getAllByRole('navigation', { name: 'Main' })[0]!;
    expect(within(nav).getByRole('link', { name: /Home/ })).toHaveAttribute('aria-current', 'page');
  });
});

describe('Home with a sparse BFF page', () => {
  it('shows only the sections the BFF could fill', async () => {
    const { mockOverrides } = await import('@/shared/api/mock/server');
    mockOverrides.set('/pages/home', () => ({
      status: 200,
      json: {
        recentlyPlayed: [],
        albumOfTheWeek: null,
        recommended: { items: [] },
        newReleases: {
          kicker: 'Out this week',
          items: [{ kind: 'album', id: 'alb-prism-hours', title: 'Prism Hours', artistName: 'Nova Hale', year: 2026, explicit: false, coverUrl: null, art: 0 }],
        },
        trending: { today: [], week: [] },
        madeForYou: { featured: null, playlists: [] },
        followedArtists: [],
        unavailable: ['recentlyPlayed', 'albumOfTheWeek', 'recommended', 'trending', 'madeForYou', 'followedArtists'],
      },
    }));
    try {
      renderApp('/');
      expect(await screen.findByRole('heading', { name: 'New releases' })).toBeInTheDocument();
      for (const name of ['Recently played', 'Recommended for you', 'Trending on ICYRE', 'Made for you', 'Artists you follow']) {
        expect(screen.queryByRole('heading', { name })).not.toBeInTheDocument();
      }
      expect(screen.queryByRole('button', { name: 'Play album' })).not.toBeInTheDocument();
    } finally {
      mockOverrides.clear();
    }
  });
});
