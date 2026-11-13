"""Verify callback ownership and map identity in the owner's private 1.9 NSO.

Emits checks only, never disassembly or game bytes. No game files may be
committed alongside this script or the generated game code.
"""
import argparse
import hashlib
import json
import sys
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("module", type=Path)
parser.add_argument("--runtime", type=Path, required=True)
args = parser.parse_args()
if hashlib.sha256(args.module.read_bytes()).hexdigest() != "f6f0b20bb1f2b67d164c24871cd473147eef1d285f12e7bd492e3126c6022e27":
    sys.exit("Unsupported private module")
sys.path.insert(0, str(args.runtime / "tools"))
from nso import Nso
from game_sites import Main, Site

m = Main(Nso(args.module))
assert m.word(0x7000cc4c3c + 0x20) == 0xf9400c14
assert m.word(0x70010a3f54) == 0xb9005a88
assert bytes(m.nso.image[0x1626a71:0x1626a78]) == b"HashId\0"
sites = json.loads((Path(__file__).parents[1] / "runtime/sites.json").read_text())
for s in sites:
    found, count = m.locate(Site(s["name"], s["kind"], s["pattern"], s["at"]))
    assert found == int(s["address"], 16) and count == 1, s["name"]
print("Verified: damage-component owner, map HashId storage and native signatures")
