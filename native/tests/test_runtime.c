// Synthetic guest memory and callees: tests adapter lifetime/thread boundaries,
// not BOTW behavior. The real game/renderer still needs a two-instance playtest.
#include <assert.h>
#include <stdarg.h>
#include <sys/mman.h>
#include "../runtime/hyrulem.c"

struct ControlReply { bool failed; };
GameSites g_site;
const char *g_game_version="1.9.0";
static unsigned calls, creates, deletes, moves, plays,bone_writes;static float last_bone_matrix[12];
static uint64_t mock_actor=HEAP_BASE+0x3000, retained=HEAP_BASE+0x4000;
static bool reject,fail_handle,deny_lease;
static uint64_t ground_pouch_item;static unsigned inventory_grants,inventory_debits;static bool inventory_full,inventory_ambiguous,remove_ambiguous;
static unsigned create_state=1;
static unsigned shape_writes,property_writes;
static unsigned effect_emits,effect_stops;static uint64_t effect_user_expected;
static bool world_test,world_no_as_request;static uint64_t invalidate_world_actor;
static bool empty_inventory;static bool weapon_test;static uint64_t weapon_vt,weapon_pouch;static unsigned weapon_entries;static uint32_t granted_modifier;static int granted_bonus;
static uint64_t local_equipped[6],gear_handles[32],gear_actors[32];static unsigned gear_allocations,gear_handles_count;static bool reject_equipment;
void *hle_alloc(uint64_t size) { if(size==FX_MAX*16)return (void *)(uintptr_t)(HLE_HEAP+0x30000);assert(size==BYTES);return (void *)(uintptr_t)(storage?HLE_HEAP+0x1000+0x400*gear_allocations++:HLE_HEAP); }
static uint64_t handle_actor(uint64_t handle) {if(handle==HLE_HEAP)return mock_actor;for(unsigned i=gear_handles_count;i>0;i--)if(gear_handles[i-1]==handle)return gear_actors[i-1];assert(false);return 0;}
void guest_call(Cpu *c,uint64_t fn) {
    calls++;
    if(fn==g_site.hm_material_set||fn==g_site.hm_bone_visible_set){uint64_t bits=LD64(c->x[0]+(fn==g_site.hm_material_set?0xf0:0xe8));unsigned i=(unsigned)c->x[1];uint32_t v=LD32(bits+i/32*4),mask=1u<<(i%32);ST32(bits+i/32*4,c->x[2]?v|mask:v&~mask);shape_writes++;}
    else if(fn==g_site.hm_fx_set_int||fn==g_site.hm_fx_set_float){uint32_t v=(uint32_t)c->x[2];if(fn==g_site.hm_fx_set_float)memcpy(&v,&c->v[0],4);ST32(LD64(c->x[0]+0x70)+c->x[1]*4,v);ST64(c->x[0]+0x68,LD64(c->x[0]+0x68)|(1ull<<c->x[1]));property_writes++;}
    else if(fn==g_site.hm_fx_emit){assert(c->x[0]==effect_user_expected&&!strcmp((char *)(uintptr_t)c->x[1],"AttackSpin"));uint64_t context=HEAP_BASE+0xe0000+effect_emits*0x100;effect_emits++;ST64(c->x[2],context);ST32(c->x[2]+8,55);ST32(context+0x20,55);}
    else if(fn==g_site.hm_fx_stop){effect_stops++;ST32(c->x[0]+8,LD32(c->x[0]+8)|0x10);}
    else if (fn==g_site.hm_create) {
        creates++;uint64_t a=mock_actor;bool gear=c->x[3]!=HLE_HEAP+HANDLE;
        if(gear) {if(reject_equipment){c->x[0]=0;return;}assert(gear_handles_count<32);a=HEAP_BASE+0x20000+0x1000*gear_handles_count;gear_handles[gear_handles_count]=c->x[3];gear_actors[gear_handles_count++]=a;uint64_t request=HEAP_BASE+0xfc000+gear_handles_count*0x20;ST64(c->x[3],request);ST64(request+0x10,a);ST64(a+0x560,HEAP_BASE+0x5000);ST64(a+0x4e0,a+0x900);ST64(a+0x10,a+0xa00);snprintf((char *)(uintptr_t)(a+0xa00),48,"%s",(char *)(uintptr_t)c->x[1]);float initial[12]={1,0,0,0,0,1,0,0,0,0,1,0};for(int j=0;j<3;j++)initial[j*4+3]=read_float(c->x[3]+VECTOR+j*4);memcpy((void *)(uintptr_t)(a+0x398),initial,sizeof initial);}
        if(weapon_test&&gear){ST64(a,weapon_vt);ST32(a+0xd4c,100);}
        ST8(a+0x64,create_state);c->x[0]=!reject;
    }
    else if (fn==g_site.hm_handle_ready) c->x[0]=1;
    else if (fn==g_site.hm_handle_failed) c->x[0]=fail_handle;
    else if (fn==g_site.hm_handle_get || fn==g_site.hm_handle_wake) c->x[0]=handle_actor(c->x[0]);
    else if(fn==g_site.player_equipment){assert(c->x[1]<6);c->x[0]=local_equipped[c->x[1]];}
    else if (fn==g_site.actor_index_assign) {
        uint64_t a=c->x[1],block=a==mock_actor?retained:a+0x800;ST64(c->x[0],block); ST32(c->x[0]+8,7); ST8(c->x[0]+12,1);
        ST32(block+0x40,7);ST64(block+0x48,a);c->x[0]=1;
    } else if (fn==g_site.actor_index_resolve) {
        assert(!LD64(c->x[1]+8)); // Local references cannot escape a callback.
        uint64_t a=LD64(LD64(c->x[0])+0x48);ST64(c->x[1]+8,a);c->x[0]=deny_lease?0:a;
        if(a==invalidate_world_actor){ST32(LD64(a+0x7c8)+0x58,0);invalidate_world_actor=0;}
    }
    else if (fn==g_site.actor_index_release) {ST64(c->x[0],0);ST32(c->x[0]+8,UINT32_MAX);ST8(c->x[0]+12,0);}
    else if (fn==g_site.actor_link_release) ST64(c->x[0]+8,0);
    else if (fn==g_site.hm_delete_actor) {assert(c->x[0]==mock_actor||c->x[0]>=HEAP_BASE+0x20000);deletes++;ST8(c->x[0]+0x64,3);}
    else if (fn==g_site.hm_set_mtx) {assert(c->x[0]==mock_actor||c->x[0]>=HEAP_BASE+0x20000);assert(c->x[2]<=1);assert(c->x[3]==0);memcpy((void *)(uintptr_t)(c->x[0]+0x398),(void *)(uintptr_t)c->x[1],48);moves++;}
    else if (fn==g_site.hm_play_as) {assert(c->x[0]==HEAP_BASE+0x5000||world_test);plays++;
        if(world_test&&!world_no_as_request){uint64_t request=HEAP_BASE+0xe8000;memset((void *)(uintptr_t)request,0,0x38);ST64(request+0x10,LD64(c->x[1]+8));ST32(request+0x18,(uint32_t)c->x[2]);ST32(request+0x1c,(uint32_t)c->x[3]);ST8(request+0x21,(unsigned char)c->x[4]);Cpu observed=*c;observed.x[1]=request;rt_hyrulem_capture_as(&observed);}
        c->x[0]=world_test&&!world_no_as_request?0:1;}
    else if(fn==g_site.hm_bone_set){bone_writes++;memcpy(last_bone_matrix,(void *)(uintptr_t)c->x[1],sizeof last_bone_matrix);assert(c->x[2]<POSE_MAX);}
    else if(fn==g_site.pouch_page_count)c->x[0]=empty_inventory?0:weapon_test?weapon_entries:1;
    else if(fn==g_site.pouch_page_item)c->x[0]=weapon_test?weapon_pouch+c->x[2]*0x100:ground_pouch_item;
    else if(fn==g_site.hm_item_remove){assert(c->x[2]==1&&c->x[3]==0);if(LD32(ground_pouch_item+0x20)>0){ST32(ground_pouch_item+0x20,LD32(ground_pouch_item+0x20)-1);inventory_debits++;if(remove_ambiguous){ST32(ground_pouch_item+0x20,UINT32_MAX);deny_lease=true;}}}
    else if(fn==g_site.hm_item_get&&weapon_test){assert(c->x[2]==900&&c->x[3]);granted_modifier=LD32(c->x[3]);granted_bonus=(int32_t)LD32(c->x[3]+4);ST64(c->x[3],0);if(!inventory_full){uint64_t p=weapon_pouch+weapon_entries++*0x100;ST32(p+0x18,0);ST32(p+0x20,900);ST8(p+0x25,1);ST64(p+0x30,p+0x80);strcpy((char *)(uintptr_t)(p+0x80),"Weapon_Sword_001");inventory_grants++;}}
    else if(fn==g_site.hm_item_get){assert(c->x[2]==1&&c->x[3]==0);if(!inventory_full){ST32(ground_pouch_item+0x20,LD32(ground_pouch_item+0x20)+1);inventory_grants++;if(inventory_ambiguous)ST32(ground_pouch_item+0x20,UINT32_MAX);}}
    // Deliberately clobber every register: the hook must preserve the caller.
    memset(c->v,0x5a,sizeof c->v);c->x[3]=0xdeadbeef;c->nzcv=0xabcdef;c->fpsr=91;
}
void control_register(const ControlCmd *cmds,int n) {assert(n==19);}
void control_field_str(ControlReply *r,const char *k,const char *v) {}
void control_field_int(ControlReply *r,const char *k,int64_t v) {}
void control_field_num(ControlReply *r,const char *k,double v) {}
void control_field_bool(ControlReply *r,const char *k,bool v) {}
bool control_reply_failed(const ControlReply *r){return r->failed;}
void control_fail(ControlReply *r,const char *fmt,...) {r->failed=true;}

static void map_at(uint64_t a,size_t n) {assert(mmap((void *)(uintptr_t)a,n,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)==(void *)(uintptr_t)a);}
int main(void) {
    setenv("HYRULEM_NATIVE_PREVIEW","1",1);
    map_at(HLE_HEAP,0x100000);map_at(HEAP_BASE,0x100000);
    uint64_t site=1;
    #define SITE(n) g_site.n=site++;
    GAME_SITES_OPTIONAL(SITE)
    #undef SITE
    g_site.player_info=HEAP_BASE;g_site.hm_creator=HEAP_BASE+8;g_site.hm_heap_mgr=HEAP_BASE+16;
    ST64(g_site.player_info,HEAP_BASE+0x100);ST64(HEAP_BASE+0x160,HEAP_BASE+0x1000);
    ST8(HEAP_BASE+0x1064,1);
    float initial_local[12]={1,0,0,3434,0,1,0,236,0,0,1,2101};memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),initial_local,sizeof initial_local);
    g_site.hm_actor_mgr=HEAP_BASE+0x28;uint64_t manager=HEAP_BASE+0xd000;
    ST64(g_site.hm_actor_mgr,manager);ST8(manager+0x29,7);ST8(manager+0x2a,8);
    ST64(g_site.hm_creator,HEAP_BASE+0x200);ST64(g_site.hm_heap_mgr,HEAP_BASE+0x300);
    ST64(HEAP_BASE+0x328,HEAP_BASE+0x400);
    ST8(mock_actor+0x64,1);ST64(mock_actor+0x560,HEAP_BASE+0x5000);ST64(mock_actor+0x4e0,HEAP_BASE+0x6000);
    ST32(HEAP_BASE+0x5138,400);ST32(HEAP_BASE+0x50b8,2);ST64(HEAP_BASE+0x50c0,HEAP_BASE+0xb000);
    // The barrier caller owns the other registers; preserve all of them.
    Cpu cpu={0};cpu.x[0]=HEAP_BASE+0x2000;cpu.x[22]=g_site.hm_actor_mgr;cpu.sp=HEAP_BASE+0xf000;cpu.v[3]=123;cpu.fpsr=77;Cpu before=cpu;
    ControlReply r={0};
    char *start[]={"hm_native_open","0123456789abcdef"};open_session(2,start,&r);assert(!r.failed && calls==0);
    char *pose[]={"hm_native_remote","0123456789abcdef","1","3434","236","2101","0.5","move","-","0","0","-","0","0","0","0"};
    remote(16,pose,&r);assert(!r.failed && calls==0); // control thread never touches the game
    cpu.x[22]=0;rt_hyrulem_tick(&cpu,0);assert(calls==0);cpu=before; // Worker/control callbacks cannot drain.
    ST8(manager+0x160,1);rt_hyrulem_tick(&cpu,0);assert(calls==0&&published.rejected_callbacks==1);ST8(manager+0x160,0);
    ST8(manager+0x28,2);rt_hyrulem_tick(&cpu,0);assert(calls==0&&published.rejected_callbacks==2);ST8(manager+0x28,0);
    rt_hyrulem_tick(&cpu,0x7000001000);assert(creates==1 && spawning);assert(!memcmp(&before,&cpu,sizeof cpu));
    spawning_at=now_ms()-16000; // Ready handle must survive a long menu/loading pause.
    rt_hyrulem_tick(&cpu,0x7000001000);assert(published.active && moves==2 && plays==2);assert(!memcmp(&before,&cpu,sizeof cpu));
    // Later poses must move the actor and keep both animation tracks.
    Desired moving=desired;moving.x+=10;moving.yaw=1;moving.seq++;snprintf(moving.tracks[0].name,64,"CutS1");moving.tracks[0].sequence=1;moving.tracks[0].force=true;desired=moving;
    rt_hyrulem_tick(&cpu,0x7000001000);assert(read_float(mock_actor+0x3a4)>3434&&read_float(mock_actor+0x3a4)<3444&&plays==3);assert(!memcmp(&before,&cpu,sizeof cpu));
    rt_hyrulem_tick(&cpu,0x7000001000);assert(plays==3); // no restart from repeated packet
    ST64(HEAP_BASE+0x50d8,HEAP_BASE+0x1000);strcpy((char *)(uintptr_t)(HEAP_BASE+0xc000),"CutS1");ST64(HEAP_BASE+0xc110,HEAP_BASE+0xc000);ST8(HEAP_BASE+0xc121,1);
    Cpu request=before;request.x[0]=HEAP_BASE+0x5000;request.x[1]=HEAP_BASE+0xc100;request.x[2]=0;request.x[3]=0;request.x[4]=1;Cpu saved=request;
    rt_hyrulem_capture_as(&request);assert(capture_count==1&&!strcmp(local_tracks[0].name,"CutS1"));assert(!memcmp(&request,&saved,sizeof request));
    ST64(HEAP_BASE+0x50d8,mock_actor);rt_hyrulem_capture_as(&request);assert(capture_count==1); // remote cannot feed back
    r.failed=false;remote(16,pose,&r);assert(r.failed); // replay cannot resurrect or move
    ST32(HEAP_BASE+0x5138,0);rt_hyrulem_tick(&cpu,0);assert(published.active&&published.visual_only&&!published.animation_ready&&deletes==0);ST32(HEAP_BASE+0x5138,400);
    char *wrong[]={"hm_native_clear","ffffffffffffffff","2"};r.failed=false;clear_remote(3,wrong,&r);assert(r.failed);
    char *clear[]={"hm_native_clear","0123456789abcdef","3"};r.failed=false;clear_remote(3,clear,&r);assert(!r.failed && deletes==0);
    rt_hyrulem_tick(&cpu,0);assert(deletes==1 && !actor && !published.active);
    hyrulem_reset();assert(!storage && !actor && !desired.session[0]);
    // Regression: the native wake operation queues activation. The retained
    // actor can remain in state 2/0 across multiple callbacks, and even a
    // failed resolver can populate a temporary local link that needs release.
    create_state=2;r.failed=false;open_session(2,start,&r);remote(16,pose,&r);
    unsigned old_moves=moves,old_plays=plays,old_deletes=deletes,old_creates=creates;
    rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);
    assert(!failed&&actor&&!published.active&&!strcmp(published.state,"activating"));
    assert(published.actor_state==2&&!strcmp(published.reference_status,"activation-pending"));
    for(int i=0;i<3;i++)rt_hyrulem_tick(&cpu,0);
    assert(creates==old_creates+1&&moves==old_moves&&plays==old_plays&&deletes==old_deletes);
    assert(!LD64(storage+LOCAL+8)&&LD8(storage+INDEX+12));
    deny_lease=true;ST8(mock_actor+0x64,0);rt_hyrulem_tick(&cpu,0);
    assert(!failed&&!LD64(storage+LOCAL+8)&&!strcmp(published.reference_status,"lease-unavailable"));
    deny_lease=false;ST8(mock_actor+0x64,1);rt_hyrulem_tick(&cpu,0);
    assert(published.active&&moves==old_moves+2&&plays==old_plays+2&&published.activation_wait_ticks==5);
    assert(!memcmp(&before,&cpu,sizeof cpu));
    // A pending actor can also be removed safely, without requiring state 1.
    ST8(mock_actor+0x64,2);clear_remote(3,clear,&r);rt_hyrulem_tick(&cpu,0);
    assert(!actor&&deletes==old_deletes+1&&!LD8(storage+INDEX+12));
    // A stale generation is not an activation delay: never move/dereference it.
    hyrulem_reset();create_state=1;open_session(2,start,&r);remote(16,pose,&r);rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);
    old_moves=moves;ST32(retained+0x40,8);rt_hyrulem_tick(&cpu,0);
    assert(!failed&&moves==old_moves&&!actor&&!strcmp(published.state,"streaming"));
    // Activation has a bounded fallback, not a rapid create/delete loop.
    hyrulem_reset();create_state=2;open_session(2,start,&r);remote(16,pose,&r);rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);
    activation_wait_at=now_ms()-16000;rt_hyrulem_tick(&cpu,0);
    assert(fallback_tried&&!failed&&!actor&&spawn_attempts==1);
    rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);activation_wait_at=now_ms()-16000;rt_hyrulem_tick(&cpu,0);
    assert(failed&&spawn_attempts==2&&!strcmp(published.error,"native-activation-timeout"));
    old_creates=creates;rt_hyrulem_tick(&cpu,0);assert(creates==old_creates);
    // Denied cleanup keeps ownership until a later permitted callback.
    hyrulem_reset();create_state=1;open_session(2,start,&r);remote(16,pose,&r);rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);
    deny_lease=true;rt_hyrulem_tick(&cpu,0);activation_wait_at=now_ms()-16000;rt_hyrulem_tick(&cpu,0);
    assert(!failed&&actor&&LD8(storage+INDEX+12)&&!strcmp(published.state,"streaming"));
    old_creates=creates;rt_hyrulem_tick(&cpu,0);assert(creates==old_creates);
    deny_lease=false;rt_hyrulem_tick(&cpu,0);assert(actor&&published.active&&!failed);
    float distant_local[12]={1,0,0,-3607,0,1,0,286,0,0,1,-1805};
    memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),distant_local,sizeof distant_local);
    rt_hyrulem_tick(&cpu,0);assert(!actor&&!failed&&!strcmp(published.state,"out-of-range"));
    memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),initial_local,sizeof initial_local);
    rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);assert(actor&&published.active&&!failed);
    hyrulem_reset();create_state=1;
    // A rejected native creation is reported once, without an allocation/retry loop.
    reject=true;r.failed=false;open_session(2,start,&r);remote(16,pose,&r);
    rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);unsigned count=creates;rt_hyrulem_tick(&cpu,0);assert(failed && creates==count && !strcmp(published.state,"error"));
    desired.received=now_ms()-3001;rt_hyrulem_tick(&cpu,0);assert(!strcmp(published.state,"error")&&published.first_error[0]);
    // One fallback attempt can restore the model without executing Player AS.
    hyrulem_reset();reject=false;fail_handle=true;r.failed=false;open_session(2,start,&r);remote(16,pose,&r);rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);assert(fallback_tried&&!failed);
    fail_handle=false;
    reject=false;unsigned previous_plays=plays;rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);
    assert(published.active&&published.resource_fallback&&published.visual_only&&plays==previous_plays&&spawn_attempts==2);
    assert(strstr((char *)(uintptr_t)(storage+NAME),"_Fallback"));desired.x=3450;desired.seq++;rt_hyrulem_tick(&cpu,0);assert(read_float(mock_actor+0x3a4)>3434&&read_float(mock_actor+0x3a4)<=3450&&plays==previous_plays);

    // A native map-enemy observation is keyed by placement, never a process
    // pointer. Received health waits for the game thread and cannot heal.
    hyrulem_reset();reject=false;r.failed=false;open_session(2,start,&r);remote(16,pose,&r);
    map_at(MAIN_BASE,0x1000);g_site.main_data=MAIN_BASE+0x1000;
    ST32(MAIN_BASE+0x100,0x91240000);ST32(MAIN_BASE+0x104,0xd65f03c0); // synthetic add x0,x0,#0x900; ret
    uint64_t enemy=HEAP_BASE+0x7000,map=HEAP_BASE+0x8000,vt=HEAP_BASE+0xa000;
    strcpy((char *)(uintptr_t)(HEAP_BASE+0x9000),"Enemy_Bokoblin_Junior");
    ST64(enemy,vt);ST64(vt+0x2b8,MAIN_BASE+0x100);ST64(enemy+0x10,HEAP_BASE+0x9000);
    memcpy((void *)(uintptr_t)(enemy+0x398),initial_local,sizeof initial_local);
    ST8(enemy+0x64,1);ST64(enemy+0x7c8,map);ST32(map+0x58,42);ST32(enemy+0x900,100);
    uint64_t creator=HEAP_BASE+0x200,head=creator+0x38;
    ST32(creator+0x4c,0x7b8);ST32(creator+0x48,1);ST64(head,enemy+0x7b8);ST64(enemy+0x7b8,head);
    rt_hyrulem_tick(&cpu,0);assert(world_count==1&&world_seen[0].health==100);
    char *hp[]={"hm_enemy_health","0123456789abcdef","42","Enemy_Bokoblin_Junior","0","0","0","60"};
    receive_health(8,hp,&r);assert(!r.failed&&LD32(enemy+0x900)==100);
    last_world_scan=0;rt_hyrulem_tick(&cpu,0);assert(LD32(enemy+0x900)==60);
    hp[7]="90";receive_health(8,hp,&r);last_world_scan=0;rt_hyrulem_tick(&cpu,0);assert(LD32(enemy+0x900)==60);
    ST32(map+0x58,99);ST32(enemy+0x900,100);last_world_scan=0;rt_hyrulem_tick(&cpu,0);assert(LD32(enemy+0x900)==100);
    hp[3]="GameROMPlayer";r.failed=false;receive_health(8,hp,&r);assert(r.failed);
    // Frame-rate independent smoothing, shortest-angle interpolation and
    // teleport snapping are checked without wall-clock sleeps.
    for(int hz=30;hz<=120;hz*=2) {
        rendered_ready=false;rendered_at=0;memset(rendered_pose,0,sizeof rendered_pose);
        Desired value={0};smooth_pose(&value,1000);
        for(int step=1;step<=hz;step++){value.x=10;value.yaw=3.13f;smooth_pose(&value,1000+(uint64_t)step*1000/hz);}
        assert(fabsf(value.x-10)<.001f&&fabsf(value.yaw-3.13f)<.001f);
        value.x=100;value.yaw=-3.13f;smooth_pose(&value,2100);assert(value.x==100&&fabsf(value.yaw-3.13f)<.1f);
    }
    // Three visible pieces are independent actors; a failed resource never
    // removes the body. Queue handlers cannot execute guest functions.
    hyrulem_reset();create_state=1;reject=false;fail_handle=false;deny_lease=false;open_session(2,start,&r);remote(16,pose,&r);
    char *piece[]={"hm_native_equipment","0123456789abcdef","1","3","Armor_Default_Head","1","0","0","0","0","1","0","0","0","0","1","0"};
    unsigned before_commands=calls;r.failed=false;receive_equipment(17,piece,&r);assert(!r.failed&&calls==before_commands);
    piece[3]="4";piece[4]="Armor_001_Upper";receive_equipment(17,piece,&r);assert(!r.failed);
    piece[3]="0";piece[4]="Weapon_Sword_001";piece[8]="2";receive_equipment(17,piece,&r);assert(!r.failed);
    for(int i=0;i<4;i++)rt_hyrulem_tick(&cpu,0);
    assert(published.active&&published.equipment_active==3);
    assert(gear_visual[0].actor!=actor&&gear_visual[0].actor!=gear_visual[3].actor);
    float s=sinf(.5f),co=cosf(.5f);assert(fabsf(read_float(gear_visual[0].actor+0x3a4)-(3434+2*co))<.01f);
    assert(fabsf(read_float(gear_visual[0].actor+0x3c4)-(2101-2*s))<.01f);
    assert(!memcmp(&cpu,&before,sizeof cpu));
    // AS slots survive capture/replay; forced continuous movement is not
    // restarted, while a new attack sequence still is.
    Desired motion=desired;snprintf(motion.tracks[0].name,64,"MoveUpper");motion.tracks[0].slot=1;motion.tracks[0].force=true;motion.tracks[0].sequence=10;
    Track cache[2]={0};unsigned p0=plays;play_tracks(&cpu,storage,actor,&motion,cache,0);assert(plays==p0+2&&cache[0].slot==1);
    motion.tracks[0].sequence++;play_tracks(&cpu,storage,actor,&motion,cache,0);assert(plays==p0+2);
    strcpy(motion.tracks[0].name,"CutS1");motion.tracks[0].sequence++;play_tracks(&cpu,storage,actor,&motion,cache,0);assert(plays==p0+3);
    motion.tracks[0].sequence++;play_tracks(&cpu,storage,actor,&motion,cache,0);assert(plays==p0+4);
    // Capture equipped model transforms relative to the owner, using only
    // reads. A marker beside the source actor must survive the whole path.
    uint64_t source=HEAP_BASE+0x90000;local_equipped[0]=source;ST8(source+0x64,1);ST64(source+0x10,source+0x800);strcpy((char *)(uintptr_t)(source+0x800),"Weapon_Sword_001");ST64(source+0x4e0,source+0x900);ST32(source+0x200,0x12345678);
    float ownerMatrix[12]={1,0,0,100,0,1,0,200,0,0,1,300};memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),ownerMatrix,sizeof ownerMatrix);
    ownerMatrix[3]=102;ownerMatrix[11]=303;memcpy((void *)(uintptr_t)(source+0x958),ownerMatrix,sizeof ownerMatrix);
    capture_gear(&cpu,HEAP_BASE+0x1000);assert(!strcmp(local_gear[0].name,"Weapon_Sword_001")&&local_gear[0].matrix[3]==2&&local_gear[0].matrix[11]==3&&LD32(source+0x200)==0x12345678);
    // Native equipment categories are sword/shield/bow, not wire slots.
    for(int raw=1;raw<=2;raw++){uint64_t equipment=source+raw*0x1000;local_equipped[raw]=equipment;ST8(equipment+0x64,1);ST64(equipment+0x10,equipment+0x800);strcpy((char *)(uintptr_t)(equipment+0x800),raw==1?"Weapon_Shield_001":"Weapon_Bow_001");ST64(equipment+0x4e0,equipment+0x900);memcpy((void *)(uintptr_t)(equipment+0x958),ownerMatrix,sizeof ownerMatrix);}
    capture_gear(&cpu,HEAP_BASE+0x1000);assert(!strcmp(local_gear[1].name,"Weapon_Bow_001")&&!strcmp(local_gear[2].name,"Weapon_Shield_001"));local_equipped[1]=local_equipped[2]=0;
    ownerMatrix[3]=NAN;memcpy((void *)(uintptr_t)(source+0x958),ownerMatrix,sizeof ownerMatrix);
    capture_gear(&cpu,HEAP_BASE+0x1000);assert(!local_gear[0].name[0]&&local_gear[0].matrix[0]==1&&isfinite(local_gear[0].matrix[3])&&local_ready);
    memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),initial_local,sizeof initial_local);
    // Parse one atomic six-slot frame and reject a late invalid matrix without
    // changing the already queued pose or any slot.
    Desired previous_visuals=desired;char *atomic[94];memcpy(atomic,pose,sizeof pose);atomic[2]="2";
    for(int i=0;i<6;i++){int at=16+i*13;atomic[at]=i==3?"Armor_Default_Head":"-";for(int j=0;j<12;j++)atomic[at+j+1]=j==0||j==5||j==10?"1":"0";}
    unsigned queued_calls=calls;r.failed=false;remote(94,atomic,&r);assert(!r.failed&&calls==queued_calls&&desired.seq==2&&!strcmp(desired.gear[3].name,"Armor_Default_Head"));
    Desired queued=desired;atomic[2]="3";atomic[93]="nan";r.failed=false;remote(94,atomic,&r);assert(r.failed&&!memcmp(&desired,&queued,sizeof queued));desired=previous_visuals;r.failed=false;
    // Change one slot to a failed model. The failed piece stays bounded and
    // the main player remains active with the other pieces.
    char *pose2[16];memcpy(pose2,pose,sizeof pose2);pose2[2]="3";remote(16,pose2,&r);
    piece[2]="3";piece[3]="0";piece[4]="Weapon_Sword_002";receive_equipment(17,piece,&r);assert(!r.failed);reject_equipment=true;
    rt_hyrulem_tick(&cpu,0);unsigned after_failure=creates;rt_hyrulem_tick(&cpu,0);assert(published.active&&gear_visual[0].failed&&creates==after_failure&&published.equipment_active==2);reject_equipment=false;
    desired.visible=false;rt_hyrulem_tick(&cpu,0);assert(!actor);for(int i=0;i<6;i++)assert(!gear_visual[i].actor&&!gear_visual[i].spawning&&!LD8(gear_visual[i].storage?gear_visual[i].storage+INDEX+12:HLE_HEAP+INDEX+12));
    // A real material is observed by stable map identity. Pickup hooks only
    // queue a request; the post-job grant checks the exact inventory delta.
    hyrulem_reset();r.failed=false;open_session(2,start,&r);remote(16,pose,&r);
    float player_root[12]={1,0,0,3437.5f,0,1,0,235.9f,0,0,1,2101.5f};memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),player_root,sizeof player_root);
    g_site.hm_pouch_mgr=HEAP_BASE+0x20;uint64_t pouch=HEAP_BASE+0xa0000;ST64(g_site.hm_pouch_mgr,pouch);ST32(pouch+0x44498,1);ST32(pouch+0x78,1);ST32(pouch+0x90,419);
    ground_pouch_item=HEAP_BASE+0xf0000;ST32(ground_pouch_item+0x20,3);ST8(ground_pouch_item+0x25,1);ST64(ground_pouch_item+0x30,ground_pouch_item+0x80);strcpy((char *)(uintptr_t)(ground_pouch_item+0x80),"Item_Fruit_A");
    rt_hyrulem_tick(&cpu,0x7000001000);rt_hyrulem_tick(&cpu,0x7000001000);
    char *material_open[]={"hm_items_open","0123456789abcdef","0123456789abcdef01234567","3437.5","235.9","2101.5"};ground_open(6,material_open,&r);assert(!r.failed);
    uint64_t apple=HEAP_BASE+0x70000,applemap=HEAP_BASE+0x78000;ST8(apple+0x64,1);ST64(apple+0x10,apple+0x900);strcpy((char *)(uintptr_t)(apple+0x900),"Item_Fruit_A");ST64(apple+0x7c8,applemap);ST32(applemap+0x58,123);
    player_root[3]=3439.5f;memcpy((void *)(uintptr_t)(apple+0x398),player_root,sizeof player_root);for(int j=0;j<3;j++){float v=player_root[j*4+3];memcpy((void *)(uintptr_t)(applemap+0x5c+j*4),&v,4);}
    ST64(head,apple+0x7b8);ST64(apple+0x7b8,head);ST32(creator+0x48,1);ground_scan_at=0;
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(ground_seen_count==1&&ground_seen[0].hash==123);
    char *material[]={"hm_item_state","0123456789abcdef","0123456789abcdef01234567","1","Item_Fruit_A","123","3439.5","235.9","2101.5","3439.5","235.9","2101.5","1","1","0","0","0","-","0","0","0","0"};ground_update(22,material,&r);assert(!r.failed);ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    uint64_t accessor=HEAP_BASE+0xf1000,safe=HEAP_BASE+0xf1100;ST64(accessor+8,apple);ST64(safe+8,apple+0x900);Cpu pickup=before;pickup.x[0]=accessor;pickup.x[1]=safe;Cpu pickup_before=pickup;unsigned grant_before=inventory_grants,calls_before=calls;
    assert(rt_hyrulem_item_pickup(&pickup));assert(!memcmp(&pickup,&pickup_before,sizeof pickup)&&calls==calls_before&&inventory_grants==grant_before);
    assert(ground_events[0].request==1&&ground_events[0].id==1);assert(rt_hyrulem_item_pickup(&pickup)&&ground_next_request==1);
    material[14]="1";material[15]="1";ground_update(22,material,&r);ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    assert(inventory_grants==grant_before+1&&LD32(ground_pouch_item+0x20)==4&&ground_receipts[0].request==1&&ground_receipts[0].result==1);
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(inventory_grants==grant_before+1); // lost receipt cannot grant twice
    assert(!memcmp(&cpu,&before,sizeof cpu));
    GroundState attempt={0};strcpy(attempt.source.name,"Item_Fruit_A");inventory_full=true;assert(ground_grant(&cpu,&attempt,0x7000001000,ground_replica[0].visual.storage)==-1&&inventory_grants==grant_before+1);inventory_full=false;
    inventory_ambiguous=true;assert(ground_grant(&cpu,&attempt,0x7000001000,ground_replica[0].visual.storage)==0);inventory_ambiguous=false;ST32(ground_pouch_item+0x20,5);assert(!memcmp(&cpu,&before,sizeof cpu));
    // Offline cleanup unpins a genuine map object without deleting it. Only
    // our replica may be deleted. A non-whitelisted actor is never intercepted.
    uint64_t untouched=HEAP_BASE+0x74000,untouched_map=HEAP_BASE+0x7a000;ST8(untouched+0x64,1);ST64(untouched+0x10,untouched+0x900);strcpy((char *)(uintptr_t)(untouched+0x900),"Item_Fruit_A");ST64(untouched+0x7c8,untouched_map);ST32(untouched_map+0x58,124);memcpy((void *)(uintptr_t)(untouched+0x398),player_root,sizeof player_root);for(int j=0;j<3;j++){float v=player_root[j*4+3];memcpy((void *)(uintptr_t)(untouched_map+0x5c+j*4),&v,4);}ST64(head,untouched+0x7b8);ST64(untouched+0x7b8,head);ground_scan_at=0;ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    // A replica can join the creator list before its async handle is drained.
    // Identify it from our handle, not from its position or a guessed name.
    char *early_material[22];memcpy(early_material,material,sizeof early_material);early_material[3]="2";early_material[4]="Item_PlantGet_A";early_material[5]="125";early_material[12]="999";early_material[13]="0";early_material[14]="0";early_material[15]="0";ground_update(22,early_material,&r);ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    uint64_t early=ground_pending_actor(ground_replica[1].visual.storage);assert(early&&ground_replica[1].visual.spawning&&!ground_replica[1].visual.actor);memcpy((void *)(uintptr_t)(early+0x398),player_root,sizeof player_root);
    ST64(head,early+0x7b8);ST64(early+0x7b8,untouched+0x7b8);ST64(untouched+0x7b8,head);ST32(creator+0x48,2);ground_scan_at=0;ground_scan(&cpu,HEAP_BASE+0x1000,&ground_live);
    for(int j=0;j<GROUND_MAX;j++)assert(ground_local[j].actor!=early);
    ST64(accessor+8,early);ST64(safe+8,early+0xa00);calls_before=calls;assert(rt_hyrulem_item_pickup(&pickup)&&ground_events[1].id==2&&calls==calls_before);
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    // Server preparation names the identity before an inventory debit. A drop
    // becomes publishable only after exactly one apple was removed.
    char *drop_state[22];memcpy(drop_state,material,sizeof drop_state);drop_state[3]="3";drop_state[5]="0";drop_state[12]="1";drop_state[13]="1";drop_state[14]="4";drop_state[15]="0";drop_state[16]="1";drop_state[17]="0123456789abcdef";ground_update(22,drop_state,&r);assert(!r.failed);
    char *drop_command[]={"hm_item_drop","0123456789abcdef","0123456789abcdef01234567","3","1"};ground_drop_command(5,drop_command,&r);assert(!r.failed);unsigned debit_before=inventory_debits;uint32_t apples_before=LD32(ground_pouch_item+0x20);
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(inventory_debits==debit_before);
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(ground_drop_last_result==1&&ground_drop_last_request==1&&inventory_debits==debit_before+1&&LD32(ground_pouch_item+0x20)==apples_before-1);
    ground_drop_command(5,drop_command,&r);ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(inventory_debits==debit_before+1&&!memcmp(&cpu,&before,sizeof cpu));
    // An uncertain debit followed by denied cleanup is terminal: subsequent
    // ticks may clean up, but cannot touch the inventory a second time.
    char *uncertain_state[22];memcpy(uncertain_state,drop_state,sizeof uncertain_state);uncertain_state[3]="4";uncertain_state[12]="2";ground_update(22,uncertain_state,&r);
    char *uncertain_command[]={"hm_item_drop","0123456789abcdef","0123456789abcdef01234567","4","2"};ground_drop_command(5,uncertain_command,&r);ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    remove_ambiguous=true;ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);remove_ambiguous=false;assert(ground_drop_last_result==2&&ground_drops[1].active&&ground_drops[1].terminal==2&&inventory_debits==debit_before+2);
    ST32(ground_pouch_item+0x20,apples_before-2);for(int j=0;j<3;j++)ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(inventory_debits==debit_before+2&&ground_drops[1].active);
    deny_lease=false;ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(!ground_drops[1].active&&inventory_debits==debit_before+2);
    GroundSource other_drop=ground_drops[0].source;other_drop.origin[0]='f';assert(!ground_source_equal(&other_drop,&ground_drops[0].source));
    uint64_t natural=HEAP_BASE+0x7b000;ST8(natural+0x64,1);ST64(natural+0x10,natural+0x900);strcpy((char *)(uintptr_t)(natural+0x900),"Item_Fruit_A");memcpy((void *)(uintptr_t)(natural+0x398),player_root,sizeof player_root);ST64(natural+0x7c8,0);ST64(head,natural+0x7b8);ST64(natural+0x7b8,head);ST32(creator+0x48,1);ground_scan_at=0;ground_scan(&cpu,HEAP_BASE+0x1000,&ground_live);for(int j=0;j<GROUND_MAX;j++)assert(ground_local[j].actor!=natural);
    ST64(accessor+8,natural);ST64(safe+8,natural+0x900);assert(!rt_hyrulem_item_pickup(&pickup));
    char *material_close[]={"hm_items_close","0123456789abcdef","0123456789abcdef01234567"};ground_close(3,material_close,&r);unsigned deleted=deletes;ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);assert(deletes==deleted+2&&LD8(untouched+0x64)==1&&!rt_hyrulem_item_pickup(&pickup));
    // Reopen from Hateno, then walk on a different part of the map. The
    // native pickup gate and scanner must use the current Link position.
    char *roaming_open[]={"hm_items_open","0123456789abcdef","0123456789abcdef01234567","3437.5","235.9","2101.5","player"};
    ground_open(7,roaming_open,&r);assert(!r.failed);
    float previous_link[12];memcpy(previous_link,(void *)(uintptr_t)(HEAP_BASE+0x1398),sizeof previous_link);
    float moved[12]={1,0,0,-3607,0,1,0,286,0,0,1,-1805};
    memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),moved,sizeof moved);
    memcpy((void *)(uintptr_t)(untouched+0x398),moved,sizeof moved);
    ST64(head,untouched+0x7b8);ST64(untouched+0x7b8,head);ST32(creator+0x48,1);
    ground_scan_at=0;ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    assert(ground_seen_count==1&&ground_seen[0].hash==124&&ground_mode.zone[0]==-3607);
    ST64(accessor+8,untouched);ST64(safe+8,untouched+0x900);
    assert(rt_hyrulem_item_pickup(&pickup));
    char *retirable[22];memcpy(retirable,material,sizeof retirable);retirable[14]="0";retirable[15]="0";
    ground_update(22,retirable,&r);
    char *forget[]={"hm_item_forget","0123456789abcdef","0123456789abcdef01234567","1"};
    ground_forget(4,forget,&r);assert(!r.failed&&ground_retire[0]==1&&ground_state[0].id==1);
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    for(int j=0;j<GROUND_EVENTS;j++)if(ground_events[j].request&&!ground_events[j].done){
        char reqtext[32];snprintf(reqtext,sizeof reqtext,"%llu",(unsigned long long)ground_events[j].request);
        char *deny[]={"hm_item_deny","0123456789abcdef","0123456789abcdef01234567",reqtext};ground_deny(4,deny,&r);
    }
    ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    assert(ground_retire[0]==2&&!ground_state[0].id&&!ground_replica[0].granted);
    ground_forget(4,forget,&r);assert(!ground_retire[0]&&!memcmp(&cpu,&before,sizeof cpu));
    ground_close(3,material_close,&r);ground_tick(&cpu,HEAP_BASE+0x1000,0x7000001000);
    memcpy((void *)(uintptr_t)(HEAP_BASE+0x1398),previous_link,sizeof previous_link);
    memcpy((void *)(uintptr_t)(untouched+0x398),player_root,sizeof player_root);
    // Two named bones in a synthetic SDK model. Capture must remove the
    // source root, then replay using another root without choosing an AS leaf.
    uint64_t owner=HEAP_BASE+0x1000,component=HEAP_BASE+0xa0000,units=component+0x100,unit=component+0x200,model=component+0x400,model_vt=component+0x700,sk=component+0x900,res=component+0xa00,dict=component+0xb00,palette=component+0xc00;
    ST64(owner+0x4e0,component);ST32(component+0x38,1);ST64(component+0x40,units);ST64(units,unit);ST64(unit,model);ST64(model,model_vt);ST64(model+0x118,sk);ST64(sk,res);ST64(sk+0x20,palette);ST16(res+0x3c,2);ST64(res+0x10,dict);
    ST64(model_vt+0x30,g_site.hm_bone_count);ST64(model_vt+0x40,g_site.hm_bone_name);ST64(model_vt+0x68,g_site.hm_bone_set);ST64(model_vt+0x70,g_site.hm_bone_get);
    ST64(dict+0x20,component+0xd00);ST64(dict+0x30,component+0xd50);strcpy((char *)(uintptr_t)(component+0xd02),"Root");strcpy((char *)(uintptr_t)(component+0xd52),"Weapon_R");
    float origin[3]={100,200,300};for(int i=0;i<2;i++){float matrix[16]={1,0,0,0,0,1,0,0,0,0,1,0,100+i,200,300,0};memcpy((void *)(uintptr_t)(palette+i*64),matrix,sizeof matrix);}
    PoseFrame captured;pose_capture(owner,origin,1.57079632679f,&captured);assert(captured.count==2&&captured.bones[1].name==pose_hash("Weapon_R"));assert(fabsf(captured.bones[1].matrix[3])<.0001f&&fabsf(captured.bones[1].matrix[11]-1)<.0001f);
    Desired skin={0};skin.pose=captured;skin.x=10;skin.y=20;skin.z=30;unsigned before_bones=bone_writes;assert(pose_apply(&cpu,owner,storage,&skin,0)==2&&bone_writes==before_bones+2);assert(fabsf(last_bone_matrix[3]-10)<.0001f&&fabsf(last_bone_matrix[7]-20)<.0001f&&fabsf(last_bone_matrix[11]-31)<.0001f);assert(!memcmp(&cpu,&before,sizeof cpu));
    // A correct ModelGroup root must not leave the SDK palette at its old
    // physical location. Preserve the relative bone offset through rotation.
    float world_target[12]={0,0,1,10,0,1,0,20,-1,0,0,30};
    before_bones=bone_writes;actor_world_rebase(&cpu,owner,storage,world_target);
    assert(bone_writes==before_bones+2&&fabsf(last_bone_matrix[3]-10)<.001f&&fabsf(last_bone_matrix[7]-20)<.001f&&fabsf(last_bone_matrix[11]-29)<.001f);
    assert(!memcmp((void *)(uintptr_t)(storage+MATRIX),world_target,48)&&!memcmp(&cpu,&before,sizeof cpu));
    // A nonuniform/scaled root requires the affine inverse, not a transpose.
    float scaled_from[12]={2,0,0,100,0,3,0,200,0,0,4,300},delta[12];
    assert(actor_world_delta(scaled_from,world_target,delta));float mapped[3]={0};
    for(unsigned row=0;row<3;row++){mapped[row]=delta[row*4+3];for(unsigned col=0;col<3;col++)mapped[row]+=delta[row*4+col]*scaled_from[col*4+3];assert(fabsf(mapped[row]-world_target[row*4+3])<.001f);}
    scaled_from[0]=0;assert(!actor_world_delta(scaled_from,world_target,delta));
    ST64(model_vt+0x70,0);before_bones=bone_writes;actor_world_rebase(&cpu,owner,storage,world_target);assert(bone_writes==before_bones);ST64(model_vt+0x70,g_site.hm_bone_get);
    char encoded[POSE_B64_MAX+1];pose_encode(&captured,encoded);PoseFrame decoded;assert(pose_decode(encoded,&decoded)&&!memcmp(&captured,&decoded,4+captured.count*sizeof(PoseBone)));char save=encoded[4];encoded[4]='!';assert(!pose_decode(encoded,&decoded));encoded[4]=save;
    captured.bones[1].name=captured.bones[0].name;pose_encode(&captured,encoded);assert(!pose_decode(encoded,&decoded));captured.bones[1].name=pose_hash("Weapon_R");pose_encode(&captured,encoded);
    char *skeletal_frame[95];memcpy(skeletal_frame,atomic,sizeof atomic);skeletal_frame[2]="3";skeletal_frame[93]="0";skeletal_frame[94]=encoded;r.failed=false;remote(95,skeletal_frame,&r);assert(!r.failed&&desired.pose.count==2);Desired intact=desired;skeletal_frame[2]="4";skeletal_frame[94]="%%%=";remote(95,skeletal_frame,&r);assert(r.failed&&!memcmp(&intact,&desired,sizeof intact));
    ST64(model_vt+0x70,0);pose_capture(owner,origin,0,&captured);assert(captured.count==0&&pose_apply(&cpu,owner,storage,&skin,0)==0);ST64(model_vt+0x70,g_site.hm_bone_get);
    float invalid=NAN;memcpy((void *)(uintptr_t)palette,&invalid,4);pose_capture(owner,origin,0,&captured);assert(captured.count==0&&pose_probe.skipped==1&&!strcmp(pose_probe.reason,"invalid-bone-matrix"));
    // Clothing comes from personal pouch entries, not the weapon actor getter.
    strcpy((char *)(uintptr_t)(ground_pouch_item+0x100),"Armor_001_Upper");ST64(ground_pouch_item+0x30,ground_pouch_item+0x100);ST32(ground_pouch_item+0x18,5);ST8(ground_pouch_item+0x24,1);ST8(ground_pouch_item+0x25,1);armor_read_at=0;capture_armor(&cpu);assert(armor_from_pouch&&!strcmp(local_armor[1],"Armor_001_Upper")&&!strcmp(local_armor[0],"Armor_Default_Head"));
    // Raw armor types 4/5/6 are distinct; arrow type 2 and unequipped
    // entries must never become remote armor, even when names look plausible.
    for(int type=4;type<=6;type++){const char *suffix=type==4?"Head":type==5?"Upper":"Lower";snprintf((char *)(uintptr_t)(ground_pouch_item+0x100),48,"Armor_001_%s",suffix);ST32(ground_pouch_item+0x18,type);armor_read_at=0;capture_armor(&cpu);assert(!strcmp(local_armor[type-4],(char *)(uintptr_t)(ground_pouch_item+0x100)));}
    ST32(ground_pouch_item+0x18,2);armor_read_at=0;capture_armor(&cpu);assert(!strcmp(local_armor[2],"Armor_Default_Lower"));ST32(ground_pouch_item+0x18,6);ST8(ground_pouch_item+0x24,0);armor_read_at=0;capture_armor(&cpu);assert(!strcmp(local_armor[2],"Armor_Default_Lower"));ST8(ground_pouch_item+0x24,1);
    #include "test_generation7.inc"
    #include "test_preview11.inc"
    #include "test_preview12.inc"
    #include "test_preview14.inc"
    #include "test_world_actors.inc"
    #include "test_lifecycle.inc"
    // A refused stump must leave the genuine collider standing. A successful
    // retry removes only the matched tree and releases its native replacement.
    uint64_t standing=HEAP_BASE+0xe5000,tree_block=standing+0x1800;
    ST64(standing,vt);ST64(standing+0x68,tree_block);ST32(standing+0x60,5);
    ST32(tree_block+0x40,5);ST64(tree_block+0x48,standing);ST8(standing+0x64,1);
    ST32(standing+0x900,40);memcpy((void *)(uintptr_t)(standing+0x398),player_root,sizeof player_root);
    EnemyHealth felled={.hash=876};strcpy(felled.name,"Obj_TreeConiferous_A_02");
    for(int j=0;j<3;j++)felled.position[j]=player_root[j*4+3];
    uint64_t dt=deletes;tree_queue(standing,&felled);assert(replacement.original==standing);
    reject_equipment=true;tree_tick(&cpu,0x7000001000,true);reject_equipment=false;
    assert(!replacement.original&&deletes==dt&&LD8(standing+0x64)==1);
    replacement.retry_at=0;deny_lease=false;tree_queue(standing,&felled);tree_tick(&cpu,0x7000001000,true);
    assert(replacement.stump.spawning&&deletes==dt);
    tree_tick(&cpu,0x7000001000,true);
    assert(!replacement.original&&deletes==dt+1&&LD8(standing+0x64)==3&&!memcmp(&cpu,&before,sizeof cpu));
    // Distinct interiors can share coordinates, names and hashes.
    uint64_t scene_type=HEAP_BASE+0xf5000,scene_name=HEAP_BASE+0xf5100;
    g_site.hm_scene_type=scene_type;g_site.hm_scene_name=scene_name;
    ST64(scene_type+8,scene_type+0x20);ST64(scene_name+8,scene_name+0x20);
    strcpy((char *)(uintptr_t)(scene_type+0x20),"MainField");strcpy((char *)(uintptr_t)(scene_name+0x20),"F-5");
    char scene_result[65];assert(actor_scene_read(scene_result)&&!strcmp(scene_result,"MainField"));
    strcpy((char *)(uintptr_t)(scene_type+0x20),"CDungeon");strcpy((char *)(uintptr_t)(scene_name+0x20),"Dungeon000");
    assert(actor_scene_read(scene_result)&&!strcmp(scene_result,"CDungeon/Dungeon000"));
    strcpy((char *)(uintptr_t)(scene_name+0x20),"Dungeon001");
    assert(actor_scene_read(scene_result)&&!strcmp(scene_result,"CDungeon/Dungeon001"));
    snprintf(local_scene,sizeof local_scene,"%s",scene_result);
    WorldActor empty_rows[1]={0};char empty_world[ACTOR_WORLD_B64+1];actor_world_encode(empty_rows,0,empty_world);
    char *scoped_world[]={"hm_world_sample","0123456789abcdef","0123456789abcdef01234567","10","3","1","0","0","0","0",empty_world,"CDungeon/Dungeon000"};
    ActorWorldMode before_scene_mode=actor_world_mode;r.failed=false;actor_world_sample(12,scoped_world,&r);
    assert(!r.failed&&!memcmp(&before_scene_mode,&actor_world_mode,sizeof before_scene_mode));
    scoped_world[11]="CDungeon/Dungeon001";actor_world_sample(12,scoped_world,&r);
    assert(!r.failed&&!strcmp(actor_world_mode.scene,"CDungeon/Dungeon001"));
    puts("final skeleton: named-bone capture/replay, atomic validation, private armor and CPU preservation passed");
    puts("ground materials: deferred pickup, exact inventory delta, receipt idempotency and uncertain-write safety passed");
    puts("equipment isolation, transforms, bounded failures, cleanup and AS smoothing checks passed");
    puts("map-enemy identity, deferred life application and no-heal checks passed");
    puts("native adapter lifetime, CPU preservation and queue checks passed");
    return 0;
}
