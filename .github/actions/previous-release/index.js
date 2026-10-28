const { appendFileSync } = require('node:fs');

async function main() {
  const current = process.env.INPUT_VERSION?.split('+k8s-', 1)[0];
  const token = process.env.INPUT_TOKEN;
  const repository = process.env.GITHUB_REPOSITORY;
  if (!current || !token || !repository || !process.env.GITHUB_OUTPUT) {
    throw new Error('version, token, GITHUB_REPOSITORY, and GITHUB_OUTPUT are required');
  }

  const apiUrl = process.env.GITHUB_API_URL || 'https://api.github.com';
  // Releases are newest first. Fetch more pages only if needed.
  for (let page = 1; ; page++) {
    const response = await fetch(`${apiUrl}/repos/${repository}/releases?per_page=100&page=${page}`, {
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: 'application/vnd.github+json',
        'X-GitHub-Api-Version': '2022-11-28',
      },
    });
    if (!response.ok) {
      throw new Error(`Failed to read GitHub releases: HTTP ${response.status}`);
    }

    const releases = await response.json();
    for (const release of releases) {
      if (release.draft || release.prerelease) continue;
      const baseVersion = release.tag_name.split('+k8s-', 1)[0];
      if (baseVersion === current) continue;
      if (!baseVersion || /[\r\n]/.test(baseVersion)) {
        throw new Error('Invalid release version');
      }
      console.log(`Release notes base: ${baseVersion}`);
      appendFileSync(process.env.GITHUB_OUTPUT, `base-version=${baseVersion}\n`);
      return;
    }
    if (releases.length < 100) break;
  }
  throw new Error('No previous stable GitHub release found');
}

main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
