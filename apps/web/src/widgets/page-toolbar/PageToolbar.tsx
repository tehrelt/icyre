import { useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate, useNavigationType } from 'react-router';

import { useCurrentUser } from '@/entities/user/api';
import { useLogoutMutation } from '@/features/auth';
import { routes } from '@/shared/config/routes';
import { Avatar, Breadcrumb, Button, Dropdown, IconButton, Menu, MenuDivider, MenuItem } from '@/shared/ui';

import styles from './PageToolbar.module.css';

export interface Crumb {
  label: string;
  to?: string;
}

/** Tracks the router history index so back/forward reflect what is possible. */
function useHistoryStack() {
  const location = useLocation();
  const navType = useNavigationType();
  const maxIdx = useRef(0);
  const [state, setState] = useState({ canBack: false, canForward: false });

  useEffect(() => {
    const idx = (window.history.state as { idx?: number } | null)?.idx ?? 0;
    if (navType === 'PUSH') maxIdx.current = idx;
    else maxIdx.current = Math.max(maxIdx.current, idx);
    setState({ canBack: idx > 0, canForward: idx < maxIdx.current });
  }, [location.key, navType]);

  return state;
}

/** Shell / Page toolbar from the product canvas (CToolbar). */
export function PageToolbar({ crumbs }: { crumbs?: Crumb[] }) {
  const location = useLocation();
  const navigate = useNavigate();
  const { canBack, canForward } = useHistoryStack();
  const user = useCurrentUser();
  const logout = useLogoutMutation();

  const handleSignOut = async () => {
    try {
      await logout.mutateAsync();
    } catch {
      // Local session is cleared in onSettled even if the request fails.
    }
    navigate(routes.login);
  };

  return (
    <header className={styles.toolbar}>
      <div className={styles.history}>
        <span className={styles.flip}>
          <IconButton icon="chevron" label="Back" size="sm" disabled={!canBack} onClick={() => navigate(-1)} />
        </span>
        <IconButton icon="chevron" label="Forward" size="sm" disabled={!canForward} onClick={() => navigate(1)} />
      </div>
      {crumbs && crumbs.length > 0 && <Breadcrumb items={crumbs} />}
      <div className={styles.spacer} />
      <span className={styles.whatsNew}>
        <Button variant="ghost" size="sm" iconStart="spark" onClick={() => navigate(routes.whatsNew)}>
          What&apos;s new
        </Button>
      </span>
      {user.data ? (
        <Dropdown
          align="end"
          trigger={
            <button type="button" aria-label={`Account — ${user.data.displayName}`} className={styles.profile}>
              <Avatar name={user.data.displayName} src={user.data.avatarUrl} size="sm" art={user.data.art} />
            </button>
          }
        >
          <Menu label="Account" autoFocus>
            <MenuItem label="Profile" icon="user" onSelect={() => navigate(routes.profile)} />
            <MenuItem label="Settings" icon="settings" onSelect={() => navigate(routes.settings)} />
            <MenuDivider />
            <MenuItem label="Sign out" danger disabled={logout.isPending} onSelect={() => void handleSignOut()} />
          </Menu>
        </Dropdown>
      ) : !user.isPending ? (
        <Button
          variant="primary"
          size="sm"
          onClick={() => navigate(routes.login, { state: { from: `${location.pathname}${location.search}` } })}
        >
          Sign in
        </Button>
      ) : null}
    </header>
  );
}
