#!/bin/bash

# Script to rerun the failed GitHub Actions job for kots PR #5517
# Job: build-kotsadm
# Run ID: 17330685346
# Job ID: 49206540460

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# GitHub repository and run details
REPO="replicatedhq/kots"
RUN_ID="17330685346"
JOB_ID="49206540460"
PR_NUMBER="5517"

echo -e "${GREEN}GitHub Actions Job Rerun Script${NC}"
echo "Repository: $REPO"
echo "PR: #$PR_NUMBER"
echo "Run ID: $RUN_ID"
echo "Job ID: $JOB_ID"
echo ""

# Function to check if gh CLI is installed
check_gh_cli() {
    if ! command -v gh &> /dev/null; then
        echo -e "${RED}GitHub CLI (gh) is not installed.${NC}"
        echo "Install it with: sudo apt install gh -y"
        exit 1
    fi
}

# Function to authenticate with GitHub
authenticate_github() {
    # Check if already authenticated
    if gh auth status &> /dev/null; then
        echo -e "${GREEN}✓ Already authenticated with GitHub${NC}"
        return 0
    fi

    echo -e "${YELLOW}GitHub authentication required. Choose a method:${NC}"
    echo "1) Use GitHub Personal Access Token (recommended)"
    echo "2) Authenticate via browser"
    echo "3) Use existing GITHUB_TOKEN environment variable"
    read -p "Enter choice (1-3): " auth_choice

    case $auth_choice in
        1)
            echo "Please enter your GitHub Personal Access Token:"
            echo "(Get one from: https://github.com/settings/tokens/new)"
            echo "Required scopes: repo, workflow"
            read -s -p "Token: " GITHUB_TOKEN
            echo ""
            echo "$GITHUB_TOKEN" | gh auth login --with-token
            ;;
        2)
            gh auth login
            ;;
        3)
            if [ -z "$GITHUB_TOKEN" ] && [ -z "$GH_TOKEN" ]; then
                echo -e "${RED}No GITHUB_TOKEN or GH_TOKEN found in environment${NC}"
                exit 1
            fi
            TOKEN="${GITHUB_TOKEN:-$GH_TOKEN}"
            echo "$TOKEN" | gh auth login --with-token
            ;;
        *)
            echo -e "${RED}Invalid choice${NC}"
            exit 1
            ;;
    esac

    # Verify authentication
    if gh auth status &> /dev/null; then
        echo -e "${GREEN}✓ Successfully authenticated${NC}"
    else
        echo -e "${RED}Authentication failed${NC}"
        exit 1
    fi
}

# Function to rerun the job
rerun_job() {
    echo -e "${YELLOW}Choose rerun option:${NC}"
    echo "1) Rerun only failed jobs (recommended)"
    echo "2) Rerun all jobs in the workflow"
    echo "3) Rerun specific job (build-kotsadm)"
    read -p "Enter choice (1-3): " rerun_choice

    case $rerun_choice in
        1)
            echo -e "${YELLOW}Rerunning failed jobs...${NC}"
            if gh run rerun $RUN_ID --repo $REPO --failed; then
                echo -e "${GREEN}✓ Successfully triggered rerun of failed jobs${NC}"
            else
                echo -e "${RED}Failed to rerun jobs${NC}"
                exit 1
            fi
            ;;
        2)
            echo -e "${YELLOW}Rerunning all jobs...${NC}"
            if gh run rerun $RUN_ID --repo $REPO; then
                echo -e "${GREEN}✓ Successfully triggered rerun of all jobs${NC}"
            else
                echo -e "${RED}Failed to rerun jobs${NC}"
                exit 1
            fi
            ;;
        3)
            echo -e "${YELLOW}Rerunning specific job (build-kotsadm)...${NC}"
            # Note: GitHub CLI doesn't support rerunning specific job directly
            # We'll use the API instead
            TOKEN=$(gh auth token)
            response=$(curl -s -X POST \
                -H "Accept: application/vnd.github+json" \
                -H "Authorization: Bearer $TOKEN" \
                -H "X-GitHub-Api-Version: 2022-11-28" \
                "https://api.github.com/repos/$REPO/actions/jobs/$JOB_ID/rerun")
            
            if [ $? -eq 0 ]; then
                echo -e "${GREEN}✓ Successfully triggered rerun of build-kotsadm job${NC}"
            else
                echo -e "${RED}Failed to rerun job. Response: $response${NC}"
                exit 1
            fi
            ;;
        *)
            echo -e "${RED}Invalid choice${NC}"
            exit 1
            ;;
    esac
}

# Function to watch the run status
watch_run() {
    echo ""
    read -p "Do you want to watch the run status? (y/n): " watch_choice
    if [[ "$watch_choice" == "y" || "$watch_choice" == "Y" ]]; then
        echo -e "${YELLOW}Opening workflow run in browser and watching status...${NC}"
        gh run view $RUN_ID --repo $REPO --web &
        gh run watch $RUN_ID --repo $REPO
    else
        echo ""
        echo -e "${GREEN}Job rerun initiated successfully!${NC}"
        echo ""
        echo "View the run at: https://github.com/$REPO/actions/runs/$RUN_ID"
        echo "Or watch with: gh run watch $RUN_ID --repo $REPO"
    fi
}

# Main execution
main() {
    echo "=========================================="
    echo ""
    
    # Step 1: Check for gh CLI
    check_gh_cli
    
    # Step 2: Authenticate
    authenticate_github
    
    # Step 3: Rerun the job
    rerun_job
    
    # Step 4: Optionally watch the run
    watch_run
    
    echo ""
    echo "=========================================="
}

# Alternative: Direct API call without gh CLI
api_rerun() {
    echo -e "${YELLOW}Using direct API call (no gh CLI required)${NC}"
    echo "Please enter your GitHub Personal Access Token:"
    read -s -p "Token: " GITHUB_TOKEN
    echo ""
    
    if [ -z "$GITHUB_TOKEN" ]; then
        echo -e "${RED}Token is required${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}Rerunning failed jobs via API...${NC}"
    response=$(curl -s -X POST \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        "https://api.github.com/repos/$REPO/actions/runs/$RUN_ID/rerun-failed-jobs")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Successfully triggered rerun via API${NC}"
        echo "View at: https://github.com/$REPO/actions/runs/$RUN_ID"
    else
        echo -e "${RED}Failed to rerun. Response: $response${NC}"
        exit 1
    fi
}

# Check if user wants to use API directly
if [ "$1" == "--api" ]; then
    api_rerun
else
    main
fi