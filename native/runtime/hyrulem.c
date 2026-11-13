// HyruleM's experimental remote actor adapter. Original implementation; no
// game code or resources. All guest calls run from Link's native update hook.
#include "hle.h"
#include "game_sites.h"
#include "control.h"
#include <errno.h>
#include <math.h>
#include <pthread.h>
#include <string.h>
#include <time.h>

typedef struct {
    char session[17];
    uint64_t seq, received;
    float x, y, z, yaw;
    int animation;
    bool visible;
} Desired;
typedef struct {
    char state[32], error[96];
    uint64_t applied, received;
    bool active;
} Published;
static pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;
static Desired desired;
static Published published;
// Game-thread owned; never dereferenced by command handlers.
static uint64_t storage, actor, spawning_at, applied;
static char session[17];
static int animation = -1;
static bool spawning, failed;
enum { HANDLE=0, INDEX=0x10, LOCAL=0x20, PARAMS=0x40, SSTR=0x120,
       KEY=0x130, NAME=0x140, VECTOR=0x180, MATRIX=0x190, ASNAME=0x1d0, BYTES=0x220 };

static uint64_t now_ms(void) {
    struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t);
    return (uint64_t)t.tv_sec*1000 + t.tv_nsec/1000000;
}
static bool range(uint64_t a, uint64_t n) { return a >= GUEST_LO && a < GUEST_HI && n <= GUEST_HI-a; }
static bool enabled(void) { const char *e=ENV("HYRULEM_NATIVE_PREVIEW"); return e && !strcmp(e,"1"); }
static bool supported(void) {
    return enabled() && !strcmp(g_game_version,"1.9.0") &&
        g_site.hm_creator && g_site.hm_heap_mgr && g_site.hm_create &&
        g_site.hm_handle_ready && g_site.hm_handle_failed && g_site.hm_handle_get &&
        g_site.hm_handle_wake && g_site.hm_handle_delete && g_site.hm_param_add &&
        g_site.hm_set_mtx && g_site.hm_delete_actor && g_site.hm_play_as &&
        g_site.actor_index_assign && g_site.actor_index_resolve &&
        g_site.actor_index_release && g_site.actor_link_release;
}
static uint64_t call(Cpu *original, uint64_t fn, uint64_t a, uint64_t b, uint64_t c,
                     uint64_t d, uint64_t e, uint64_t f, uint64_t g) {
    Cpu scratch=*original;
    scratch.x[0]=a; scratch.x[1]=b; scratch.x[2]=c; scratch.x[3]=d;
    scratch.x[4]=e; scratch.x[5]=f; scratch.x[6]=g;
    guest_call(&scratch,fn); return scratch.x[0];
}
static void publish(const char *state, const char *error, uint64_t seq, bool active) {
    pthread_mutex_lock(&mutex);
    snprintf(published.state,sizeof published.state,"%s",state);
    snprintf(published.error,sizeof published.error,"%s",error?error:"");
    published.applied=seq; published.received=desired.seq; published.active=active;
    pthread_mutex_unlock(&mutex);
}
static void release_local(Cpu *c) { call(c,g_site.actor_link_release,storage+LOCAL,0,0,0,0,0,0); }
static uint64_t resolve(Cpu *c) {
    if (!actor || !LD8(storage+INDEX+12) || LD32(storage+INDEX+8)==UINT32_MAX) return 0;
    uint64_t block=LD64(storage+INDEX);
    if (!range(block,0x54) || LD32(block+0x40)!=LD32(storage+INDEX+8) || LD64(block+0x48)!=actor) return 0;
    uint64_t a=call(c,g_site.actor_index_resolve,storage+INDEX,storage+LOCAL,0,0,0,0,0);
    if (a!=actor || !range(a,0x600) || LD64(storage+LOCAL+8)!=a || LD8(a+0x64)!=1) {
        release_local(c); return 0;
    }
    return a;
}
static void remove_remote(Cpu *c) {
    if (!storage) return;
    if (spawning) call(c,g_site.hm_handle_delete,storage+HANDLE,0,0,0,0,0,0);
    uint64_t a=resolve(c);
    if (a) { call(c,g_site.hm_delete_actor,a,0,0,0,0,0,0); release_local(c); }
    call(c,g_site.actor_index_release,storage+INDEX,0,0,0,0,0,0);
    actor=0; spawning=false; animation=-1;
}
static bool prepare(Cpu *c, Desired *d, uint64_t vt) {
    uint64_t creator=LD64(g_site.hm_creator), manager=LD64(g_site.hm_heap_mgr);
    if (!range(creator,0x60) || !range(manager,0x30) || LD8(creator+0x59)) return false;
    uint64_t heap=LD64(manager+0x28);
    if (!range(heap,8)) return false;
    memset((void *)(uintptr_t)(storage+HANDLE),0,16);
    memset((void *)(uintptr_t)(storage+PARAMS),0,0xd8);
    float xyz[3]={d->x,d->y,d->z};
    memcpy((void *)(uintptr_t)(storage+VECTOR),xyz,sizeof xyz);
    ST64(storage+SSTR,vt); ST64(storage+SSTR+8,storage+KEY);
    // InstParamPack::Buffer: position starts after its u16 count/s16 cursor.
    call(c,g_site.hm_param_add,storage+PARAMS+0x10,storage+VECTOR,storage+SSTR,12,4,0,0);
    if (!call(c,g_site.hm_create,creator,storage+NAME,heap,storage+HANDLE,storage+PARAMS,0,1)) return false;
    spawning=true; spawning_at=now_ms(); return true;
}
static void set_pose(Cpu *c, uint64_t a, Desired *d, uint64_t vt) {
    float s=sinf(d->yaw), co=cosf(d->yaw);
    float matrix[12]={co,0,s,d->x, 0,1,0,d->y, -s,0,co,d->z};
    memcpy((void *)(uintptr_t)(storage+MATRIX),matrix,sizeof matrix);
    call(c,g_site.hm_set_mtx,a,storage+MATRIX,1,1,0,0,0);
    if (animation==d->animation) return;
    uint64_t as=LD64(a+0x560);
    if (!range(as,8)) return;
    const char *name=d->animation==1?"Move":d->animation==2?"CutS1":"Wait";
    snprintf((char *)(uintptr_t)(storage+ASNAME),48,"%s",name);
    ST64(storage+SSTR,vt); ST64(storage+SSTR+8,storage+ASNAME);
    Cpu scratch=*c;
    scratch.x[0]=as; scratch.x[1]=storage+SSTR;
    scratch.x[2]=0; scratch.x[3]=0; scratch.x[4]=1;
    float blend=-1.f; scratch.v[0]=scratch.v[1]=0;
    memcpy(&scratch.v[0],&blend,sizeof blend); memcpy(&scratch.v[1],&blend,sizeof blend);
    guest_call(&scratch,g_site.hm_play_as);
    animation=d->animation;
}

// Hooked only at the original local Player update. Full Cpu state is preserved
// by each call; no actor work ever runs on the presenter/control thread.
void rt_hyrulem_tick(Cpu *c, uint64_t safe_string_vtable) {
    if (!supported() || !range(g_site.player_info,8)) return;
    uint64_t info=LD64(g_site.player_info);
    if (!range(info,0x68) || c->x[0]!=LD64(info+0x60)) return;
    Desired d;
    pthread_mutex_lock(&mutex); d=desired; pthread_mutex_unlock(&mutex);
    if (!storage) {
        if (!d.session[0]) return;
        storage=(uint64_t)(uintptr_t)hle_alloc(BYTES);
        if (!range(storage,BYTES)) { publish("error","guest-storage",0,false); return; }
        ST32(storage+INDEX+8,UINT32_MAX);
        strcpy((char *)(uintptr_t)(storage+NAME),"HyruleM_RemoteLink");
        strcpy((char *)(uintptr_t)(storage+KEY),"@P");
    }
    if (strcmp(session,d.session)) {
        remove_remote(c); snprintf(session,sizeof session,"%s",d.session);
        applied=0; failed=false;
    }
    bool stale=d.received && now_ms()-d.received>2500;
    if (!d.visible || stale) {
        remove_remote(c); applied=d.seq;
        publish(stale?"expired":"idle","",applied,false); return;
    }
    if (failed) return; // An engine rejection requires a new session, never a spawn loop.
    if (spawning) {
        if (call(c,g_site.hm_handle_failed,storage+HANDLE,0,0,0,0,0,0) || now_ms()-spawning_at>15000) {
            remove_remote(c); failed=true; publish("error","native-spawn-failed-or-timeout",applied,false); return;
        }
        if (!call(c,g_site.hm_handle_ready,storage+HANDLE,0,0,0,0,0,0)) { publish("spawning","",applied,false); return; }
        uint64_t a=call(c,g_site.hm_handle_get,storage+HANDLE,0,0,0,0,0,0);
        if (!range(a,0x600) || a==c->x[0] ||
            !call(c,g_site.actor_index_assign,storage+INDEX,a,1,0,0,0,0)) {
            remove_remote(c); failed=true; publish("error","native-reference-failed",applied,false); return;
        }
        actor=a;
        uint64_t awake=call(c,g_site.hm_handle_wake,storage+HANDLE,0,0,0,0,0,0);
        spawning=false;
        if (awake!=a) { remove_remote(c); failed=true; publish("error","native-wake-failed",applied,false); return; }
    }
    if (!actor) {
        if (!prepare(c,&d,safe_string_vtable)) { failed=true; publish("error","native-spawn-rejected-check-private-overlay",applied,false); return; }
        publish("spawning","",applied,false); return;
    }
    uint64_t a=resolve(c);
    if (!a) { remove_remote(c); failed=true; publish("error","remote-unloaded-open-new-session",applied,false); return; }
    set_pose(c,a,&d,safe_string_vtable); release_local(c);
    applied=d.seq; publish("active","",applied,true);
}

RT_ON_RESET(hyrulem_reset) {
    pthread_mutex_lock(&mutex);
    memset(&desired,0,sizeof desired); memset(&published,0,sizeof published);
    pthread_mutex_unlock(&mutex);
    storage=actor=spawning_at=applied=0; session[0]=0;
    animation=-1; spawning=failed=false;
}
static bool token(const char *s) {
    if (strlen(s)!=16) return false;
    for (int i=0;i<16;i++) if (!((s[i]>='0'&&s[i]<='9')||(s[i]>='a'&&s[i]<='f'))) return false;
    return true;
}
static bool integer(const char *s, uint64_t *n) {
    if (!s[0] || s[0]=='-' || s[0]=='+') return false;
    errno=0; char *end; *n=strtoull(s,&end,10); return !errno && !*end && *n;
}
static bool number(const char *s, float *out, float limit) {
    char *end; errno=0; *out=strtof(s,&end);
    return *s && !errno && !*end && isfinite(*out) && fabsf(*out)<=limit;
}
static void caps(int argc,char **argv,ControlReply *r) {
    control_field_int(r,"abi",1); control_field_str(r,"runtime","SavageWind-HyruleM");
    control_field_str(r,"game_version",g_game_version); control_field_bool(r,"simulated",false);
    control_field_int(r,"preview_abi",1); control_field_bool(r,"native_preview",supported());
    // These are end-to-end Gen1 capabilities, not addresses found in a binary.
    // Keep them false until combat, pickups and AI are wired and play-tested.
    const char *names[]={"native_actors","animation","collision","personal_vitals","personal_inventory",
        "pickup_hook","damage_hook","enemy_authority","enemy_target","world_identity"};
    for (unsigned i=0;i<sizeof names/sizeof *names;i++) control_field_bool(r,names[i],false);
}
static void open_session(int argc,char **argv,ControlReply *r) {
    if (argc!=2 || !token(argv[1]) || !supported()) { control_fail(r,"Native preview disabled, incompatible or invalid session"); return; }
    pthread_mutex_lock(&mutex);
    if (strcmp(desired.session,argv[1])) { memset(&desired,0,sizeof desired); snprintf(desired.session,sizeof desired.session,"%s",argv[1]); }
    pthread_mutex_unlock(&mutex); control_field_bool(r,"queued",true);
}
static void remote(int argc,char **argv,ControlReply *r) {
    Desired next={0};
    if (argc!=8 || !supported() || !token(argv[1]) || !integer(argv[2],&next.seq) ||
        !number(argv[3],&next.x,100000) || !number(argv[4],&next.y,50000) ||
        !number(argv[5],&next.z,100000) || !number(argv[6],&next.yaw,10000)) {
        control_fail(r,"Invalid native remote pose"); return;
    }
    next.animation=!strcmp(argv[7],"move")?1:!strcmp(argv[7],"attack")?2:!strcmp(argv[7],"wait")?0:-1;
    if (next.animation<0) { control_fail(r,"Unsupported animation"); return; }
    pthread_mutex_lock(&mutex);
    bool ok=!strcmp(desired.session,argv[1]) && next.seq>desired.seq;
    if (ok) { snprintf(next.session,sizeof next.session,"%s",argv[1]); next.visible=true; next.received=now_ms(); desired=next; }
    pthread_mutex_unlock(&mutex);
    if (!ok) { control_fail(r,"Wrong session or stale sequence"); return; }
    control_field_int(r,"queued_sequence",next.seq);
}
static void clear_remote(int argc,char **argv,ControlReply *r) {
    uint64_t seq;
    if (argc!=3 || !token(argv[1]) || !integer(argv[2],&seq)) { control_fail(r,"Invalid native clear"); return; }
    pthread_mutex_lock(&mutex); bool ok=!strcmp(desired.session,argv[1]) && seq>desired.seq;
    if (ok) { desired.seq=seq; desired.visible=false; desired.received=now_ms(); }
    pthread_mutex_unlock(&mutex);
    if (!ok) { control_fail(r,"Wrong session or stale sequence"); return; }
    control_field_int(r,"queued_sequence",seq);
}
static void status(int argc,char **argv,ControlReply *r) {
    pthread_mutex_lock(&mutex); Published p=published; pthread_mutex_unlock(&mutex);
    control_field_str(r,"state",p.state[0]?p.state:"idle"); control_field_str(r,"error",p.error);
    control_field_int(r,"applied_sequence",p.applied); control_field_int(r,"received_sequence",p.received);
    control_field_bool(r,"active",p.active);
}
void control_hyrulem_init(void) {
    static const ControlCmd cmds[]={
        {"hm_caps","","HyruleM native capabilities and preview availability",caps,CONTROL_PLAYER},
        {"hm_native_status","","Native actor game-thread acknowledgement",status,CONTROL_PLAYER},
        {"hm_native_open","<session>","Open an experimental native actor session",open_session,0},
        {"hm_native_remote","<session> <seq> <x> <y> <z> <yaw> <wait|move|attack>","Queue remote Link pose",remote,0},
        {"hm_native_clear","<session> <seq>","Remove only HyruleM's remote actor",clear_remote,0},
    };
    control_register(cmds,(int)(sizeof cmds/sizeof *cmds));
}
