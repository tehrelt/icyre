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

  test('truncates long track title and artist on separate lines in mobile mini-player without hiding play button', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 800 });
    await page.goto('/');

    await page.getByRole('button', { name: 'Play album' }).click();

    const player = page.getByRole('region', { name: 'Player' });
    const playBtn = player.getByRole('button', { name: 'Pause' });
    await expect(playBtn).toBeVisible();

    const metaBtn = player.getByRole('button', { name: /Open expanded player for/ });
    await expect(metaBtn).toBeVisible();

    const metrics = await page.evaluate(() => {
      const region = document.querySelector('[role="region"][aria-label="Player"]')!;
      const btn = region.querySelector('button[aria-label^="Open expanded player for"]') as HTMLButtonElement;
      const spans = btn.querySelectorAll('span');
      const titleEl = spans[0]!;
      const artistEl = spans[1]!;
      const pauseBtn = region.querySelector('button[aria-label="Pause"]') as HTMLButtonElement;

      titleEl.textContent = 'A Very Long Track Title That Must Truncate Cleanly On Narrow Mobile Viewports';
      artistEl.textContent = 'An Equally Long Artist Name That Must Stay On Its Own Line Without Pushing Controls';

      const btnRect = btn.getBoundingClientRect();
      const titleRect = titleEl.getBoundingClientRect();
      const artistRect = artistEl.getBoundingClientRect();
      const playRect = pauseBtn.getBoundingClientRect();
      const titleStyle = getComputedStyle(titleEl);
      const artistStyle = getComputedStyle(artistEl);

      return {
        titleTop: titleRect.top,
        titleBottom: titleRect.bottom,
        artistTop: artistRect.top,
        titleScrollW: titleEl.scrollWidth,
        titleClientW: titleEl.clientWidth,
        artistScrollW: artistEl.scrollWidth,
        artistClientW: artistEl.clientWidth,
        titleOverflow: titleStyle.textOverflow,
        artistOverflow: artistStyle.textOverflow,
        metaRight: btnRect.right,
        playLeft: playRect.left,
        playRight: playRect.right,
        playWidth: playRect.width,
        playHeight: playRect.height,
        viewportWidth: window.innerWidth,
        docOverflow: document.documentElement.scrollWidth - window.innerWidth,
      };
    });

    expect(metrics.artistTop).toBeGreaterThanOrEqual(metrics.titleBottom - 1);
    expect(metrics.titleScrollW).toBeGreaterThan(metrics.titleClientW);
    expect(metrics.artistScrollW).toBeGreaterThan(metrics.artistClientW);
    expect(metrics.titleOverflow).toBe('ellipsis');
    expect(metrics.artistOverflow).toBe('ellipsis');
    expect(metrics.metaRight).toBeLessThanOrEqual(metrics.playLeft + 1);
    expect(metrics.playWidth).toBeGreaterThanOrEqual(40);
    expect(metrics.playHeight).toBeGreaterThanOrEqual(40);
    expect(metrics.playRight).toBeLessThanOrEqual(metrics.viewportWidth);
    expect(metrics.docOverflow).toBeLessThanOrEqual(0);
  });
});
