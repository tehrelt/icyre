import { cx } from '@/shared/lib/cx';
import { Button, Icon, Popover } from '@/shared/ui';

import { useAudioDevices } from '../model/useAudioDevices';

import styles from './DevicePicker.module.css';
import { DeviceGlyph } from './PlayerGlyphs';

export interface DevicePickerProps {
  placement?: 'top' | 'bottom';
  align?: 'start' | 'end';
  size?: 'sm' | 'md';
}

/** Audio output device selector popover for the player bar and expanded player. */
export function DevicePicker({ placement = 'top', align = 'end', size = 'sm' }: DevicePickerProps) {
  const { devices, selectedId, pendingId, error, selectDevice, canSwitchOutput, canEnumerate, refreshing, refresh } = useAudioDevices();
  const activeNonDefault = selectedId !== 'default';

  return (
    <Popover
      title="Audio output"
      placement={placement}
      align={align}
      width={320}
      trigger={
        <button
          type="button"
          className={cx('ic-iconbtn', activeNonDefault ? 'ic-iconbtn-active' : 'ic-iconbtn-neutral', `ic-iconbtn-${size}`)}
          aria-label="Connect to a device"
          title="Audio output"
        >
          <DeviceGlyph />
        </button>
      }
      footer={
        canEnumerate ? (
          <Button variant="ghost" size="sm" loading={refreshing} onClick={() => void refresh()}>
            Refresh devices
          </Button>
        ) : undefined
      }
    >
      <div role="radiogroup" aria-label="Audio output devices" className={styles.list}>
        {devices.map((device) => {
          const selected = device.id === selectedId;
          const pending = device.id === pendingId;
          const disabled = (!canSwitchOutput && device.id !== 'default') || pendingId !== null;
          return (
            <button
              key={device.id}
              type="button"
              role="radio"
              aria-checked={selected}
              disabled={disabled}
              className={cx(styles.row, selected && styles.selected)}
              onClick={() => selectDevice(device.id)}
            >
              <DeviceGlyph />
              <span className={styles.info}>
                <span className={styles.name}>{device.label}</span>
                <span className={styles.kind}>
                  {pending ? 'Switching…' : selected ? 'Active output' : device.kind === 'default' ? 'Browser audio' : 'External output'}
                </span>
              </span>
              {selected ? <Icon name="check" size={16} className={styles.check} /> : null}
            </button>
          );
        })}
      </div>
      {error ? (
        <p role="alert" className={styles.error}>
          {error}
        </p>
      ) : null}
      {!canSwitchOutput ? (
        <p className={styles.note}>Output switching isn&apos;t supported in this browser. Audio plays through your system default.</p>
      ) : null}
    </Popover>
  );
}
