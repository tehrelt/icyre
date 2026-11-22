import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { mockAccounts, mockAuth } from '@/shared/api/mock/server';
import { getAccessToken, resetSessionForTests } from '@/shared/api/session';
import { renderApp } from '@/test/render';

beforeEach(() => {
  resetSessionForTests();
  mockAuth.signedIn = false;
  mockAuth.email = 'rin@example.com';
  mockAuth.tokens.clear();
  for (const key of Array.from(mockAccounts.keys())) {
    if (key !== 'rin@example.com') mockAccounts.delete(key);
  }
});

afterEach(() => {
  resetSessionForTests();
  mockAuth.signedIn = true;
  mockAuth.email = 'rin@example.com';
  mockAuth.tokens.clear();
});

describe('Auth screens', () => {
  it('validates login fields with Zod before submitting', async () => {
    renderApp('/login');
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome back' })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText('Enter your email')).toBeInTheDocument();
    expect(screen.getByText('Enter your password')).toBeInTheDocument();
  });

  it('toggles password visibility', async () => {
    renderApp('/login');
    const passwordInput = await screen.findByLabelText('Password');
    expect(passwordInput).toHaveAttribute('type', 'password');

    await userEvent.click(screen.getByRole('button', { name: 'Show password' }));
    expect(passwordInput).toHaveAttribute('type', 'text');
    expect(screen.getByRole('button', { name: 'Hide password' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('shows an error banner on invalid credentials', async () => {
    renderApp('/login');
    await userEvent.type(await screen.findByLabelText('Email address'), 'rin@example.com');
    await userEvent.type(screen.getByLabelText('Password'), 'wrong-password');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Email or password is incorrect.');
    expect(getAccessToken()).toBeNull();
  });

  it('signs in and redirects to Home with active session', async () => {
    const { router } = renderApp('/login');
    await userEvent.type(await screen.findByLabelText('Email address'), 'rin@example.com');
    await userEvent.type(screen.getByLabelText('Password'), 'password1234');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(getAccessToken()).toBeTruthy();
    expect(await screen.findByRole('button', { name: 'Account — Rin Aoki' })).toBeInTheDocument();
  });

  it('switches between Sign in and Create account modes', async () => {
    const { router } = renderApp('/login');
    await userEvent.click(await screen.findByRole('radio', { name: 'Create account' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/register'));
    expect(screen.getByRole('heading', { level: 1, name: 'Create your account' })).toBeInTheDocument();
  });

  it('validates registration password policy, confirmation, and terms', async () => {
    renderApp('/register');
    await userEvent.type(await screen.findByLabelText('Email address'), 'new@example.com');
    await userEvent.type(screen.getByLabelText('Password'), 'short');
    expect(screen.getByText('Weak')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Confirm password'), 'different');
    await userEvent.click(screen.getByRole('checkbox', { name: 'I agree to the Terms and Privacy Policy' }));
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }));

    expect(await screen.findByText('Use at least 10 characters')).toBeInTheDocument();
    expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
    expect(screen.getByText('Accept the Terms and Privacy Policy to create an account.')).toBeInTheDocument();
  });

  it('shows field error when email is already taken', async () => {
    renderApp('/register');
    await userEvent.type(await screen.findByLabelText('Email address'), 'rin@example.com');
    await userEvent.type(screen.getByLabelText('Password'), 'longenoughpass');
    await userEvent.type(screen.getByLabelText('Confirm password'), 'longenoughpass');
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }));

    expect(await screen.findByText('An account with this email already exists.')).toBeInTheDocument();
  });

  it('registers a new account and redirects to Home', async () => {
    const { router } = renderApp('/register');
    await userEvent.type(await screen.findByLabelText('Email address'), 'mira@example.com');
    await userEvent.type(screen.getByLabelText('Password'), 'StrongPass123!');
    expect(screen.getByText('Strong')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Confirm password'), 'StrongPass123!');
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    expect(getAccessToken()).toBeTruthy();
    expect(await screen.findByRole('button', { name: 'Account — mira' })).toBeInTheDocument();
  });

  it('toggles the featured showcase preview on the auth stage', async () => {
    renderApp('/login');
    const playBtn = await screen.findByRole('button', { name: 'Play Prism Hours' });
    await userEvent.click(playBtn);
    expect(screen.getByRole('button', { name: 'Pause Prism Hours' })).toBeInTheDocument();
    expect(screen.getAllByRole('img', { name: 'Now playing' }).length).toBeGreaterThanOrEqual(1);
  });

  it('signs out from account menu and redirects to /login', async () => {
    mockAuth.signedIn = true;
    const { router } = renderApp('/');

    const accountBtn = await screen.findByRole('button', { name: 'Account — Rin Aoki' });
    await userEvent.click(accountBtn);
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Sign out' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome back' })).toBeInTheDocument();
    expect(getAccessToken()).toBeNull();
  });

  it('shows Sign in in toolbar when anonymous and navigates to /login', async () => {
    const { router } = renderApp('/');

    const signInBtn = await screen.findByRole('button', { name: 'Sign in' });
    await userEvent.click(signInBtn);
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
  });
});
