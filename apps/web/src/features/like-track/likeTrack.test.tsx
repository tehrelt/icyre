import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { mockAuth, mockLibrary } from '@/shared/api/mock/server';
import { resetSessionForTests } from '@/shared/api/session';
import { renderApp } from '@/test/render';

import { useLikeStore } from './model/likeStore';

const likedCount = () => within(screen.getByRole('link', { name: /Liked tracks/ })).queryByText(/^\d+$/)?.textContent;

beforeEach(() => {
  resetSessionForTests();
  mockAuth.signedIn = true;
  mockAuth.tokens.clear();
  useLikeStore.setState({ overrides: {} });
});

describe('Liking tracks', () => {
  it('saves a track to Liked tracks and updates the sidebar counter', async () => {
    renderApp('/album/alb-prism-hours');
    const tracks = await screen.findByRole('table', { name: 'Tracks' });
    await waitFor(() => expect(likedCount()).toBe('342'));

    const row = within(tracks).getByRole('row', { name: /Frozen Choir/ });
    await userEvent.click(within(row).getByRole('button', { name: 'Add to Liked' }));

    expect(within(row).getByRole('button', { name: 'Remove from Liked' })).toHaveAttribute('aria-pressed', 'true');
    await waitFor(() => expect(likedCount()).toBe('343'));
    expect(mockLibrary.tracks.has('trk-frozen-choir')).toBe(true);

    await userEvent.click(within(row).getByRole('button', { name: 'Remove from Liked' }));
    await waitFor(() => expect(likedCount()).toBe('342'));
    expect(mockLibrary.tracks.has('trk-frozen-choir')).toBe(false);
  });

  it('rolls the heart back when there is no session', async () => {
    mockAuth.signedIn = false;
    renderApp('/album/alb-prism-hours');
    const tracks = await screen.findByRole('table', { name: 'Tracks' });
    const row = within(tracks).getByRole('row', { name: /Frozen Choir/ });

    await userEvent.click(within(row).getByRole('button', { name: 'Add to Liked' }));
    await waitFor(() => expect(within(row).getByRole('button', { name: 'Add to Liked' })).toHaveAttribute('aria-pressed', 'false'));
    expect(mockLibrary.tracks.has('trk-frozen-choir')).toBe(false);
  });
});
