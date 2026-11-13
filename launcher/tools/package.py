"""Package owned launcher/server code; never include a game dump or generated DLL."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import zipfile

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT.parent
EXPECTED = "667c4909a8ed72c002e5aa40a00d38720627eebf48b860beb5cd8805421f1285"
parser = argparse.ArgumentParser()
parser.add_argument("--runtime", type=Path)
parser.add_argument("--out", type=Path, required=True)
parser.add_argument("--update-only", action="store_true")
args = parser.parse_args()
version = re.search(r'const Version = "([^"]+)"', (ROOT/"internal/launcher/config.go").read_text()).group(1)
binaries = ["HyruleLauncher.exe", "HyruleM-Server.exe"]
for name in binaries:
    if not (ROOT/"dist"/name).is_file():
        raise SystemExit(f"Build dist/{name} first.")
if not args.update_only:
    if not args.runtime or hashlib.sha256(args.runtime.read_bytes()).hexdigest() != EXPECTED:
        raise SystemExit("Provide the verified official Savage Wind ZIP with --runtime, or use --update-only.")
prefix = f"HyruleM_{version}_Windows_x64/"
checksums = {}
args.out.parent.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(args.out, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=7) as z:
    def add(path, relative, stored=False):
        z.write(path, prefix+relative, compress_type=zipfile.ZIP_STORED if stored else zipfile.ZIP_DEFLATED)
        checksums[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
    for name in binaries:
        add(ROOT/"dist"/name, name)
    add(REPO/"README.md", "README.md")
    add(REPO/"CHANGELOG.md", "CHANGELOG.md")
    for note in sorted((REPO/"releases").glob("*.md")):
        add(note, "releases/"+note.name)
    for name in ["LEIA_PRIMEIRO.txt", "LICENSE", "GO_LICENSE.txt", "THIRD_PARTY_NOTICES.md", "SANDBOX.md", "FILTRO_CAMPANHA.md"]:
        add(ROOT/name, name)
    for path in sorted((REPO/"docs").glob("*.md")):
        add(path, "docs/"+path.name)
    if not args.update_only:
        add(args.runtime, "runtime/SavageWind-0.5.3-alpha.5-windows.zip", stored=True)
    z.writestr(prefix+"release.json", json.dumps({"version": version, "platform": "windows-amd64", "runtime": "Savage Wind v0.5.3-alpha.5", "game_included": False, "sources_included": True, "runtime_included": not args.update_only, "multiplayer": "experimental-external-procedural-avatars", "native_game_actor": False, "real_game_validation": "pending"}, indent=2))
    for path in sorted(REPO.rglob("*")):
        if not path.is_file():
            continue
        relative = path.relative_to(REPO)
        if any(part in {"dist", ".git", "__pycache__", "extracted", "romfs", "exefs", "hyrulem", "save", "save-profiles"} for part in relative.parts):
            continue
        if path.suffix.lower() in {".sav", ".zip", ".dll", ".exe", ".nsp", ".xci", ".bfttf", ".log"}:
            continue
        add(path, "fontes/HyruleM/"+relative.as_posix())
    z.writestr(prefix+"SHA256SUMS.txt", "".join(f"{value}  {name}\n" for name,value in sorted(checksums.items())))
print(json.dumps({"file": str(args.out), "bytes": args.out.stat().st_size, "sha256": hashlib.sha256(args.out.read_bytes()).hexdigest()}, indent=2))
