#!/usr/bin/env bash
# Install tracked git hooks from .githooks/ into .git/hooks/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HOOKS_SRC="$ROOT/.githooks"
HOOKS_DST="$ROOT/.git/hooks"

if [ ! -d "$ROOT/.git" ]; then
  echo "error: not a git repository ($ROOT)" >&2
  exit 1
fi

mkdir -p "$HOOKS_DST"

for hook in "$HOOKS_SRC"/*; do
  [ -f "$hook" ] || continue
  name="$(basename "$hook")"
  install -m 755 "$hook" "$HOOKS_DST/$name"
  echo "installed $name"
done

echo "Git hooks installed. Run 'make install-dev-tools' if pre-commit reports missing commands."
