"""Compile and execute the world-hook fragment with synthetic guest memory.

No game module or generated game code is required or written by this test.
The real adapter's CPU/lifecycle behavior is covered by test_runtime.c.
"""
import argparse
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
from types import SimpleNamespace

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('runtime',type=Path)
args=parser.parse_args()
sys.path.insert(0,str(args.runtime/'tools'))
import patches

base=0x7000000000
image=bytearray(0x1000)
struct.pack_into('<I',image,0x100,0xf94002c8) # synthetic ldr x8,[x22]
struct.pack_into('<I',image,0x24,0x9000001a) # synthetic SafeString GOT page
struct.pack_into('<I',image,0x30,0xf940435a) # synthetic GOT slot +0x80
gen=SimpleNamespace(base=base,nso=SimpleNamespace(image=image),sites={'pouch_item_get':base})
fragment=patches._hm_world_tick(gen,base+0x100)
text=(args.runtime/'tools/patches.py').read_text()
item_tick=text[text.index('def _item_tick('):text.index('# tools/game_sites.py site ->')]
assert 'rt_hyrulem_tick' not in item_tick
assert "PATCHES['hm_world_tick'] = _hm_world_tick" in text
source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
#include <sys/mman.h>
static unsigned calls;
void rt_hyrulem_tick(Cpu *c, uint64_t vt) {
    assert(c->x[22]==0x7000000200ull && vt==0x7000000310ull);calls++;
}
static void generated_hook(Cpu *c) { FRAGMENT }
int main(void) {
    assert(mmap((void *)0x7000000000ull,4096,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)!=(void *)-1);
    ST64(0x7000000080ull,0x7000000300ull);ST64(0x7000000200ull,0x7000000400ull);
    Cpu c;memset(&c,0xa5,sizeof c);c.x[22]=0x7000000200ull;Cpu expected=c;
    expected.x[8]=0x7000000400ull;generated_hook(&c);
    assert(calls==1&&!memcmp(&c,&expected,sizeof c));return 0;
}
'''.replace('FRAGMENT',fragment)
with tempfile.TemporaryDirectory(prefix='hyrulem-hook-') as temp:
    p=Path(temp);(p/'hook.c').write_text(source)
    subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
    subprocess.run([str(p/'hook')],check=True)
print('Post-job hook emission: argument, original load and CPU preservation passed')
