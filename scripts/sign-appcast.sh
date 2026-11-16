#!/bin/bash
set -euo pipefail
: "${SPARKLE_SIGNING_KEY:?Missing Sparkle signing key.}"
: "${SPARKLE_DIR:?Missing Sparkle tools.}"
: "${RELEASE_TAG:?Missing release tag.}"
: "${RELEASE_REPO:?Missing release repository.}"

asset=ETS2-Save-Editor-macOS-universal.zip
archive="dist/$asset"
sign_update="$SPARKLE_DIR/bin/sign_update"
signature="$(printf '%s' "$SPARKLE_SIGNING_KEY" | "$sign_update" --ed-key-file - -p "$archive")"
[[ "$signature" =~ ^[a-zA-Z0-9+/]{86}==$ ]]
size="$(stat -f '%z' "$archive")"

description=""
notes_file=".github/release-notes/$RELEASE_TAG.md"
if [[ -f "$notes_file" ]]; then
    description="$(python3 - "$notes_file" <<'PYTHON'
from html import escape
from pathlib import Path
import sys

notes = Path(sys.argv[1]).read_text(encoding="utf-8")
items = [line[2:].strip() for line in notes.splitlines() if line.startswith("- ")]
if not items:
    sys.exit("Release notes must contain at least one bullet.")
body = "".join(f"<li>{escape(item)}</li>" for item in items)
print(f"      <description><![CDATA[<ul>{body}</ul>]]></description>")
PYTHON
)"
fi

cat > dist/appcast.xml <<XML
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>ETS2 Save Editor</title>
    <item>
      <title>ETS2 Save Editor ${RELEASE_TAG#v}</title>
      <sparkle:version>${RELEASE_TAG#v}</sparkle:version>
      <sparkle:shortVersionString>${RELEASE_TAG#v}</sparkle:shortVersionString>
$description
      <sparkle:minimumSystemVersion>13.0</sparkle:minimumSystemVersion>
      <enclosure url="https://github.com/$RELEASE_REPO/releases/download/$RELEASE_TAG/$asset"
                 sparkle:edSignature="$signature" length="$size" type="application/octet-stream" />
    </item>
  </channel>
</rss>
XML

printf '%s' "$SPARKLE_SIGNING_KEY" | "$sign_update" --ed-key-file - dist/appcast.xml
printf '%s' "$SPARKLE_SIGNING_KEY" | "$sign_update" --ed-key-file - --verify dist/appcast.xml
printf '%s' "$SPARKLE_SIGNING_KEY" | "$sign_update" --ed-key-file - --verify "$archive" "$signature"
