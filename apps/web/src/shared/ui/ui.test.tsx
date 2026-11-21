import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { createRef } from 'react';

import { Button, Checkbox, Chip, Divider, IconButton, Input, PlayerButton, Progress, Radio, SegmentedControl, Switch, Textarea, TopNavItem, TrackRow } from './index';

describe('Design System components', () => {
  it('Button renders variant classes and disables while loading', () => {
    render(
      <Button variant="iridescent" size="lg" iconStart="play" loading>
        Play album
      </Button>,
    );
    const btn = screen.getByRole('button', { name: /play album/i });
    expect(btn).toHaveClass('ic-btn', 'ic-btn-iridescent', 'ic-btn-lg', 'is-loading');
    expect(btn).toBeDisabled();
  });

  it('IconButton requires an accessible label', () => {
    render(<IconButton icon="more" label="More options" />);
    expect(screen.getByRole('button', { name: 'More options' })).toBeInTheDocument();
  });

  it('Chip toggles aria-pressed', async () => {
    render(<Chip>Albums</Chip>);
    const chip = screen.getByRole('button', { name: 'Albums' });
    expect(chip).toHaveAttribute('aria-pressed', 'false');
    await userEvent.click(chip);
    expect(chip).toHaveAttribute('aria-pressed', 'true');
  });

  it('SegmentedControl is a radiogroup', async () => {
    const onChange = vi.fn();
    render(
      <SegmentedControl
        label="Period"
        options={[
          { value: 'today', label: 'Today' },
          { value: 'week', label: 'This week' },
        ]}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole('radio', { name: 'This week' }));
    expect(onChange).toHaveBeenCalledWith('week');
    expect(screen.getByRole('radio', { name: 'This week' })).toHaveAttribute('aria-checked', 'true');
  });

  it('PlayerButton communicates toggle state', () => {
    render(<PlayerButton kind="repeat" repeatMode="one" />);
    expect(screen.getByRole('button', { name: 'Repeat one' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('TrackRow shows the playing state with two cues and plays on click', async () => {
    const onPlay = vi.fn();
    const { rerender } = render(<TrackRow index={3} title="Glass Tides" artist="Nova Hale" duration={227} onPlay={onPlay} />);
    expect(screen.getByText('3:47')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Play Glass Tides' }));
    expect(onPlay).toHaveBeenCalledOnce();

    rerender(<TrackRow index={3} title="Glass Tides" artist="Nova Hale" duration={227} state="playing" />);
    expect(screen.getByRole('row')).toHaveClass('is-current');
    expect(screen.getByRole('img', { name: 'Now playing' })).toBeInTheDocument();
  });

  it('unavailable TrackRow has no play control', () => {
    render(<TrackRow index={9} title="Northern Index" artist="Nova Hale" duration={219} state="unavailable" />);
    expect(screen.queryByRole('button', { name: /play/i })).not.toBeInTheDocument();
    expect(screen.getByText('Unavailable')).toBeInTheDocument();
  });

  it('Checkbox reports native changes', async () => {
    const onChange = vi.fn();
    render(<Checkbox label="Download over Wi-Fi" description="Saves mobile data" onChange={onChange} />);
    const checkbox = screen.getByRole('checkbox', { name: 'Download over Wi-Fi' });
    expect(checkbox).toHaveAccessibleDescription('Saves mobile data');
    await userEvent.click(checkbox);
    expect(onChange).toHaveBeenCalledWith(true);
    expect(checkbox).toBeChecked();
  });

  it('Checkbox exposes mixed state', () => {
    render(<Checkbox label="Select all" indeterminate />);
    expect(screen.getByRole('checkbox', { name: 'Select all' })).toHaveAttribute('aria-checked', 'mixed');
  });

  it('Radio preserves native grouping', async () => {
    render(
      <>
        <Radio name="quality" value="high" label="High quality" defaultChecked />
        <Radio name="quality" value="low" label="Low quality" />
      </>,
    );
    await userEvent.click(screen.getByRole('radio', { name: 'Low quality' }));
    expect(screen.getByRole('radio', { name: 'High quality' })).not.toBeChecked();
    expect(screen.getByRole('radio', { name: 'Low quality' })).toBeChecked();
  });

  it('Switch exposes switch semantics', async () => {
    const onChange = vi.fn();
    render(<Switch label="Autoplay" onChange={onChange} />);
    await userEvent.click(screen.getByRole('switch', { name: 'Autoplay' }));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('Progress supports both modes', () => {
    const { rerender } = render(<Progress label="Upload" value={25} />);
    expect(screen.getByRole('progressbar', { name: 'Upload' })).toHaveAttribute('aria-valuenow', '25');
    expect(screen.getByText('25%')).toBeInTheDocument();
    rerender(<Progress label="Processing" />);
    expect(screen.getByRole('progressbar', { name: 'Processing' })).not.toHaveAttribute('aria-valuenow');
  });

  it('Divider and TopNavItem expose navigation state', () => {
    render(
      <>
        <Divider label="Library" />
        <TopNavItem label="Discover" active />
      </>,
    );
    expect(screen.getByRole('separator', { name: 'Library' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Discover' })).toHaveAttribute('aria-current', 'page');
  });

  it('Input associates label, hint, error, and forwards ref', async () => {
    const ref = createRef<HTMLInputElement>();
    const { rerender } = render(<Input ref={ref} label="Playlist name" hint="Visible on your profile" size="lg" iconStart="search" />);
    const input = screen.getByRole('textbox', { name: 'Playlist name' });
    expect(ref.current).toBe(input);
    expect(input).toHaveAccessibleDescription('Visible on your profile');
    expect(input.closest('.ic-field')).toHaveClass('ic-field-lg');

    await userEvent.type(input, 'Night');
    expect(input).toHaveValue('Night');

    rerender(<Input ref={ref} label="Playlist name" error="Name is required" />);
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Name is required');
    expect(screen.getByRole('alert')).toHaveTextContent('Name is required');
    expect(input.closest('.ic-field')).toHaveClass('is-error');
  });

  it('Textarea tracks character counter and error state', async () => {
    const ref = createRef<HTMLTextAreaElement>();
    render(<Textarea ref={ref} label="Bio" maxLength={100} defaultValue="Hi" />);
    const area = screen.getByRole('textbox', { name: 'Bio' });
    expect(ref.current).toBe(area);
    expect(screen.getByText('2/100')).toBeInTheDocument();
    await userEvent.type(area, '!');
    expect(screen.getByText('3/100')).toBeInTheDocument();
  });
});
