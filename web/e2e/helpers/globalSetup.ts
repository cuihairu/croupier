import type { FullConfig } from '@playwright/test';
import { shouldStartRealFixture, startRealFixture } from './realFixture';

export default async function globalSetup(_config: FullConfig): Promise<void> {
  if (!shouldStartRealFixture()) {
    console.log('[real-dashboard fixture] skipped (no real-dashboard project selected)');
    return;
  }
  await startRealFixture();
}
