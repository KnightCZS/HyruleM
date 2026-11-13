"""Apply HyruleM's original adapter to the pinned Savage Wind source tree.

No game files are needed or emitted. The modified generator resolves signatures
against each player's own module when rebuilding their private game library.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil

UPSTREAM = "740b18025fe11ba03c117dbb56668db304b5043e"
HERE = Path(__file__).resolve().parent


def replace_once(path, old, new):
    text = path.read_text()
    if new in text:
        return
    if text.count(old) != 1:
        raise ValueError(f"Unexpected upstream source: {path.name}")
    path.write_text(text.replace(old, new, 1))


def apply(root):
    if hashlib.sha256((root / "Makefile").read_bytes().replace(b"\r\n", b"\n")).hexdigest() != "73dbd3c376e8259347dc1f423eb7795e705a8494eec5074bd0fd8e63502074a8":
        raise ValueError("Use the exact pinned upstream commit " + UPSTREAM)
    sites = json.loads((HERE / "runtime/sites.json").read_text())
    module = root / "tools/game_sites.py"
    code = module.read_text()
    marker = "# HyruleM native preview sites"
    entries = [f"    Site({s['name']!r}, {s['kind']!r}, {s['pattern']!r}, {s['at']})," for s in sites]
    group = repr(tuple(s["name"] for s in sites))
    addition = marker + "\nSITES += [\n" + "\n".join(entries) + "\n]\nOPTIONAL_GROUPS.append(" + group + ")\n"
    # No KNOWN fallback for this optional group. Each signature must match.
    if marker in code:
        start=code.index(marker)
        end=code.index("OPTIONAL = {n for g in OPTIONAL_GROUPS for n in g}",start)
        code=code[:start]+addition+"\n"+code[end:]
    else:
        code = code.replace("OPTIONAL = {n for g in OPTIONAL_GROUPS for n in g}", addition + "\nOPTIONAL = {n for g in OPTIONAL_GROUPS for n in g}")
        if marker not in code:
            raise ValueError("Unsupported game_sites.py")
    module.write_text(code)
    header = root / "runtime/game_sites.h"
    h = header.read_text()
    entries = " \\\n".join("    X(" + s["name"] + ")" for s in sites)
    if "X(hm_create)" in h:
        start=h.index("    X(hm_create)");end=h.index("\n\n",start)
        h=h[:start]+entries+h[end:]
    else:
        h = h.replace("    X(actor_info)\n", "    X(actor_info) \\\n" + entries + "\n")
        if "X(hm_create)" not in h:
            raise ValueError("Unsupported game_sites.h")
    header.write_text(h)
    replace_once(module,
        "    return name, resolve(Main(nso, base), KNOWN.get(name) if v and v.supported else None, log)",
        "    sites = resolve(Main(nso, base), KNOWN.get(name) if v and v.supported else None, log)\n"
        "    # The adapter uses layouts verified only for this exact main build.\n"
        "    if nso.build_id != 'cd57b23fa4bbad65803d9788c01821ee':\n"
        "        for key in tuple(sites):\n"
        "            if key.startswith('hm_'):\n"
        "                sites.pop(key)\n"
        "    return name, sites")
    patch = root / "tools/patches.py"
    replace_once(patch,
        "return ('{ extern void rt_item_tick(Cpu *, uint64_t, uint64_t, uint64_t); '",
        "return ('{ extern void rt_hyrulem_tick(Cpu *, uint64_t); '\n            'extern void rt_item_tick(Cpu *, uint64_t, uint64_t, uint64_t); '")
    replace_once(patch,
        'f\'rt_item_tick(c, {get:#x}ull, {gen.sites["pouch_mgr"]:#x}ull, LD64({got:#x}ull) + 0x10); c->sp -= 0x70; }}\'',
        'f\'rt_hyrulem_tick(c, LD64({got:#x}ull) + 0x10); rt_item_tick(c, {get:#x}ull, {gen.sites["pouch_mgr"]:#x}ull, LD64({got:#x}ull) + 0x10); c->sp -= 0x70; }}\'')
    replace_once(root / "runtime/control_game.c", "void control_game_init(void) {",
        "void control_hyrulem_init(void);\nvoid control_game_init(void) {\n    control_hyrulem_init();")
    shutil.copyfile(HERE / "runtime/hyrulem.c", root / "runtime/hyrulem.c")
    stamp = {"upstream": UPSTREAM, "adapter_sha256": hashlib.sha256((HERE / "runtime/hyrulem.c").read_bytes()).hexdigest()}
    (root / "hyrulem-build.json").write_text(json.dumps(stamp, indent=2) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    apply(args.source.resolve())
    print("HyruleM native preview adapter applied; rebuild app AND generator helper.")
