#!/bin/sh
set -e
# This script updates the Table of Contents in the README.md file.

# Check if markdown-toc is installed
if ! command -v markdown-toc >/dev/null 2>&1; then
  echo "markdown-toc not found. Installing via Homebrew..."
  brew install markdown-toc
fi

# Find the root of the Git workspace
WORKSPACE="$(git rev-parse --show-toplevel)"

# Update the README.md with a table of contents
markdown-toc -i "$WORKSPACE/README.md" --maxdepth 3