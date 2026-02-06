#!/usr/bin/env bash
set -euo pipefail

MAIN_WORKTREE="$(git rev-parse --show-toplevel)"
WORKTREE_DIR="$(dirname "$MAIN_WORKTREE")/worktrees"
MAIN_ITERATR="$MAIN_WORKTREE/.iteratr"

usage() {
  echo "Usage: $0 [-b] <branch>"
  echo ""
  echo "Create a git worktree in ../worktrees/<branch> with .iteratr symlinked"
  echo "from the main worktree."
  echo ""
  echo "Options:"
  echo "  -b    Create a new branch"
  exit 1
}

if [ $# -lt 1 ]; then
  usage
fi

new_branch=false
while getopts "b" opt; do
  case $opt in
    b) new_branch=true ;;
    *) usage ;;
  esac
done
shift $((OPTIND - 1))

branch="${1:?branch name required}"
name="${branch##*/}"
target="$WORKTREE_DIR/$name"

mkdir -p "$WORKTREE_DIR"

if [ "$new_branch" = true ]; then
  git worktree add -b "$branch" "$target"
else
  git worktree add "$target" "$branch"
fi

if [ -d "$MAIN_ITERATR" ]; then
  ln -sfn "$MAIN_ITERATR" "$target/.iteratr"
  echo "Symlinked $target/.iteratr -> $MAIN_ITERATR"
else
  echo "Warning: $MAIN_ITERATR does not exist yet, skipping symlink"
fi

echo "Worktree created at $target"
