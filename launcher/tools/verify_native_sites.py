"""Verify the research site map against a privately supplied BOTW main NSO.

Requires Python 3 and lz4. Reads only; prints metadata, never game code or assets.
This checks static signatures, not the runtime safety of calling a function.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import struct

ROOT = Path(__file__).resolve().parents[1]


def pattern_regex(pattern):
    parts = []
    for token in pattern.split():
        value, _, mask = token.partition('/')
        value, mask = int(value, 16), int(mask or 'ffffffff', 16)
        for i in range(4):
            v, m = (value >> (8*i)) & 255, (mask >> (8*i)) & 255
            if m == 255:
                parts.append(re.escape(bytes([v])))
            else:
                parts.append(b'[' + b''.join(re.escape(bytes([x])) for x in range(256) if x & m == v & m) + b']')
    return re.compile(b''.join(parts), re.DOTALL)


def verify(path, manifest):
    raw = path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != manifest['module_sha256']:
        raise ValueError('module SHA256 differs; no addresses may be used for this file')
    if len(raw) < 256 or raw[:4] != b'NSO0' or raw[0x40:0x60].rstrip(b'\0').hex() != manifest['build_id']:
        raise ValueError('invalid NSO/build ID')
    import lz4.block
    flags = struct.unpack_from('<I', raw, 12)[0]
    segments = []
    for i in range(3):
        file_off, mem_off, size = struct.unpack_from('<III', raw, 0x10 + i*16)
        compressed = struct.unpack_from('<I', raw, 0x60 + i*4)[0]
        if size > 128*1024*1024 or mem_off + size > 128*1024*1024 or file_off + compressed > len(raw):
            raise ValueError('segment outside bounds')
        data = raw[file_off:file_off + compressed]
        if flags & (1 << i):
            data = lz4.block.decompress(data, uncompressed_size=size)
        if len(data) != size:
            raise ValueError('incorrect decompressed size')
        segments.append((mem_off, data))
    image = bytearray(max(off + len(data) for off, data in segments))
    for off, data in segments:
        image[off:off+len(data)] = data
    base = int(manifest['main_base'], 0)
    text_off, text = segments[0]
    for name, f in manifest['functions'].items():
        off = int(f['address'], 0) - base
        if not text_off <= off < off + f['size'] <= text_off + len(text):
            raise ValueError(f'{name}: function outside text')
        if hashlib.sha256(image[off:off+f['size']]).hexdigest() != f['sha256']:
            raise ValueError(f'{name}: code fingerprint differs')
        hits = [m.start() for m in pattern_regex(f['pattern']).finditer(text) if m.start() % 4 == 0]
        expected = off - text_off - 4*f['pattern_at']
        if expected not in hits or f['pattern_unique'] and len(hits) != 1:
            raise ValueError(f'{name}: signature/uniqueness differs')
    for edge in manifest['call_edges']:
        addr = int(edge['site'], 0)
        word = struct.unpack_from('<I', image, addr-base)[0]
        immediate = word & 0x3ffffff
        if immediate & (1 << 25):
            immediate -= 1 << 26
        if word & 0xfc000000 != 0x94000000 or addr + immediate*4 != int(manifest['functions'][edge['target']]['address'], 0):
            raise ValueError('call edge does not match')
    return {'game_version': manifest['game_version'], 'module_sha256': manifest['module_sha256'],
            'functions_verified': len(manifest['functions']), 'call_edges_verified': len(manifest['call_edges']),
            'unique_patterns': sum(f['pattern_unique'] for f in manifest['functions'].values()),
            'static_verification': 'passed', 'runtime_call_validation': 'pending', 'native_backend_ready': False}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('main', type=Path)
    args = parser.parse_args()
    manifest = json.loads((ROOT/'internal/nativebridge/native_190_sites.json').read_text())
    try:
        print(json.dumps(verify(args.main, manifest), indent=2))
    except (ValueError, ImportError) as exc:
        parser.exit(1, str(exc) + '\n')
