# GitHub Actions Job Rerun Script

This script helps rerun the failed **build-kotsadm** job from kots PR #5517.

## Failed Job Details
- **Repository:** replicatedhq/kots
- **PR:** #5517 (upgrade jest)
- **Run ID:** 17330685346
- **Job ID:** 49206540460
- **Failed Step:** apko image build
- **Direct Link:** https://github.com/replicatedhq/kots/actions/runs/17330685346/job/49206540460

## Prerequisites

### Option 1: Using GitHub CLI (Recommended)
1. GitHub CLI must be installed (the script will check this)
2. GitHub Personal Access Token with `repo` and `workflow` scopes
   - Get one here: https://github.com/settings/tokens/new

### Option 2: Using Direct API
1. Only requires curl (pre-installed on most systems)
2. GitHub Personal Access Token with `repo` and `workflow` scopes

## Usage

### Standard Mode (with GitHub CLI)
```bash
./rerun-job.sh
```

The script will:
1. Check if GitHub CLI is installed
2. Prompt for authentication method
3. Ask which jobs to rerun (failed only, all, or specific)
4. Optionally watch the run status

### API Mode (without GitHub CLI)
```bash
./rerun-job.sh --api
```

This mode uses direct API calls and only requires a GitHub token.

## Authentication Options

When running the script, you can authenticate in three ways:

1. **Personal Access Token** (Recommended)
   - Create a token at: https://github.com/settings/tokens/new
   - Required scopes: `repo`, `workflow`
   - The script will prompt for the token

2. **Browser Authentication**
   - Opens your browser for OAuth flow
   - Requires interactive access

3. **Environment Variable**
   - Set `GITHUB_TOKEN` or `GH_TOKEN` before running
   ```bash
   export GITHUB_TOKEN="your_token_here"
   ./rerun-job.sh
   ```

## Rerun Options

The script offers three rerun strategies:

1. **Rerun Failed Jobs Only** (Recommended)
   - Only reruns jobs that failed in the workflow
   - Faster and more efficient

2. **Rerun All Jobs**
   - Reruns the entire workflow
   - Use if you suspect environmental issues

3. **Rerun Specific Job**
   - Attempts to rerun only the build-kotsadm job
   - Uses GitHub API directly

## Monitoring

After triggering the rerun, you can:
- Let the script watch the run status
- View in browser: https://github.com/replicatedhq/kots/actions/runs/17330685346
- Watch manually: `gh run watch 17330685346 --repo replicatedhq/kots`

## Troubleshooting

### "gh: command not found"
Install GitHub CLI:
```bash
sudo apt update && sudo apt install gh -y
```

### Authentication Failed
- Ensure your token has the required scopes
- Check token expiration
- Try regenerating the token

### API Error Response
- Check your token permissions
- Verify you have access to the repository
- Ensure the workflow/job IDs are correct

## Notes

- The original failure was in the apko image build step
- This appears to be a potentially transient failure
- If the job fails again with the same error, investigate the apko build configuration