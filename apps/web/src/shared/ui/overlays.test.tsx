import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';

import { Button } from './core';
import { QueueItem } from './media';
import { ContextMenu, Drawer, Dropdown, Menu, MenuItem, Modal, Popover, Toast, Tooltip } from './overlays';

describe('overlay components', () => {
  it('shows Tooltip on focus', async () => {
    render(
      <Tooltip content="Play next">
        <button type="button">Queue</button>
      </Tooltip>,
    );
    await userEvent.tab();
    expect(screen.getByRole('tooltip')).toHaveTextContent('Play next');
    await userEvent.tab();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('navigates Menu with arrows', async () => {
    render(
      <Menu autoFocus>
        <MenuItem label="Play next" />
        <MenuItem label="Remove" danger />
      </Menu>,
    );
    await waitFor(() => expect(screen.getByRole('menuitem', { name: 'Play next' })).toHaveFocus());
    await userEvent.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Remove' })).toHaveFocus();
  });

  it('opens and selects Dropdown items', async () => {
    const onSelect = vi.fn();
    render(
      <Dropdown trigger={<button type="button">More</button>}>
        <Menu>
          <MenuItem label="Add to queue" onSelect={onSelect} />
        </Menu>
      </Dropdown>,
    );
    await userEvent.click(screen.getByRole('button', { name: 'More' }));
    await userEvent.click(screen.getByRole('menuitem', { name: 'Add to queue' }));
    expect(onSelect).toHaveBeenCalledOnce();
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('opens ContextMenu at pointer', () => {
    render(
      <ContextMenu menu={<Menu><MenuItem label="Remove" /></Menu>}>
        <div>Track</div>
      </ContextMenu>,
    );
    fireEvent.contextMenu(screen.getByText('Track'), { clientX: 20, clientY: 30 });
    expect(screen.getByRole('menuitem', { name: 'Remove' })).toBeInTheDocument();
  });

  it('opens and closes Popover', async () => {
    render(
      <Popover trigger={<button type="button">Details</button>} title="Track details">
        Lossless audio
      </Popover>,
    );
    await userEvent.click(screen.getByRole('button', { name: 'Details' }));
    expect(screen.getByRole('dialog', { name: 'Track details' })).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('traps Modal lifecycle', async () => {
    const onClose = vi.fn();
    render(
      <Modal open onClose={onClose} title="Create playlist" footer={<Button>Create</Button>}>
        Playlist form
      </Modal>,
    );
    expect(screen.getByRole('dialog', { name: 'Create playlist' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus());
    await userEvent.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('renders Drawer through portal', () => {
    render(<Drawer open onClose={vi.fn()} title="Queue">Upcoming tracks</Drawer>);
    expect(screen.getByRole('dialog', { name: 'Queue' })).toHaveTextContent('Upcoming tracks');
  });

  it('runs Toast actions', async () => {
    const onAction = vi.fn();
    const onDismiss = vi.fn();
    render(<Toast title="Added to queue" actionLabel="Undo" onAction={onAction} onDismiss={onDismiss} />);
    await userEvent.click(screen.getByRole('button', { name: 'Undo' }));
    await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(onAction).toHaveBeenCalledOnce();
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it('renders current QueueItem state', async () => {
    const onPlay = vi.fn();
    const onRemove = vi.fn();
    render(
      <div role="list">
        <QueueItem title="Glass Tides" artist="Nova Hale" duration={227} state="current" onPlay={onPlay} onRemove={onRemove} />
      </div>,
    );
    expect(screen.getByRole('listitem')).toHaveAttribute('aria-current', 'true');
    expect(screen.getByRole('img', { name: 'Now playing' })).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Pause Glass Tides' }));
    await userEvent.click(screen.getByRole('button', { name: 'Remove Glass Tides from queue' }));
    expect(onPlay).toHaveBeenCalledOnce();
    expect(onRemove).toHaveBeenCalledOnce();
  });

  it('restores focus after Modal closes', async () => {
    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>Open modal</button>
          <Modal open={open} onClose={() => setOpen(false)} title="Details">Content</Modal>
        </>
      );
    }
    render(<Harness />);
    const trigger = screen.getByRole('button', { name: 'Open modal' });
    await userEvent.click(trigger);
    await userEvent.keyboard('{Escape}');
    await waitFor(() => expect(trigger).toHaveFocus());
  });
});
