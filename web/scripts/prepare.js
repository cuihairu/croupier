const fs = require('fs');
const { execSync } = require('child_process');
const path = require('path');

const gitDir = path.join(process.cwd(), '.git');
if (!fs.existsSync(gitDir)) {
  // eslint-disable-next-line no-console
  console.log('husky install skipped (no git metadata)');
  process.exit(0);
}

try {
  // Use local husky binary to avoid shell-specific syntax.
  const huskyBin = path.join(
    process.cwd(),
    'node_modules',
    '.bin',
    process.platform === 'win32' ? 'husky.cmd' : 'husky',
  );
  execSync(`${huskyBin} install`, { stdio: 'inherit' });
} catch (err) {
  // eslint-disable-next-line no-console
  console.warn('husky install failed, continuing without hooks');
}
