#!/bin/bash

set -euo pipefail


# Usage function
usage() {
    echo "Usage: ./scripts/release.sh <tag> [--dry-run]"
}

# Parse arguments
TAG=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            if [[ -z "$TAG" ]]; then
                TAG="$1"
            else
                echo "Error: Unknown argument: $1" >&2
                usage
                exit 1
            fi
            shift
            ;;
    esac
done

if [[ -z "$TAG" ]]; then
    echo "Error: Tag is required" >&2
    usage
    exit 1
fi

# Validate tag format (basic semver check)
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
    echo "Error: Tag must follow semantic versioning format (e.g., v1.2.3 or v1.2.3-beta.1)" >&2
    exit 1
fi

if ! git diff-index --quiet HEAD --; then
    echo "Error: Working directory is not clean." >&2
    git status --porcelain
    exit 1
fi

if git tag --list | grep -q "^$TAG$"; then
    echo "Error: Tag '$TAG' already exists locally" >&2
    exit 1
fi

# Check remote tags
git fetch --tags > /dev/null 2>&1 || true
if git ls-remote --tags origin | grep -q "refs/tags/$TAG$"; then
    echo "Error: Tag '$TAG' already exists on remote" >&2
    exit 1
fi

# Show release information
COMMIT=$(git rev-parse HEAD)
SHORT_COMMIT=$(git rev-parse --short HEAD)
REPO_URL=$(git config --get remote.origin.url | sed 's/git@github.com:/https:\/\/github.com\//' | sed 's/\.git$//')
LAST_TAG=$(git tag --sort=-version:refname | grep -v -- '-rc' | head -n 1 2>/dev/null || echo "")

echo "================================================"
echo " Go SDK Release"
echo "================================================"
printf "%-13s %s\n" "version:" "$TAG"
printf "%-13s %s\n" "commit:" "$SHORT_COMMIT"
printf "%-13s %s\n" "code:" "$REPO_URL/commit/$COMMIT"
if [[ -n "$LAST_TAG" ]]; then
    printf "%-13s %s\n" "changeset:" "$REPO_URL/compare/$LAST_TAG...$COMMIT"
else
    printf "%-13s %s\n" "changeset:" "$REPO_URL/commits/$COMMIT"
fi
echo ""

# Confirmation prompt (skip in dry-run)
if [[ "$DRY_RUN" == true ]]; then
    exit 0
fi

read -p "Are you ready to release version $TAG? Type 'YOLO' to continue: " -r
echo ""
if [[ "$REPLY" != "YOLO" ]]; then
    exit 0
fi

if ! make ci; then
    echo "Error: make ci failed" >&2
    exit 1
fi

git tag -a "$TAG" -m "Release $TAG"
git push origin "$TAG"

echo "Tag $TAG has been created and pushed to origin. Check GitHub Actions for build progress:"
echo "$REPO_URL/actions"
