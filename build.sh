#!/bin/bash
set -e

export PATH="/opt/homebrew/bin:$PATH"

echo "● Building bcommit..."
go build -o bcommit ./cmd/bcommit

echo "● Installing to ~/.local/bin..."
cp bcommit ~/.local/bin/bcommit

echo "✓ Done. Run 'bcommit' from any repo."
