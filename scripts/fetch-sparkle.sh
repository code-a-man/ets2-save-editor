#!/bin/bash
set -euo pipefail

target="${1:?Set the Sparkle extraction directory.}"
mkdir -p "$target"
archive="$target/Sparkle-2.10.1.tar.xz"
curl -fsSL --retry 3 --max-time 90 -o "$archive" \
    https://github.com/sparkle-project/Sparkle/releases/download/2.10.1/Sparkle-2.10.1.tar.xz
printf '%s  %s\n' 1a856d2a9edb8e628de88121d7be04cf0e2167f8641650b78fca00b14244cf27 "$archive" | shasum -a 256 -c
tar -xf "$archive" -C "$target"
