import { useCallback, useEffect, useState } from 'react';

import { usePlayerStore } from './playerStore';

export interface AudioOutputDevice {
  id: string;
  label: string;
  kind: 'default' | 'external';
}

const DEFAULT_DEVICE: AudioOutputDevice = {
  id: 'default',
  label: 'System default',
  kind: 'default',
};

function supportsSinkId(): boolean {
  return typeof HTMLAudioElement !== 'undefined' && 'setSinkId' in HTMLAudioElement.prototype;
}

function supportsEnumerate(): boolean {
  return typeof navigator !== 'undefined' && typeof navigator.mediaDevices?.enumerateDevices === 'function';
}

async function enumerateAudioOutputs(): Promise<AudioOutputDevice[]> {
  if (!supportsEnumerate()) return [DEFAULT_DEVICE];
  try {
    const all = await navigator.mediaDevices.enumerateDevices();
    const outputs = all.filter((d) => d.kind === 'audiooutput');
    const defaultEntry = outputs.find((d) => d.deviceId === 'default');
    const next: AudioOutputDevice[] = [
      {
        id: 'default',
        label: defaultEntry?.label ? `System default (${defaultEntry.label})` : 'System default',
        kind: 'default',
      },
    ];
    for (const d of outputs) {
      if (!d.deviceId || d.deviceId === 'default') continue;
      next.push({
        id: d.deviceId,
        label: d.label || `Audio output ${next.length}`,
        kind: 'external',
      });
    }
    return next;
  } catch {
    return [DEFAULT_DEVICE];
  }
}

/** Lists browser audio output devices and tracks the active sink selection. */
export function useAudioDevices() {
  const selectedId = usePlayerStore((s) => s.outputDeviceId);
  const pendingId = usePlayerStore((s) => s.pendingOutputDeviceId);
  const error = usePlayerStore((s) => s.outputDeviceError);
  const selectDevice = usePlayerStore((s) => s.setOutputDevice);
  const [devices, setDevices] = useState<AudioOutputDevice[]>([DEFAULT_DEVICE]);
  const [refreshing, setRefreshing] = useState(false);

  const canSwitchOutput = supportsSinkId();
  const canEnumerate = supportsEnumerate();

  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      setDevices(await enumerateAudioOutputs());
    } finally {
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    const sync = () => {
      void enumerateAudioOutputs().then((next) => {
        if (!cancelled) setDevices(next);
      });
    };
    sync();
    if (!supportsEnumerate() || typeof navigator.mediaDevices.addEventListener !== 'function') {
      return () => {
        cancelled = true;
      };
    }
    navigator.mediaDevices.addEventListener('devicechange', sync);
    return () => {
      cancelled = true;
      navigator.mediaDevices.removeEventListener('devicechange', sync);
    };
  }, []);

  return {
    devices,
    selectedId,
    pendingId,
    error,
    selectDevice,
    canSwitchOutput,
    canEnumerate,
    refreshing,
    refresh,
  };
}
