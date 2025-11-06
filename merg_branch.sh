#!/bin/bash
# A safe script to merge the current branch into main.

set -e  # Exit immediately if a command exits with a non-zero status.

# Get the current branch name
current_branch=$(git rev-parse --abbrev-ref HEAD)

if [ "$current_branch" = "main" ]; then
  echo "You're already on the main branch. Nothing to merge."
  exit 1
fi

echo "Merging branch '$current_branch' into 'main'..."

# Ensure working directory is clean
if ! git diff-index --quiet HEAD --; then
  echo "You have uncommitted changes. Please commit or stash them first."
  exit 1
fi

# Fetch latest changes
git fetch origin

# Switch to main and update
git checkout main
git pull origin main

# Merge current branch into main
git merge --no-ff "$current_branch" -m "Merge branch '$current_branch' into main"

# Push updated main to remote
git push origin main

# Go back to original branch
git checkout "$current_branch"

echo "✅ Successfully merged '$current_branch' into 'main' and pushed to origin."