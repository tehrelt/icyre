import { expect, test } from '@playwright/test';

test.describe('ICYRE web e2e', () => {
  test('opens Home, plays album, and keeps playback across navigation', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('heading', { name: 'Salt Lanterns' })).toBeVisible();

    const player = page.getByRole('region', { name: 'Player' });
    await expect(player.getByText('Nothing playing')).toBeVisible();

    await page.getByRole('button', { name: 'Play album' }).click();

    await expect(player.getByText('Mira Solen')).toBeVisible();
    await expect(player.getByRole('button', { name: 'Pause' })).toBeVisible();

    await page.getByRole('link', { name: 'Search' }).first().click();
    await expect(page).toHaveURL(/\/search$/);
    await expect(page.getByRole('searchbox', { name: 'Search' })).toBeVisible();

    await expect(player.getByText('Mira Solen')).toBeVisible();
    await expect(player.getByRole('button', { name: 'Pause' })).toBeVisible();

    await page.getByRole('link', { name: 'Home' }).first().click();
    await expect(page).toHaveURL(/\/$/);
    await page.getByRole('region', { name: 'Salt Lanterns' }).getByRole('button', { name: /^Salt Lanterns/ }).click();
    await expect(page).toHaveURL(/\/album\/alb-salt-lanterns$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Salt Lanterns' })).toBeVisible();
    await expect(player.getByRole('button', { name: 'Pause' })).toBeVisible();
  });

  test('searches catalog and clears query', async ({ page }) => {
    await page.goto('/search');

    const input = page.getByRole('searchbox', { name: 'Search' });
    await input.fill('Salt');

    await expect(page).toHaveURL(/\/search\?q=Salt/);
    await expect(page.getByRole('heading', { name: 'Top result' })).toBeVisible();

    await page.getByRole('button', { name: 'Clear search' }).click();
    await expect(input).toHaveValue('');
  });

  for (const viewport of [
    { name: 'desktop', width: 1440, height: 900 },
    { name: 'tablet', width: 900, height: 800 },
    { name: 'mobile', width: 390, height: 844 },
  ]) {
    test(`has no horizontal overflow at ${viewport.name} (${viewport.width}px)`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });

      for (const path of ['/', '/album/alb-salt-lanterns', '/search']) {
        await page.goto(path);
        await page.waitForLoadState('networkidle');
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
        expect(overflow, `horizontal overflow on ${path} at ${viewport.width}px`).toBeLessThanOrEqual(0);
      }
    });
  }
});
