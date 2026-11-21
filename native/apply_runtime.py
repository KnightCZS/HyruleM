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
    # Remove the previous worker hook when updating an already-patched tree.
    # Cross-actor work belongs after the engine's job wait, not in PlayerDamage.
    text=patch.read_text().replace("return ('{ extern void rt_hyrulem_tick(Cpu *, uint64_t); '\n            'extern void rt_item_tick(Cpu *, uint64_t, uint64_t, uint64_t); '",
        "return ('{ extern void rt_item_tick(Cpu *, uint64_t, uint64_t, uint64_t); '")
    text=text.replace('rt_hyrulem_tick(c, LD64({got:#x}ull) + 0x10); rt_item_tick(c,', 'rt_item_tick(c,')
    patch.write_text(text)
    replace_once(root / "runtime/control_game.c", "void control_game_init(void) {",
        "void control_hyrulem_init(void);\nvoid control_game_init(void) {\n    control_hyrulem_init();")
    # Canonicalize our section when upgrading a previously patched source tree.
    # Both callbacks must be inserted together for repeated applies to be stable.
    text=patch.read_text()
    marker="# HyruleM native game hooks"
    start=text.find(marker)
    if start<0:start=text.find("def _hm_capture_as(")
    if start>=0:
        end=text.index("def table(sites):",start)
        text=text[:start]+text[end:]
        patch.write_text(text)
    replace_once(patch, "def table(sites):", marker+"\ndef _hm_capture_as(gen, a):\n"
        "    assert _word(gen, a) == 0xd10203ff, hex(_word(gen, a))\n"
        "    return '{ extern void rt_hyrulem_capture_as(Cpu *); rt_hyrulem_capture_as(c); c->sp -= 0x80; }'\n\n\n"
        "PATCHES['hm_as_play_entry'] = _hm_capture_as\n\n\n"
        "def _hm_world_tick(gen, a):\n"
        "    assert _word(gen, a) == 0xf94002c8, hex(_word(gen, a))\n"
        "    # The original caller has joined the actor jobs and reset access phase.\n"
        "    get = gen.sites['pouch_item_get']\n"
        "    adrp, ldr = _word(gen, get + 0x24), _word(gen, get + 0x30)\n"
        "    assert adrp & 0x9f00001f == 0x9000001a and ldr & 0xffc003ff == 0xf940035a\n"
        "    got = adrp_page(adrp, get + 0x24) + ((ldr >> 10) & 0xfff) * 8\n"
        "    return ('{ extern void rt_hyrulem_tick(Cpu *, uint64_t); '\n"
        "            f'rt_hyrulem_tick(c, LD64({got:#x}ull) + 0x10); c->x[8] = LD64(c->x[22]); }}')\n\n\n"
        "PATCHES['hm_world_tick'] = _hm_world_tick\n\n\n"
        "def _hm_fx_capture(gen, a):\n"
        "    word = _word(gen, a)\n"
        "    assert word == 0x17ffffe6, hex(word)\n"
        "    return ('{ extern void rt_hyrulem_capture_effect(Cpu *); rt_hyrulem_capture_effect(c); } '\n"
        "            f'goto L_{a - 0x68:x};')\n\n"
        "def _hm_fx_stop(gen, a):\n"
        "    assert _word(gen, a) == 0xa9bd7bfd, hex(_word(gen, a))\n"
        "    return '{ extern void rt_hyrulem_stop_effect(Cpu *); rt_hyrulem_stop_effect(c); c->sp -= 0x30; ST64(c->sp, c->x[29]); ST64(c->sp + 8, c->x[30]); }'\n\n"
        "def _hm_fx_created(gen, a):\n"
        "    assert _word(gen, a) == 0xb9001a88, hex(_word(gen, a))\n"
        "    return '{ extern void rt_hyrulem_created_effect(Cpu *); rt_hyrulem_created_effect(c); ST32(c->x[20] + 0x18, (uint32_t)c->x[8]); }'\n\n"
        "def _hm_world_ai_dispatch(gen, a):\n"
        "    assert _word(gen, a) == 0xf942ae60, hex(_word(gen, a))\n"
        "    return ('{ extern _Bool rt_hyrulem_world_ai(Cpu *); if (rt_hyrulem_world_ai(c)) ' + f'goto L_{a + 0x14:x}; ' + 'c->x[0] = LD64(c->x[19] + 0x558); }')\n\n"
        "PATCHES['hm_world_ai_dispatch'] = _hm_world_ai_dispatch\n"
        "PATCHES['hm_fx_created'] = _hm_fx_created\n"
        "PATCHES['hm_fx_capture'] = _hm_fx_capture\nPATCHES['hm_fx_stop'] = _hm_fx_stop\n\ndef table(sites):")
    text=patch.read_text()
    marker="# HyruleM material award hook"
    if marker in text:
        a=text.index(marker);b=text.index("def table(sites):",a);text=text[:a]+text[b:]
    addition=(marker+"\ndef _hm_item_pickup(gen, a):\n"
        "    assert _word(gen, a) == 0xa9bc7bfd, hex(_word(gen, a))\n"
        "    return '{ extern _Bool rt_hyrulem_item_pickup(Cpu *); if (rt_hyrulem_item_pickup(c)) return; c->sp -= 0x40; ST64(c->sp, c->x[29]); ST64(c->sp + 8, c->x[30]); }'\n\n"
        "PATCHES['hm_item_pickup'] = _hm_item_pickup\n\n\n")
    addition+=("def _hm_capture_birth(gen, a):\n"
        "    assert _word(gen, a) == 0xf942e2c8, hex(_word(gen, a))\n"
        "    return '{ extern void rt_hyrulem_capture_birth(Cpu *); rt_hyrulem_capture_birth(c); c->x[8] = LD64(c->x[22] + 0x5c0); }'\n\n"
        "PATCHES['hm_birth_capture'] = _hm_capture_birth\n\n")
    addition+=("def _hm_capture_zero(gen, a):\n"
        "    assert _word(gen, a) == 0xa9bb7bfd, hex(_word(gen, a))\n"
        "    return '{ extern void rt_hyrulem_capture_zero(Cpu *); rt_hyrulem_capture_zero(c); c->sp -= 0x50; ST64(c->sp, c->x[29]); ST64(c->sp + 8, c->x[30]); }'\n\n"
        "PATCHES['hm_delete_actor'] = _hm_capture_zero\n\n")
    addition+=("def _hm_weapon_drop(gen, a):\n"
        "    assert _word(gen, a) == 0xf942cc08, hex(_word(gen, a))\n"
        "    return '{ extern void rt_hyrulem_capture_drop(Cpu *); rt_hyrulem_capture_drop(c); c->x[8] = LD64(c->x[0] + 0x598); }'\n\n"
        "PATCHES['hm_weapon_drop'] = _hm_weapon_drop\nPATCHES['hm_weapon_throw'] = _hm_weapon_drop\n\n")
    text=text.replace("def table(sites):",addition+"def table(sites):")
    patch.write_text(text)
    main = root / "runtime/main.c"
    replace_once(main, '#include "gamelib.h"', '#include "gamelib.h"\n#include "gamebuild.h"')
    replace_once(main, 'bool scan = false, precompile = false, add_steam = false;',
                 'bool scan = false, precompile = false, add_steam = false, hyrulem_id = false, hyrulem_build = false;')
    replace_once(main, 'else if (!strcmp(argv[i], "--config")) config = true;',
                 'else if (!strcmp(argv[i], "--config")) config = true;\n'
                 '        else if (!strcmp(argv[i], "--hyrulem-gamelib-id")) hyrulem_id = true;\n'
                 '        else if (!strcmp(argv[i], "--hyrulem-build-game")) hyrulem_build = true;')
    replace_once(main, '    settings_load();',
                 '    settings_load();\n'
                 '    if (hyrulem_id) { puts(gamelib_id()); return 0; }\n'
                 '    if (hyrulem_build) {\n'
                 '        int cores = SDL_GetNumLogicalCPUCores(), ram = SDL_GetSystemRAM() / 1024;\n'
                 '        int jobs = cores < ram - 2 ? cores : ram - 2; if (jobs < 1) jobs = 1;\n'
                 '        GameBuildOpts opts = {NULL, jobs}; char why[800] = "";\n'
                 '        bool ok = gamebuild_run(root, &opts, why, sizeof why);\n'
                 '        if (!ok) fprintf(stderr, "[hyrulem-build] %s\\n", why);\n'
                 '        return ok ? 0 : 1;\n'
                 '    }')
    shutil.copyfile(HERE / "runtime/hyrulem_scene.inc", root / "runtime/hyrulem_scene.inc")
    shutil.copyfile(HERE / "runtime/hyrulem.c", root / "runtime/hyrulem.c")
    shutil.copyfile(HERE / "runtime/hyrulem_items.inc", root / "runtime/hyrulem_items.inc")
    shutil.copyfile(HERE / "runtime/hyrulem_pose.inc", root / "runtime/hyrulem_pose.inc")
    shutil.copyfile(HERE / "runtime/hyrulem_effects.inc", root / "runtime/hyrulem_effects.inc")
    replace_once(root / "runtime/control.c", "MAX_LINE = 4096,", "MAX_LINE = 65536,")
    shutil.copyfile(HERE / "runtime/hyrulem_world.inc", root / "runtime/hyrulem_world.inc")
    shutil.copyfile(HERE / "runtime/hyrulem_birth.inc", root / "runtime/hyrulem_birth.inc")
    shutil.copyfile(HERE / "runtime/hyrulem_lifecycle.inc", root / "runtime/hyrulem_lifecycle.inc")
    shutil.copyfile(HERE / "runtime/hyrulem_weapons.inc", root / "runtime/hyrulem_weapons.inc")
    stamp = {"birth_sha256": hashlib.sha256((HERE / "runtime/hyrulem_birth.inc").read_bytes()).hexdigest(), "lifecycle_sha256": hashlib.sha256((HERE / "runtime/hyrulem_lifecycle.inc").read_bytes()).hexdigest(), "scene_sha256": hashlib.sha256((HERE / "runtime/hyrulem_scene.inc").read_bytes()).hexdigest(), "upstream": UPSTREAM, "preview_abi": 13, "effects_sha256": hashlib.sha256((HERE / "runtime/hyrulem_effects.inc").read_bytes()).hexdigest(), "weapons_sha256": hashlib.sha256((HERE / "runtime/hyrulem_weapons.inc").read_bytes()).hexdigest(), "world_sha256": hashlib.sha256((HERE / "runtime/hyrulem_world.inc").read_bytes()).hexdigest(), "pose_sha256": hashlib.sha256((HERE / "runtime/hyrulem_pose.inc").read_bytes()).hexdigest(), "items_sha256": hashlib.sha256((HERE / "runtime/hyrulem_items.inc").read_bytes()).hexdigest(), "adapter_sha256": hashlib.sha256((HERE / "runtime/hyrulem.c").read_bytes()).hexdigest()}
    (root / "hyrulem-build.json").write_text(json.dumps(stamp, indent=2) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    apply(args.source.resolve())
    print("HyruleM native preview adapter applied; rebuild app AND generator helper.")
