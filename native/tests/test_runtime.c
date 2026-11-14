// Synthetic guest memory and callees: tests adapter lifetime/thread boundaries,
// not BOTW behavior. The real game/renderer still needs a two-instance playtest.
#include <assert.h>
#include <stdarg.h>
#include <sys/mman.h>
#include "../runtime/hyrulem.c"

struct ControlReply { bool failed; };
GameSites g_site;
const char *g_game_version="1.9.0";
static unsigned calls, creates, deletes, moves, plays;
static uint64_t mock_actor=HEAP_BASE+0x3000, retained=HEAP_BASE+0x4000;
static bool reject,fail_handle;
void *hle_alloc(uint64_t size) { assert(size==BYTES); return (void *)(uintptr_t)HLE_HEAP; }
void guest_call(Cpu *c,uint64_t fn) {
    calls++;
    if (fn==g_site.hm_create) { creates++; assert(c->x[3]==HLE_HEAP+HANDLE); c->x[0]=!reject; }
    else if (fn==g_site.hm_handle_ready) c->x[0]=1;
    else if (fn==g_site.hm_handle_failed) c->x[0]=fail_handle;
    else if (fn==g_site.hm_handle_get || fn==g_site.hm_handle_wake) c->x[0]=mock_actor;
    else if (fn==g_site.actor_index_assign) {
        ST64(c->x[0],retained); ST32(c->x[0]+8,7); ST8(c->x[0]+12,1);
        ST32(retained+0x40,7);ST64(retained+0x48,mock_actor);c->x[0]=1;
    } else if (fn==g_site.actor_index_resolve) {ST64(c->x[1]+8,mock_actor);c->x[0]=mock_actor;}
    else if (fn==g_site.actor_index_release) {ST64(c->x[0],0);ST32(c->x[0]+8,UINT32_MAX);ST8(c->x[0]+12,0);}
    else if (fn==g_site.actor_link_release) ST64(c->x[0]+8,0);
    else if (fn==g_site.hm_delete_actor) {assert(c->x[0]==mock_actor);deletes++;}
    else if (fn==g_site.hm_set_mtx) {assert(c->x[0]==mock_actor);assert(c->x[2]<=1);memcpy((void *)(uintptr_t)(mock_actor+0x398),(void *)(uintptr_t)c->x[1],48);moves++;}
    else if (fn==g_site.hm_play_as) {assert(c->x[0]==HEAP_BASE+0x5000);plays++;}
    // Deliberately clobber every register: the hook must preserve the caller.
    memset(c->v,0x5a,sizeof c->v);c->x[3]=0xdeadbeef;c->nzcv=0xabcdef;c->fpsr=91;
}
void control_register(const ControlCmd *cmds,int n) {assert(n==8);}
void control_field_str(ControlReply *r,const char *k,const char *v) {}
void control_field_int(ControlReply *r,const char *k,int64_t v) {}
void control_field_num(ControlReply *r,const char *k,double v) {}
void control_field_bool(ControlReply *r,const char *k,bool v) {}
void control_fail(ControlReply *r,const char *fmt,...) {r->failed=true;}

static void map_at(uint64_t a,size_t n) {assert(mmap((void *)(uintptr_t)a,n,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANONYMOUS|MAP_FIXED,-1,0)==(void *)(uintptr_t)a);}
int main(void) {
    setenv("HYRULEM_NATIVE_PREVIEW","1",1);
    map_at(HLE_HEAP,0x1000);map_at(HEAP_BASE,0x10000);
    uint64_t site=1;
    #define SITE(n) g_site.n=site++;
    GAME_SITES_OPTIONAL(SITE)
    #undef SITE
    g_site.player_info=HEAP_BASE;g_site.hm_creator=HEAP_BASE+8;g_site.hm_heap_mgr=HEAP_BASE+16;
    ST64(g_site.player_info,HEAP_BASE+0x100);ST64(HEAP_BASE+0x160,HEAP_BASE+0x1000);
    ST64(g_site.hm_creator,HEAP_BASE+0x200);ST64(g_site.hm_heap_mgr,HEAP_BASE+0x300);
    ST64(HEAP_BASE+0x328,HEAP_BASE+0x400);
    ST8(mock_actor+0x64,1);ST64(mock_actor+0x560,HEAP_BASE+0x5000);ST64(mock_actor+0x4e0,HEAP_BASE+0x6000);
    ST32(HEAP_BASE+0x5138,400);ST32(HEAP_BASE+0x50b8,2);ST64(HEAP_BASE+0x50c0,HEAP_BASE+0xb000);
    // The real guest entry receives a damage component with its owner at +0x18.
    ST64(HEAP_BASE+0x2018,HEAP_BASE+0x1000);
    Cpu cpu={0};cpu.x[0]=HEAP_BASE+0x2000;cpu.sp=HEAP_BASE+0xf000;cpu.v[3]=123;cpu.fpsr=77;Cpu before=cpu;
    ControlReply r={0};
    char *start[]={"hm_native_open","0123456789abcdef"};open_session(2,start,&r);assert(!r.failed && calls==0);
    char *pose[]={"hm_native_remote","0123456789abcdef","1","3434","236","2101","0.5","move","-","0","0","-","0","0"};
    remote(14,pose,&r);assert(!r.failed && calls==0); // control thread never touches the game
    cpu.x[0]=mock_actor;rt_hyrulem_tick(&cpu,0);assert(calls==0);cpu=before; // only local update may drain
    cpu.x[0]=HEAP_BASE+0x1000;rt_hyrulem_tick(&cpu,0);assert(calls==0);cpu=before; // regression: Actor != damage component
    rt_hyrulem_tick(&cpu,0x7000001000);assert(creates==1 && spawning);assert(!memcmp(&before,&cpu,sizeof cpu));
    spawning_at=now_ms()-16000; // Ready handle must survive a long menu/loading pause.
    rt_hyrulem_tick(&cpu,0x7000001000);assert(published.active && moves==2 && plays==2);assert(!memcmp(&before,&cpu,sizeof cpu));
    // Later poses must move the actor and keep both animation tracks.
    Desired moving=desired;moving.x+=10;moving.yaw=1;moving.seq++;snprintf(moving.tracks[0].name,64,"CutS1");moving.tracks[0].sequence=1;moving.tracks[0].force=true;desired=moving;
    rt_hyrulem_tick(&cpu,0x7000001000);assert(read_float(mock_actor+0x3a4)==3444&&plays==3);assert(!memcmp(&before,&cpu,sizeof cpu));
    rt_hyrulem_tick(&cpu,0x7000001000);assert(plays==3); // no restart from repeated packet
    ST64(HEAP_BASE+0x50d8,HEAP_BASE+0x1000);strcpy((char *)(uintptr_t)(HEAP_BASE+0xc000),"CutS1");ST64(HEAP_BASE+0xc110,HEAP_BASE+0xc000);ST8(HEAP_BASE+0xc121,1);
    Cpu request=before;request.x[0]=HEAP_BASE+0x5000;request.x[1]=HEAP_BASE+0xc100;request.x[2]=0;request.x[3]=0;request.x[4]=1;Cpu saved=request;
    rt_hyrulem_capture_as(&request);assert(capture_count==1&&!strcmp(local_tracks[0].name,"CutS1"));assert(!memcmp(&request,&saved,sizeof request));
    ST64(HEAP_BASE+0x50d8,mock_actor);rt_hyrulem_capture_as(&request);assert(capture_count==1); // remote cannot feed back
    r.failed=false;remote(14,pose,&r);assert(r.failed); // replay cannot resurrect or move
    ST32(HEAP_BASE+0x5138,0);rt_hyrulem_tick(&cpu,0);assert(published.active&&published.visual_only&&!published.animation_ready&&deletes==0);ST32(HEAP_BASE+0x5138,400);
    char *wrong[]={"hm_native_clear","ffffffffffffffff","2"};r.failed=false;clear_remote(3,wrong,&r);assert(r.failed);
    char *clear[]={"hm_native_clear","0123456789abcdef","3"};r.failed=false;clear_remote(3,clear,&r);assert(!r.failed && deletes==0);
    rt_hyrulem_tick(&cpu,0);assert(deletes==1 && !actor && !published.active);
    hyrulem_reset();assert(!storage && !actor && !desired.session[0]);
    // A rejected native creation is reported once, without an allocation/retry loop.
    reject=true;r.failed=false;open_session(2,start,&r);remote(14,pose,&r);
    rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);unsigned count=creates;rt_hyrulem_tick(&cpu,0);assert(failed && creates==count && !strcmp(published.state,"error"));
    desired.received=now_ms()-3001;rt_hyrulem_tick(&cpu,0);assert(!strcmp(published.state,"error")&&published.first_error[0]);
    // One fallback attempt can restore the model without executing Player AS.
    hyrulem_reset();reject=false;fail_handle=true;r.failed=false;open_session(2,start,&r);remote(14,pose,&r);rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);assert(fallback_tried&&!failed);
    fail_handle=false;
    reject=false;unsigned previous_plays=plays;rt_hyrulem_tick(&cpu,0);rt_hyrulem_tick(&cpu,0);
    assert(published.active&&published.resource_fallback&&published.visual_only&&plays==previous_plays&&spawn_attempts==2);
    assert(strstr((char *)(uintptr_t)(storage+NAME),"_Fallback"));desired.x=3450;desired.seq++;rt_hyrulem_tick(&cpu,0);assert(read_float(mock_actor+0x3a4)==3450&&plays==previous_plays);

    // A native map-enemy observation is keyed by placement, never a process
    // pointer. Received health waits for the game thread and cannot heal.
    hyrulem_reset();reject=false;r.failed=false;open_session(2,start,&r);remote(14,pose,&r);
    map_at(MAIN_BASE,0x1000);g_site.main_data=MAIN_BASE+0x1000;
    ST32(MAIN_BASE+0x100,0x91240000);ST32(MAIN_BASE+0x104,0xd65f03c0); // synthetic add x0,x0,#0x900; ret
    uint64_t enemy=HEAP_BASE+0x7000,map=HEAP_BASE+0x8000,vt=HEAP_BASE+0xa000;
    strcpy((char *)(uintptr_t)(HEAP_BASE+0x9000),"Enemy_Bokoblin_Junior");
    ST64(enemy,vt);ST64(vt+0x2b8,MAIN_BASE+0x100);ST64(enemy+0x10,HEAP_BASE+0x9000);
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
    puts("map-enemy identity, deferred life application and no-heal checks passed");
    puts("native adapter lifetime, CPU preservation and queue checks passed");
    return 0;
}
