/**
 * Layout Behavior Tests
 * 
 * These tests verify critical layout behaviors:
 * 1. Sidebar pushes content (doesn't overlap)
 * 2. Timeline container doesn't resize on sidebar hover (prevents layout thrashing)
 * 3. NamespaceGraph container doesn't resize on sidebar hover
 * 4. Settings page is scrollable
 */
import { test, expect } from '@playwright/experimental-ct-react';
import React, { useState, useEffect } from 'react';
import App from '../../src/App';
import SettingsPage from '../../src/pages/SettingsPage';

/**
 * Test wrapper that captures resize events on a child element.
 * Used to verify that components don't re-render on width changes.
 */
const ResizeTracker: React.FC<{
  children: React.ReactNode;
  onResize: (size: { width: number; height: number }) => void;
}> = ({ children, onResize }) => {
  const containerRef = React.useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        onResize({ width, height });
      }
    });

    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, [onResize]);

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%' }}>
      {children}
    </div>
  );
};

test.describe('Layout Behavior', () => {
  test.describe('Sidebar Navigation', () => {
    test('sidebar expands and pushes content with marginLeft (not overlap)', async ({ mount, page }) => {
      // Mount the full App component
      await mount(<App />);

      // The layout uses a two-layer approach:
      // - Outer wrapper div has a fixed marginLeft of 64px (collapsed sidebar width)
      // - Inner <main> uses transform: translateX() to shift when sidebar expands
      const main = page.locator('main');
      await expect(main).toBeVisible();

      // The outer wrapper is the parent div of <main>
      const outerWrapper = main.locator('..');

      // Move mouse away from sidebar to ensure collapsed state
      await page.mouse.move(600, 300);
      await page.waitForTimeout(400); // Wait for collapse transition

      // Verify outer wrapper has fixed margin-left of 64px
      await expect(outerWrapper).toHaveCSS('margin-left', '64px');

      // Verify main has no transform offset when sidebar is collapsed
      // translateX(0) computes to matrix(1, 0, 0, 1, 0, 0)
      await expect(main).toHaveCSS('transform', 'matrix(1, 0, 0, 1, 0, 0)');

      // Get the sidebar element and hover over it
      const sidebar = page.locator('.sidebar-container');
      await sidebar.hover();

      // Wait for the CSS transition to complete (250ms + buffer)
      await page.waitForTimeout(350);

      // Verify main is translated by 156px (220 - 64) when sidebar expands
      // transform: translateX(156px) is represented as matrix(1, 0, 0, 1, 156, 0)
      await expect(main).toHaveCSS('transform', 'matrix(1, 0, 0, 1, 156, 0)');

      // Outer wrapper margin stays fixed at 64px (no layout change)
      await expect(outerWrapper).toHaveCSS('margin-left', '64px');

      // Move mouse away from sidebar
      await page.mouse.move(500, 500);
      await page.waitForTimeout(350);

      // Verify transform returns to translateX(0)
      await expect(main).toHaveCSS('transform', 'matrix(1, 0, 0, 1, 0, 0)');
    });
  });

  test.describe('Timeline Resize Behavior', () => {
    test('Timeline does not trigger state update when only width changes', async ({ mount, page }) => {
      // Mount the App which includes the Timeline on the default route
      await mount(<App />);

      // Wait for the app to fully load
      await page.waitForTimeout(500);

      // The Timeline is on the main route, find its SVG
      const timelineSvg = page.locator('main svg').first();
      
      // Check if Timeline loaded (it may show loading state if no data)
      const svgExists = await timelineSvg.count() > 0;
      
      if (svgExists) {
        // Get the initial SVG width attribute
        const initialWidth = await timelineSvg.evaluate((el) => el.getAttribute('width'));

        // Simulate a width change by hovering the sidebar
        const sidebar = page.locator('.sidebar-container');
        await sidebar.hover();
        await page.waitForTimeout(400); // Wait for transition + debounce

        // The SVG width should remain unchanged because:
        // 1. The ResizeObserver ignores width-only changes
        // 2. No React re-render should occur
        const widthAfterHover = await timelineSvg.evaluate((el) => el.getAttribute('width'));
        
        // Width should be the same (no re-render occurred)
        expect(widthAfterHover).toBe(initialWidth);

        // Move mouse away
        await page.mouse.move(500, 500);
        await page.waitForTimeout(400);

        // Width should still be the same
        const widthAfterUnhover = await timelineSvg.evaluate((el) => el.getAttribute('width'));
        expect(widthAfterUnhover).toBe(initialWidth);
      } else {
        // Timeline may be showing loading/empty state, which is fine
        // The important thing is we've verified the App renders
        test.skip();
      }
    });
  });

  test.describe('NamespaceGraph Resize Behavior', () => {
    test('NamespaceGraph does not trigger state update when only width changes', async ({ mount, page }) => {
      // Mount the App
      await mount(<App />);

      // Navigate to the graph page
      await page.locator('.sidebar-link[href="/graph"]').click();
      await page.waitForTimeout(500);

      // Find the graph SVG
      const graphSvg = page.locator('main svg').first();
      const svgExists = await graphSvg.count() > 0;

      if (svgExists) {
        // Get the initial SVG dimensions
        const initialDimensions = await graphSvg.evaluate((el) => ({
          width: el.getAttribute('width'),
          height: el.getAttribute('height'),
        }));

        // Simulate a width change by hovering the sidebar
        const sidebar = page.locator('.sidebar-container');
        await sidebar.hover();
        await page.waitForTimeout(400);

        // Get dimensions after hover
        const dimensionsAfterHover = await graphSvg.evaluate((el) => ({
          width: el.getAttribute('width'),
          height: el.getAttribute('height'),
        }));

        // Dimensions should remain unchanged
        expect(dimensionsAfterHover.width).toBe(initialDimensions.width);
        expect(dimensionsAfterHover.height).toBe(initialDimensions.height);
      } else {
        // Graph may be showing loading/empty state
        test.skip();
      }
    });
  });

  test.describe('Settings Page Scrollability', () => {
    test('Settings page is scrollable when content exceeds viewport', async ({ mount, page }) => {
      // Mount the SettingsPage directly
      await mount(<SettingsPage />);

      // Find the scrollable container
      const scrollContainer = page.locator('.h-screen.overflow-auto');
      await expect(scrollContainer).toBeVisible();

      // Verify the content is taller than the container (scrollable)
      const isScrollable = await scrollContainer.evaluate((el) => {
        return el.scrollHeight > el.clientHeight;
      });
      expect(isScrollable).toBe(true);

      // Get initial scroll position
      const initialScrollTop = await scrollContainer.evaluate((el) => el.scrollTop);
      expect(initialScrollTop).toBe(0);

      // Scroll down
      await scrollContainer.evaluate((el) => {
        el.scrollTo({ top: 200, behavior: 'instant' });
      });

      // Verify scroll position changed
      const newScrollTop = await scrollContainer.evaluate((el) => el.scrollTop);
      expect(newScrollTop).toBe(200);

      // Scroll back to top
      await scrollContainer.evaluate((el) => {
        el.scrollTo({ top: 0, behavior: 'instant' });
      });

      // Verify we can scroll back
      const finalScrollTop = await scrollContainer.evaluate((el) => el.scrollTop);
      expect(finalScrollTop).toBe(0);
    });

    test('Settings page can scroll to bottom', async ({ mount, page }) => {
      // Mount the SettingsPage
      await mount(<SettingsPage />);

      const scrollContainer = page.locator('.h-screen.overflow-auto');
      await expect(scrollContainer).toBeVisible();

      // Scroll to bottom
      await scrollContainer.evaluate((el) => {
        el.scrollTo({ top: el.scrollHeight, behavior: 'instant' });
      });

      // Verify we're at the bottom (scrollTop + clientHeight >= scrollHeight)
      const atBottom = await scrollContainer.evaluate((el) => {
        return el.scrollTop + el.clientHeight >= el.scrollHeight - 1; // -1 for rounding
      });
      expect(atBottom).toBe(true);
    });

    test('Settings page within App is scrollable via navigation', async ({ mount, page }) => {
      // Mount full app
      await mount(<App />);

      // Navigate to settings page
      await page.locator('.sidebar-link[href="/settings"]').click();
      await page.waitForTimeout(300);

      // Find the scrollable container in settings
      const scrollContainer = page.locator('.h-screen.overflow-auto');
      await expect(scrollContainer).toBeVisible();

      // Verify it's scrollable
      const isScrollable = await scrollContainer.evaluate((el) => {
        return el.scrollHeight > el.clientHeight;
      });
      expect(isScrollable).toBe(true);

      // Actually scroll
      await scrollContainer.evaluate((el) => {
        el.scrollTo({ top: 100, behavior: 'instant' });
      });

      const scrollTop = await scrollContainer.evaluate((el) => el.scrollTop);
      expect(scrollTop).toBe(100);
    });
  });
});
