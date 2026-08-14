import type { FullConfig } from '@playwright/test';
import { shouldStartRealFixture, stopRealFixture } from './realFixture';

export default async function globalTeardown(_config: FullConfig): Promise<void> {
  if (!shouldStartRealFixture()) {
    return;
  }
  await stopRealFixture();
}
