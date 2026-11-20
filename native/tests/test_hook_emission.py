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

# A blocked award returns before its original stack write. An ordinary award
# must retain the exact STP pre-index operation and every other CPU register.
struct.pack_into('<I',image,0x180,0xa9bc7bfd)
fragment=patches._hm_item_pickup(gen,base+0x180)
source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
#include <sys/mman.h>
static _Bool blocked;
_Bool rt_hyrulem_item_pickup(Cpu *c) { return blocked; }
static void generated_hook(Cpu *c) { FRAGMENT }
int main(void) {
 assert(mmap((void *)0x7000000000ull,4096,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)!=(void *)-1);
 Cpu c;memset(&c,0xa5,sizeof c);c.sp=0x7000000f00ull;Cpu before=c;
 blocked=1;generated_hook(&c);assert(!memcmp(&c,&before,sizeof c));
 blocked=0;generated_hook(&c);Cpu expected=before;expected.sp-=0x40;
 assert(!memcmp(&c,&expected,sizeof c)&&LD64(c.sp)==before.x[29]&&LD64(c.sp+8)==before.x[30]);return 0;
}
'''.replace('FRAGMENT',fragment)
with tempfile.TemporaryDirectory(prefix='hyrulem-item-hook-') as temp:
 p=Path(temp);(p/'hook.c').write_text(source)
 subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
 subprocess.run([str(p/'hook')],check=True)
print('Material hook: queued return and original STP/CPU preservation passed')

# Native weapon capture/deletion callbacks only inspect the incoming CPU.
# The generated fragments must still execute exactly the overwritten load/STP.
for name,offset,word,callback in [
 ('delete',0x200,0xa9bb7bfd,'rt_hyrulem_capture_zero'),
 ('drop',0x240,0xf942cc08,'rt_hyrulem_capture_drop'),
 ('throw',0x280,0xf942cc08,'rt_hyrulem_capture_drop'),
]:
 struct.pack_into('<I',image,offset,word)
 fragment=(patches._hm_capture_zero if name=='delete' else patches._hm_weapon_drop)(gen,base+offset)
 expected=('expected.sp-=0x50; assert(LD64(c.sp)==before.x[29]&&LD64(c.sp+8)==before.x[30]);'
           if name=='delete' else 'expected.x[8]=0x12345678ull;')
 source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
#include <sys/mman.h>
static unsigned captures;
void CALLBACK(Cpu *c) {assert(c->x[0]==0x7000000400ull);captures++;}
static void generated_hook(Cpu *c) { FRAGMENT }
int main(void) {
 assert(mmap((void *)0x7000000000ull,4096,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)!=(void *)-1);
 Cpu c;memset(&c,0xa5,sizeof c);c.sp=0x7000000f00ull;c.x[0]=0x7000000400ull;
 ST64(c.x[0]+0x598,0x12345678ull);Cpu before=c,expected=c;generated_hook(&c);
 EXPECTED
 assert(captures==1&&!memcmp(&c,&expected,sizeof c));return 0;
}
'''.replace('CALLBACK',callback).replace('FRAGMENT',fragment).replace('EXPECTED',expected)
 with tempfile.TemporaryDirectory(prefix='hyrulem-'+name+'-hook-') as temp:
  p=Path(temp);(p/'hook.c').write_text(source)
  subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
  subprocess.run([str(p/'hook')],check=True)
print('Deletion/drop/throw hooks: original load/STP and full CPU preservation passed')

# Visual effect hooks do not replace the native request. The capture runs only
# after it returned, then preserves the original branch to its epilogue.
struct.pack_into('<I',image,0x340,0x17ffffe6)
fragment=patches._hm_fx_capture(gen,base+0x340)
source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
static unsigned calls;
void rt_hyrulem_capture_effect(Cpu *c){assert(c->x[19]==19&&c->x[20]==20&&c->x[21]==21);calls++;}
static void generated(Cpu *c){FRAGMENT assert(0);L_70000002d8:;}
int main(void){Cpu c;memset(&c,0xa5,sizeof c);c.x[19]=19;c.x[20]=20;c.x[21]=21;Cpu before=c;generated(&c);assert(calls==1&&!memcmp(&c,&before,sizeof c));return 0;}
'''.replace('FRAGMENT',fragment)
with tempfile.TemporaryDirectory(prefix='hyrulem-fx-capture-') as temp:
 p=Path(temp);(p/'hook.c').write_text(source)
 subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
 subprocess.run([str(p/'hook')],check=True)
struct.pack_into('<I',image,0x380,0xa9bd7bfd)
fragment=patches._hm_fx_stop(gen,base+0x380)
source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
#include <sys/mman.h>
static unsigned calls;
void rt_hyrulem_stop_effect(Cpu *c){assert(c->x[0]==0x7000000400ull);calls++;}
static void generated(Cpu *c){FRAGMENT}
int main(void){assert(mmap((void *)0x7000000000ull,4096,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)!=(void *)-1);Cpu c;memset(&c,0xa5,sizeof c);c.sp=0x7000000f00ull;c.x[0]=0x7000000400ull;Cpu expected=c;expected.sp-=0x30;generated(&c);assert(calls==1&&!memcmp(&c,&expected,sizeof c)&&LD64(c.sp)==c.x[29]&&LD64(c.sp+8)==c.x[30]);return 0;}
'''.replace('FRAGMENT',fragment)
with tempfile.TemporaryDirectory(prefix='hyrulem-fx-stop-') as temp:
 p=Path(temp);(p/'hook.c').write_text(source)
 subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
 subprocess.run([str(p/'hook')],check=True)
print('Visual events: post-request branch and native stop STP/CPU preservation passed')

# Direct descriptor emission can omit an output handle. Preserve the initialized
# event hook's original STR and every CPU register/flag in both generator modes.
struct.pack_into('<I',image,0x3c0,0xb9001a88)
fragment=patches._hm_fx_created(gen,base+0x3c0)
source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
#include <sys/mman.h>
static unsigned calls;
void rt_hyrulem_created_effect(Cpu *c){assert(c->x[20]==0x7000000400ull&&c->x[23]==23);calls++;}
static void generated(Cpu *c){FRAGMENT}
int main(void){assert(mmap((void *)0x7000000000ull,4096,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)!=(void *)-1);Cpu c;memset(&c,0xa5,sizeof c);c.x[20]=0x7000000400ull;c.x[23]=23;c.x[8]=0x1234;Cpu before=c;generated(&c);assert(calls==1&&!memcmp(&c,&before,sizeof c)&&LD32(c.x[20]+0x18)==0x1234);return 0;}
'''.replace('FRAGMENT',fragment)
with tempfile.TemporaryDirectory(prefix='hyrulem-fx-created-') as temp:
 p=Path(temp);(p/'hook.c').write_text(source)
 subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
 subprocess.run([str(p/'hook')],check=True)
print('Direct visual descriptor hook: original STR and full CPU preservation passed')

# A follower suppresses only the matched actor's RootAI calculation. The host
# and every unrelated actor must execute the original prologue unchanged.
struct.pack_into('<I',image,0x400,0xa9bd7bfd)
fragment=patches._hm_world_ai_calc(gen,base+0x400)
source='''#include "cpu.h"
#include <assert.h>
#include <string.h>
#include <sys/mman.h>
static _Bool skip;
_Bool rt_hyrulem_world_ai(Cpu *c){assert(c->x[0]==0x7000000400ull);return skip;}
static void generated(Cpu *c){FRAGMENT}
int main(void){assert(mmap((void *)0x7000000000ull,4096,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)!=(void *)-1);Cpu c;memset(&c,0xa5,sizeof c);c.x[0]=0x7000000400ull;c.sp=0x7000000f00ull;Cpu before=c;skip=1;generated(&c);assert(!memcmp(&c,&before,sizeof c));skip=0;generated(&c);Cpu expected=before;expected.sp-=0x30;assert(!memcmp(&c,&expected,sizeof c)&&LD64(c.sp)==before.x[29]&&LD64(c.sp+8)==before.x[30]);return 0;}
'''.replace('FRAGMENT',fragment)
with tempfile.TemporaryDirectory(prefix='hyrulem-world-ai-') as temp:
 p=Path(temp);(p/'hook.c').write_text(source)
 subprocess.run([sys.executable,'-m','ziglang','cc','-std=gnu11','-O0','-UNDEBUG','-I'+str((args.runtime/'runtime').resolve()),str(p/'hook.c'),'-lm','-o',str(p/'hook')],check=True)
 subprocess.run([str(p/'hook')],check=True)
print('World AI authority: conditional return, original STP and CPU preservation passed')
