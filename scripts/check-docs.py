"""Check repository Markdown fences and relative links without executing examples."""
from pathlib import Path
import re
from urllib.parse import unquote, urlparse

root = Path(__file__).resolve().parents[1]
files = [root / "README.md", *sorted((root / "docs").rglob("*.md"))]
links = 0
for file in files:
    source = file.read_text(encoding="utf-8-sig")
    if len(re.findall(r"^```", source, re.M)) % 2:
        raise SystemExit(f"Unclosed code fence: {file.relative_to(root)}")
    prose = re.sub(r"^```[^\n]*\n.*?^```[^\n]*$", "", source, flags=re.M | re.S)
    for destination in re.findall(r"\[[^\]]*\]\(([^)]+)\)", prose):
        destination = destination.split("#", 1)[0]
        if not destination or urlparse(destination).scheme:
            continue
        if not (file.parent / unquote(destination)).exists():
            raise SystemExit(f"Broken link: {file.relative_to(root)} -> {destination}")
        links += 1
print(f"Checked {len(files)} Markdown documents and {links} relative links")
