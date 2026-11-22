import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { useFollowStore } from '@/features/follow-artist';
import { useLikeStore } from '@/features/like-track/model/likeStore';
import { usePlayerStore } from '@/features/player';
import { useAlbumSaveStore } from '@/features/save-album';
import { mockAuth, mockFaults, mockLibrary } from '@/shared/api/mock/server';
import { resetSessionForTests } from '@/shared/api/session';
import { renderApp } from '@/test/render';

const INITIAL_TRACKS = Array.from(mockLibrary.tracks);
const INITIAL_ALBUMS = Array.from(mockLibrary.albums);
const INITIAL_ARTISTS = Array.from(mockLibrary.followedArtists);

beforeEach(() => {
  resetSessionForTests();
  mockAuth.signedIn = true;
  mockAuth.email = 'rin@example.com';
  mockAuth.tokens.clear();
  mockFaults.clear();
  mockLibrary.tracks = new Set(INITIAL_TRACKS);
  mockLibrary.albums = new Set(INITIAL_ALBUMS);
  mockLibrary.followedArtists = new Set(INITIAL_ARTISTS);
  useLikeStore.setState({ overrides: {} });
  useAlbumSaveStore.setState({ overrides: {} });
  useFollowStore.setState({ overrides: {} });
});

afterEach(() => {
  mockFaults.clear();
});

describe('Library screen', () => {
  it('renders overview with liked spotlight, playlists, saved albums and followed artists', async () => {
    renderApp('/library');
    expect(await screen.findByRole('heading', { level: 1, name: 'Your library' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Your saved songs' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Playlists' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Saved albums' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Followed artists' })).toBeInTheDocument();

    expect(screen.getByRole('button', { name: /^Late night drives/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Prism Hours/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Nova Hale/ })).toBeInTheDocument();
  });

  it('switches tabs and syncs URL query parameter', async () => {
    const { router } = renderApp('/library');
    await screen.findByRole('heading', { level: 1, name: 'Your library' });

    await userEvent.click(screen.getByRole('tab', { name: /Albums/ }));
    expect(router.state.location.search).toBe('?tab=albums');
    expect(screen.queryByRole('heading', { name: 'Playlists' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Salt Lanterns/ })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('tab', { name: /Liked tracks/ }));
    expect(router.state.location.search).toBe('?tab=tracks');
    const table = screen.getByRole('table', { name: 'Liked tracks' });
    expect(within(table).getByText('Glass Tides')).toBeInTheDocument();
  });

  it('filters library items and clears empty filter state', async () => {
    renderApp('/library');
    await screen.findByRole('heading', { level: 1, name: 'Your library' });

    const filterInput = screen.getByRole('searchbox', { name: 'Filter library' });
    await userEvent.type(filterInput, 'Salt Lanterns');

    expect(screen.getByRole('button', { name: /^Salt Lanterns/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Late night drives/ })).not.toBeInTheDocument();

    await userEvent.clear(filterInput);
    await userEvent.type(filterInput, 'nonexistent-xyz');
    expect(await screen.findByRole('heading', { name: /No matches for/ })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Clear filter' }));
    expect(await screen.findByRole('heading', { name: 'Playlists' })).toBeInTheDocument();
  });

  it('sorts items alphabetically when A–Z is selected', async () => {
    renderApp('/library?tab=albums');
    await screen.findByRole('button', { name: /^Prism Hours/ });

    await userEvent.click(screen.getByRole('radio', { name: 'A–Z' }));
    const titles = screen
      .getAllByRole('button', { name: /^(Prism Hours|Salt Lanterns|Hollow Signal|Winter Index|Lowlight Atlas|Quiet Machines)/ })
      .map((el) => el.querySelector('.ic-card-title')?.textContent);

    expect(titles).toEqual(['Hollow Signal', 'Lowlight Atlas', 'Prism Hours', 'Quiet Machines', 'Salt Lanterns', 'Winter Index']);
  });

  it('plays liked tracks from the spotlight and updates the global player', async () => {
    renderApp('/library');
    await screen.findByRole('heading', { name: 'Your saved songs' });

    await userEvent.click(screen.getByRole('button', { name: 'Play liked' }));
    await waitFor(() => expect(usePlayerStore.getState().source).toEqual({ kind: 'library', id: 'liked-tracks' }));
    expect(usePlayerStore.getState().isPlaying).toBe(true);
    expect(within(screen.getByRole('main')).getByRole('button', { name: 'Pause' })).toBeInTheDocument();
  });

  it('removes a saved album from library and updates sidebar counter', async () => {
    renderApp('/library?tab=albums');
    await screen.findByRole('button', { name: /^Prism Hours/ });

    await userEvent.click(screen.getByRole('button', { name: 'Options for Prism Hours' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Remove from library' }));

    await waitFor(() => expect(screen.queryByRole('button', { name: /^Prism Hours/ })).not.toBeInTheDocument());
    expect(mockLibrary.albums.has('alb-prism-hours')).toBe(false);
    const sidebarNav = screen.getAllByRole('navigation', { name: 'Main' })[0]!;
    const libraryLink = within(sidebarNav).getByRole('link', { name: /Library/ });
    await waitFor(() => expect(within(libraryLink).getByText('127')).toBeInTheDocument());
  });

  it('unfollows an artist from the Artists tab', async () => {
    renderApp('/library?tab=artists');
    await screen.findByRole('button', { name: /^Nova Hale/ });

    await userEvent.click(screen.getByRole('button', { name: 'Options for Nova Hale' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Unfollow' }));

    await waitFor(() => expect(screen.queryByRole('button', { name: /^Nova Hale/ })).not.toBeInTheDocument());
    expect(mockLibrary.followedArtists.has('art-nova-hale')).toBe(false);
  });

  it('renders /liked route with hero, breadcrumb and unlikes a track', async () => {
    renderApp('/liked');
    expect(await screen.findByRole('heading', { level: 1, name: 'Liked tracks' })).toBeInTheDocument();
    const crumbs = screen.getByRole('navigation', { name: 'Breadcrumb' });
    expect(within(crumbs).getByRole('link', { name: 'Library' })).toBeInTheDocument();

    const table = screen.getByRole('table', { name: 'Liked tracks' });
    const row = within(table).getByRole('row', { name: /Glass Tides/ });
    await userEvent.click(within(row).getByRole('button', { name: 'Remove from Liked' }));

    await waitFor(() => expect(within(table).queryByRole('row', { name: /Glass Tides/ })).not.toBeInTheDocument());
    expect(mockLibrary.tracks.has('trk-glass-tides')).toBe(false);
  });

  it('shows sign-in prompt when visitor is signed out', async () => {
    mockAuth.signedIn = false;
    const { router } = renderApp('/library');

    const heading = await screen.findByRole('heading', { name: 'Sign in to view your library' });
    await userEvent.click(within(heading.closest('section')!).getByRole('button', { name: 'Sign in' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
  });

  it('shows error alert with retry when library upstream fails', async () => {
    mockFaults.add('/me/library/albums');
    renderApp('/library');

    const alert = await screen.findByRole('alert');
    expect(within(alert).getByText("Library isn't responding")).toBeInTheDocument();
    mockFaults.delete('/me/library/albums');
    await userEvent.click(within(alert).getByRole('button', { name: 'Try again' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'Your library' })).toBeInTheDocument();
  });
});
