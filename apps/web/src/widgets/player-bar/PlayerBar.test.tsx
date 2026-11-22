import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import type { Track } from '@/entities/track/model';
import { useLikeStore } from '@/features/like-track';
import { SimulatedAudioEngine } from '@/features/player/engine/SimulatedAudioEngine';
import { initialPlayerState, usePlayerStore } from '@/features/player/model/playerStore';
import { mockAuth, mockLibrary } from '@/shared/api/mock/server';
import { resetSessionForTests } from '@/shared/api/session';
import { renderApp } from '@/test/render';

const makeTrack = (id: string, title: string, extra: Partial<Track> = {}): Track => ({
  id,
  title,
  artistName: 'Nova Hale',
  albumId: 'alb-prism-hours',
  albumTitle: 'Prism Hours',
  durationSec: 210,
  explicit: false,
  coverUrl: null,
  art: 0,
  liked: false,
  available: true,
  ...extra,
});

const sampleQueue = [
  makeTrack('trk-1', 'Night Index'),
  makeTrack('trk-2', 'Glass Tides', { explicit: true }),
  makeTrack('trk-3', 'Hollow Signal'),
  makeTrack('trk-4', 'Pearl Static'),
];

beforeEach(() => {
  resetSessionForTests();
  mockAuth.signedIn = true;
  mockAuth.tokens.clear();
  useLikeStore.setState({ overrides: {} });
  usePlayerStore.setState(initialPlayerState);
});

describe('PlayerBar — Queue drawer, Like, Device picker, Expanded player', () => {
  it('opens the queue drawer, plays, removes, and clears tracks', async () => {
    renderApp('/');
    await act(async () => {
      usePlayerStore.getState().playQueue(sampleQueue, 1, { kind: 'album', id: 'alb-prism-hours' });
    });

    const player = screen.getByRole('region', { name: 'Player' });
    await userEvent.click(within(player).getByRole('button', { name: 'Queue' }));

    const drawer = await screen.findByRole('dialog', { name: 'Queue' });
    expect(within(drawer).getByText('Played · 1')).toBeInTheDocument();
    expect(within(drawer).getByText('Now playing')).toBeInTheDocument();
    expect(within(drawer).getByText('Next up · 2')).toBeInTheDocument();

    // Jump to an upcoming track from the queue.
    await userEvent.click(within(drawer).getByRole('button', { name: 'Play Hollow Signal' }));
    expect(usePlayerStore.getState().currentTrack?.id).toBe('trk-3');

    // Remove an upcoming track.
    await userEvent.click(within(drawer).getByRole('button', { name: 'Remove Pearl Static from queue' }));
    expect(usePlayerStore.getState().queue.map((t) => t.id)).toEqual(['trk-1', 'trk-2', 'trk-3']);

    // Clear the entire queue and verify empty state.
    await userEvent.click(within(drawer).getByRole('button', { name: 'Clear queue' }));
    expect(within(drawer).getByText('Your queue is empty')).toBeInTheDocument();
    expect(usePlayerStore.getState().currentTrack).toBeNull();
  });

  it('clears only upcoming tracks from the queue drawer', async () => {
    renderApp('/');
    await act(async () => {
      usePlayerStore.getState().playQueue(sampleQueue, 1);
    });

    const player = screen.getByRole('region', { name: 'Player' });
    await userEvent.click(within(player).getByRole('button', { name: 'Queue' }));

    const drawer = await screen.findByRole('dialog', { name: 'Queue' });
    await userEvent.click(within(drawer).getByRole('button', { name: 'Clear' }));
    expect(usePlayerStore.getState().queue.map((t) => t.id)).toEqual(['trk-1', 'trk-2']);
    expect(within(drawer).queryByText(/Next up/)).not.toBeInTheDocument();
  });

  it('likes and unlikes the current track from the player bar', async () => {
    renderApp('/');
    await act(async () => {
      usePlayerStore.getState().playQueue([makeTrack('trk-like-test', 'Glass Tides')]);
    });

    const player = screen.getByRole('region', { name: 'Player' });
    const likeBtn = await within(player).findByRole('button', { name: 'Save to Liked tracks' });
    await userEvent.click(likeBtn);

    await waitFor(() => expect(within(player).getByRole('button', { name: 'Remove from Liked tracks' })).toHaveAttribute('aria-pressed', 'true'));
    expect(mockLibrary.tracks.has('trk-like-test')).toBe(true);

    await userEvent.click(within(player).getByRole('button', { name: 'Remove from Liked tracks' }));
    await waitFor(() => expect(within(player).getByRole('button', { name: 'Save to Liked tracks' })).toHaveAttribute('aria-pressed', 'false'));
    expect(mockLibrary.tracks.has('trk-like-test')).toBe(false);
  });

  it('lists audio output devices and selects one when setSinkId is supported', async () => {
    const enumerateDevices = vi.fn(async () => [
      { deviceId: 'default', kind: 'audiooutput', label: 'Built-in Speakers', groupId: 'g1', toJSON: () => ({}) },
      { deviceId: 'dac-1', kind: 'audiooutput', label: 'USB-C Reference DAC', groupId: 'g2', toJSON: () => ({}) },
      { deviceId: 'mic-1', kind: 'audioinput', label: 'Studio Mic', groupId: 'g3', toJSON: () => ({}) },
    ]);
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        enumerateDevices,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    });
    Object.defineProperty(HTMLAudioElement.prototype, 'setSinkId', {
      configurable: true,
      value: vi.fn(async () => undefined),
    });

    try {
      renderApp('/');
      const player = screen.getByRole('region', { name: 'Player' });
      await userEvent.click(within(player).getByRole('button', { name: 'Connect to a device' }));

      const dialog = await screen.findByRole('dialog', { name: 'Audio output' });
      expect(within(dialog).getByRole('radio', { name: /System default \(Built-in Speakers\)/ })).toHaveAttribute('aria-checked', 'true');
      const dacOption = within(dialog).getByRole('radio', { name: /USB-C Reference DAC/ });
      await userEvent.click(dacOption);

      await waitFor(() => expect(usePlayerStore.getState().outputDeviceId).toBe('dac-1'));
      expect(dacOption).toHaveAttribute('aria-checked', 'true');
      expect(within(dacOption).getByText('Active output')).toBeInTheDocument();
      expect(within(dialog).queryByText(/Output switching isn't supported/)).not.toBeInTheDocument();
    } finally {
      delete (HTMLAudioElement.prototype as unknown as { setSinkId?: unknown }).setSinkId;
    }
  });

  it('keeps the previous active output and shows an error when switching fails', async () => {
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        enumerateDevices: vi.fn(async () => [
          { deviceId: 'default', kind: 'audiooutput', label: 'Built-in Speakers', groupId: 'g1', toJSON: () => ({}) },
          { deviceId: 'dac-1', kind: 'audiooutput', label: 'USB-C Reference DAC', groupId: 'g2', toJSON: () => ({}) },
        ]),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    });
    Object.defineProperty(HTMLAudioElement.prototype, 'setSinkId', {
      configurable: true,
      value: vi.fn(async () => undefined),
    });
    const spy = vi.spyOn(SimulatedAudioEngine.prototype, 'setOutputDevice').mockRejectedValueOnce(new DOMException('Device disconnected', 'NotFoundError'));

    try {
      renderApp('/');
      const player = screen.getByRole('region', { name: 'Player' });
      await userEvent.click(within(player).getByRole('button', { name: 'Connect to a device' }));

      const dialog = await screen.findByRole('dialog', { name: 'Audio output' });
      const defaultOption = within(dialog).getByRole('radio', { name: /System default \(Built-in Speakers\)/ });
      const dacOption = within(dialog).getByRole('radio', { name: /USB-C Reference DAC/ });

      await userEvent.click(dacOption);

      await waitFor(() => expect(within(dialog).getByRole('alert')).toHaveTextContent('Could not switch audio output'));
      expect(usePlayerStore.getState().outputDeviceId).toBe('default');
      expect(defaultOption).toHaveAttribute('aria-checked', 'true');
      expect(within(defaultOption).getByText('Active output')).toBeInTheDocument();
      expect(dacOption).toHaveAttribute('aria-checked', 'false');
      expect(within(dacOption).queryByText('Active output')).not.toBeInTheDocument();
    } finally {
      spy.mockRestore();
      delete (HTMLAudioElement.prototype as unknown as { setSinkId?: unknown }).setSinkId;
    }
  });

  it('disables overlapping device clicks while switching and keeps A active if B rejects', async () => {
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        enumerateDevices: vi.fn(async () => [
          { deviceId: 'default', kind: 'audiooutput', label: 'Built-in Speakers', groupId: 'g1', toJSON: () => ({}) },
          { deviceId: 'dac-a', kind: 'audiooutput', label: 'Studio DAC A', groupId: 'g2', toJSON: () => ({}) },
          { deviceId: 'dac-b', kind: 'audiooutput', label: 'Studio DAC B', groupId: 'g3', toJSON: () => ({}) },
        ]),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    });
    Object.defineProperty(HTMLAudioElement.prototype, 'setSinkId', {
      configurable: true,
      value: vi.fn(async () => undefined),
    });

    const deferred = new Map<string, { resolve: () => void; reject: (err: Error) => void }>();
    const spy = vi.spyOn(SimulatedAudioEngine.prototype, 'setOutputDevice').mockImplementation(
      (id: string) =>
        new Promise<void>((resolve, reject) => {
          deferred.set(id, { resolve, reject });
        }),
    );

    try {
      renderApp('/');
      const player = screen.getByRole('region', { name: 'Player' });
      await userEvent.click(within(player).getByRole('button', { name: 'Connect to a device' }));

      const dialog = await screen.findByRole('dialog', { name: 'Audio output' });
      const optionA = within(dialog).getByRole('radio', { name: /Studio DAC A/ });
      const optionB = within(dialog).getByRole('radio', { name: /Studio DAC B/ });

      await userEvent.click(optionA);
      expect(optionA).toBeDisabled();
      expect(optionB).toBeDisabled();
      expect(within(optionA).getByText('Switching…')).toBeInTheDocument();

      await act(async () => {
        deferred.get('dac-a')!.resolve();
      });

      await waitFor(() => expect(optionA).toHaveAttribute('aria-checked', 'true'));
      expect(within(optionA).getByText('Active output')).toBeInTheDocument();
      expect(optionB).not.toBeDisabled();

      await userEvent.click(optionB);
      expect(within(optionB).getByText('Switching…')).toBeInTheDocument();

      await act(async () => {
        deferred.get('dac-b')!.reject(new DOMException('Device disconnected', 'NotFoundError'));
      });

      await waitFor(() => expect(within(dialog).getByRole('alert')).toHaveTextContent('Could not switch audio output'));
      expect(usePlayerStore.getState().outputDeviceId).toBe('dac-a');
      expect(optionA).toHaveAttribute('aria-checked', 'true');
      expect(within(optionA).getByText('Active output')).toBeInTheDocument();
      expect(optionB).toHaveAttribute('aria-checked', 'false');
      expect(within(optionB).queryByText('Active output')).not.toBeInTheDocument();
    } finally {
      spy.mockRestore();
      delete (HTMLAudioElement.prototype as unknown as { setSinkId?: unknown }).setSinkId;
    }
  });

  it('disables external outputs and shows a note when setSinkId is unsupported', async () => {
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        enumerateDevices: vi.fn(async () => [
          { deviceId: 'default', kind: 'audiooutput', label: 'Built-in Speakers', groupId: 'g1', toJSON: () => ({}) },
          { deviceId: 'dac-1', kind: 'audiooutput', label: 'USB-C Reference DAC', groupId: 'g2', toJSON: () => ({}) },
        ]),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
    });

    renderApp('/');
    const player = screen.getByRole('region', { name: 'Player' });
    await userEvent.click(within(player).getByRole('button', { name: 'Connect to a device' }));

    const dialog = await screen.findByRole('dialog', { name: 'Audio output' });
    expect(within(dialog).getByRole('radio', { name: /USB-C Reference DAC/ })).toBeDisabled();
    expect(within(dialog).getByText(/Output switching isn't supported in this browser/)).toBeInTheDocument();
  });

  it('opens the expanded player, controls playback, likes, and navigates to the album', async () => {
    const { router } = renderApp('/');
    await act(async () => {
      usePlayerStore.getState().playQueue(sampleQueue, 1, { kind: 'album', id: 'alb-prism-hours' });
    });

    const player = screen.getByRole('region', { name: 'Player' });
    await userEvent.click(await within(player).findByRole('button', { name: 'Open expanded player' }));

    const expanded = await screen.findByRole('dialog', { name: 'Now playing' });
    expect(within(expanded).getByRole('heading', { level: 2, name: 'Glass Tides' })).toBeInTheDocument();
    expect(within(expanded).getByText('Playing from album')).toBeInTheDocument();

    // Toggle play/pause inside expanded player.
    await userEvent.click(within(expanded).getByRole('button', { name: 'Pause' }));
    expect(usePlayerStore.getState().isPlaying).toBe(false);

    // Like inside expanded player.
    await userEvent.click(within(expanded).getByRole('button', { name: 'Save to Liked tracks' }));
    await waitFor(() => expect(mockLibrary.tracks.has('trk-2')).toBe(true));

    // Navigate to album from expanded player and verify it closes.
    await userEvent.click(within(expanded).getByRole('link', { name: 'Prism Hours' }));
    expect(router.state.location.pathname).toBe('/album/alb-prism-hours');
    expect(screen.queryByRole('dialog', { name: 'Now playing' })).not.toBeInTheDocument();
  });

  it('opens the expanded player from the track meta button and closes on Escape', async () => {
    renderApp('/');
    await act(async () => {
      usePlayerStore.getState().playQueue([
        makeTrack('trk-long', 'A Very Long Track Title That Should Truncate Cleanly On Narrow Mobile Screens', {
          artistName: 'An Equally Long Artist Name That Must Not Push Controls',
        }),
      ]);
    });

    const player = screen.getByRole('region', { name: 'Player' });
    const metaBtn = await within(player).findByRole('button', {
      name: 'Open expanded player for A Very Long Track Title That Should Truncate Cleanly On Narrow Mobile Screens',
    });
    expect(metaBtn.className).toMatch(/metaBtn/);
    const titleEl = within(metaBtn).getByText('A Very Long Track Title That Should Truncate Cleanly On Narrow Mobile Screens');
    const artistEl = within(metaBtn).getByText('An Equally Long Artist Name That Must Not Push Controls');
    expect(titleEl).not.toBe(artistEl);

    await userEvent.click(metaBtn);

    expect(await screen.findByRole('dialog', { name: 'Now playing' })).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('dialog', { name: 'Now playing' })).not.toBeInTheDocument();
  });
});
