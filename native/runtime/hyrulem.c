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
    uint64_t applied, received, ticks, last_tick;
    bool active, model_ready, animation_ready;
} Published;
static pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;
static Desired desired;
static Published published;
// Game-thread owned; never dereferenced by command handlers.
static uint64_t storage, actor, spawning_at, applied;
static char session[17];
static int animation = -1;
static bool spawning, failed;
static uint64_t tick_count;
enum { WORLD_MAX=64, HEALTH_MAX=128 };
typedef struct {
    uint32_t hash;
    char name[64];
    float spawn[3], position[3];
    int health;
} EnemyHealth;
static EnemyHealth world_seen[WORLD_MAX], health_pending[HEALTH_MAX];
static int world_count, health_count;
static uint64_t last_world_scan;
enum { HANDLE=0, INDEX=0x10, LOCAL=0x20, PARAMS=0x40, SSTR=0x120,
       KEY=0x130, NAME=0x140, VECTOR=0x180, MATRIX=0x190, ASNAME=0x1d0, BYTES=0x220 };

static uint64_t now_ms(void) {
    struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t);
    return (uint64_t)t.tv_sec*1000 + t.tv_nsec/1000000;
}
static bool range(uint64_t a, uint64_t n) { return a >= GUEST_LO && a < GUEST_HI && n <= GUEST_HI-a; }
static float read_float(uint64_t a) { uint32_t bits=LD32(a);float f;memcpy(&f,&bits,sizeof f);return f; }
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
    published.ticks=tick_count; published.last_tick=now_ms();
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

// Only decode trivial native getters returning an embedded life integer. Do not
// execute an arbitrary virtual method while holding the creator's list lock.
static uint64_t enemy_life(uint64_t a) {
    uint64_t vt=LD64(a);
    if (!range(vt,0x2c0)) return 0;
    uint64_t fn=LD64(vt+0x2b8), life=a;
    if (fn<MAIN_BASE || fn+12>g_site.main_data) return 0;
    for (int i=0;i<3;i++) {
        uint32_t w=LD32(fn+i*4);
        if (w==0xd65f03c0) return life!=a && life-a<0x10000 && range(life,4)?life:0;
        if ((w&0xff8003ff)!=0x91000000) return 0;
        life+=((w>>10)&0xfff) << (((w>>22)&1)?12:0);
    }
    return 0;
}
static bool enemy_name(uint64_t ptr,char out[64]) {
    if (!range(ptr,64)) return false;
    for (int i=0;i<64;i++) {
        unsigned char ch=LD8(ptr+i);out[i]=(char)ch;
        if (!ch) return i>6 && !strncmp(out,"Enemy_",6);
        if (!((ch>='A'&&ch<='Z')||(ch>='a'&&ch<='z')||(ch>='0'&&ch<='9')||ch=='_')) return false;
    }
    return false;
}
static bool same_enemy(const EnemyHealth *a,const EnemyHealth *b) {
    if (a->hash!=b->hash || strcmp(a->name,b->name)) return false;
    for(int i=0;i<3;i++) if(fabsf(a->spawn[i]-b->spawn[i])>.02f) return false;
    return true;
}
static bool world_supported(void) { return supported() && g_site.hm_map_hash_init && g_site.pouch_lock && g_site.pouch_unlock; }
static void world_scan(Cpu *c,uint64_t local,bool connected) {
    uint64_t now=now_ms();
    if (!connected || !world_supported() || now-last_world_scan<180) return;
    last_world_scan=now;
    EnemyHealth pending[HEALTH_MAX],seen[WORLD_MAX];int np=0,n=0;
    pthread_mutex_lock(&mutex);np=health_count;memcpy(pending,health_pending,sizeof *pending*np);pthread_mutex_unlock(&mutex);
    uint64_t creator=LD64(g_site.hm_creator);
    if(!range(creator,0xe8)||LD32(creator+0x4c)!=0x7b8)return;
    float origin[3]={read_float(local+0x3a4),read_float(local+0x3b4),read_float(local+0x3c4)};
    // ActorCreator owns this exact sead::CriticalSection; its native lock adds
    // the internal mutex offset. The lock prevents list nodes being freed.
    call(c,g_site.pouch_lock,creator+0xa8,0,0,0,0,0,0);
    uint64_t head=creator+0x38,node=LD64(head);unsigned limit=LD32(creator+0x48);
    if(limit>8192)limit=0;
    for(unsigned step=0;node!=head&&step<limit&&n<WORLD_MAX;step++) {
        if(!range(node,16)||node<0x7b8)break;
        uint64_t a=node-0x7b8;node=LD64(node);
        if(!range(a,0x7d0)||a==local||LD8(a+0x64)!=1)continue;
        EnemyHealth e={0};
        if(!enemy_name(LD64(a+0x10),e.name))continue;
        uint64_t map=LD64(a+0x7c8),life=enemy_life(a);
        if(!range(map,0x70)||!life)continue;
        e.hash=LD32(map+0x58);e.health=(int32_t)LD32(life);
        if(!e.hash||e.hash==UINT32_MAX||e.health<0||e.health>1000000)continue;
        float distance=0;
        for(int i=0;i<3;i++) {e.spawn[i]=read_float(map+0x5c+i*4);e.position[i]=read_float(a+0x3a4+i*16);float delta=e.position[i]-origin[i];distance+=delta*delta;}
        if(!isfinite(distance)||distance>140*140)continue;
        bool valid=true;for(int i=0;i<3;i++)if(!isfinite(e.spawn[i])||fabsf(e.spawn[i])>100000)valid=false;
        if(!valid)continue;
        for(int i=0;i<np;i++) if(same_enemy(&e,&pending[i])&&pending[i].health<e.health) {
            // Like the original mod's health convergence: never heal from an
            // old snapshot and never write another player's private life.
            e.health=pending[i].health;ST32(life,(uint32_t)e.health);break;
        }
        seen[n++]=e;
    }
    call(c,g_site.pouch_unlock,creator+0xa8,0,0,0,0,0,0);
    pthread_mutex_lock(&mutex);world_count=n;memcpy(world_seen,seen,sizeof *seen*n);pthread_mutex_unlock(&mutex);
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
    if (!range(info,0x68)) return;
    uint64_t local=LD64(info+0x60), damage=c->x[0];
    // The hooked function is PlayerDamage::update, not Player::update.
    // Its original first loads read the owner Actor from this + 0x18. Comparing
    // the component pointer itself with PlayerInfo::actor skipped every frame.
    if (!range(local,0x600) || !range(damage,0x20) || LD64(damage+0x18)!=local) return;
    tick_count++;
    pthread_mutex_lock(&mutex);
    published.ticks=tick_count; published.last_tick=now_ms();
    pthread_mutex_unlock(&mutex);
    Desired d;
    pthread_mutex_lock(&mutex); d=desired; pthread_mutex_unlock(&mutex);
    world_scan(c,local,d.session[0]&&d.received&&now_ms()-d.received<2500);
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
        if (!range(a,0x600) || a==local ||
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
    bool model_ready=range(LD64(a+0x4e0),0x60), animation_ready=range(LD64(a+0x560),8);
    pthread_mutex_lock(&mutex);
    published.model_ready=model_ready; published.animation_ready=animation_ready;
    pthread_mutex_unlock(&mutex);
    if (!model_ready || !animation_ready) {
        release_local(c); remove_remote(c); failed=true;
        publish("error",!model_ready?"remote-model-not-loaded":"remote-animation-not-loaded",applied,false); return;
    }
    set_pose(c,a,&d,safe_string_vtable); release_local(c);
    applied=d.seq; publish("active","",applied,true);
}

RT_ON_RESET(hyrulem_reset) {
    pthread_mutex_lock(&mutex);
    memset(&desired,0,sizeof desired); memset(&published,0,sizeof published);
    world_count=health_count=0;last_world_scan=0;
    pthread_mutex_unlock(&mutex);
    storage=actor=spawning_at=applied=tick_count=0; session[0]=0;
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
    control_field_int(r,"preview_abi",2); control_field_bool(r,"native_preview",supported());
    control_field_bool(r,"shared_enemy_health",world_supported());
    // These are end-to-end Gen1 capabilities, not addresses found in a binary.
    // Keep them false until combat, pickups and AI are wired and play-tested.
    const char *names[]={"native_actors","animation","collision","personal_vitals","personal_inventory",
        "pickup_hook","damage_hook","enemy_authority","enemy_target","world_identity"};
    for (unsigned i=0;i<sizeof names/sizeof *names;i++) control_field_bool(r,names[i],false);
}
static void open_session(int argc,char **argv,ControlReply *r) {
    if (argc!=2 || !token(argv[1]) || !supported()) { control_fail(r,"Native preview disabled, incompatible or invalid session"); return; }
    pthread_mutex_lock(&mutex);
    if (strcmp(desired.session,argv[1])) { memset(&desired,0,sizeof desired); health_count=world_count=0; snprintf(desired.session,sizeof desired.session,"%s",argv[1]); }
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
    control_field_int(r,"game_ticks",p.ticks);
    control_field_int(r,"tick_age_ms",p.last_tick?(int64_t)(now_ms()-p.last_tick):-1);
    control_field_bool(r,"model_ready",p.model_ready);
    control_field_bool(r,"animation_ready",p.animation_ready);
    control_field_bool(r,"game_paused",range(g_site.pause_state,4)&&LD32(g_site.pause_state)!=0);
}
static void world_status(int argc,char **argv,ControlReply *r) {
    EnemyHealth copy[WORLD_MAX];int n;
    pthread_mutex_lock(&mutex);n=world_count;memcpy(copy,world_seen,sizeof *copy*n);pthread_mutex_unlock(&mutex);
    char json[24000];size_t used=0;json[used++]='[';
    for(int i=0;i<n;i++) {
        EnemyHealth *e=&copy[i];
        int written=snprintf(json+used,sizeof json-used,"%s{\"hash\":%u,\"actor\":\"%s\",\"spawn\":{\"X\":%.3f,\"Y\":%.3f,\"Z\":%.3f},\"position\":{\"X\":%.3f,\"Y\":%.3f,\"Z\":%.3f},\"health\":%d}",i?",":"",e->hash,e->name,e->spawn[0],e->spawn[1],e->spawn[2],e->position[0],e->position[1],e->position[2],e->health);
        if(written<0||(size_t)written>=sizeof json-used-2){control_fail(r,"Enemy snapshot overflow");return;}used+=(size_t)written;
    }
    json[used++]=']';json[used]=0;control_field_str(r,"enemies_json",json);control_field_int(r,"count",n);
}
static void receive_health(int argc,char **argv,ControlReply *r) {
    EnemyHealth e={0};uint64_t hash,hp;
    if(argc!=8||!world_supported()||!token(argv[1])||!integer(argv[2],&hash)||hash>=UINT32_MAX||strlen(argv[3])>=sizeof e.name||strncmp(argv[3],"Enemy_",6)||!number(argv[4],&e.spawn[0],100000)||!number(argv[5],&e.spawn[1],50000)||!number(argv[6],&e.spawn[2],100000)) {control_fail(r,"Invalid enemy health");return;}
    if(!strcmp(argv[7],"0"))hp=0;else if(!integer(argv[7],&hp)||hp>1000000){control_fail(r,"Invalid enemy life");return;}
    for(const char *p=argv[3];*p;p++)if(!((*p>='A'&&*p<='Z')||(*p>='a'&&*p<='z')||(*p>='0'&&*p<='9')||*p=='_')){control_fail(r,"Invalid enemy actor");return;}
    e.hash=(uint32_t)hash;e.health=(int)hp;strcpy(e.name,argv[3]);
    pthread_mutex_lock(&mutex);
    bool ok=!strcmp(desired.session,argv[1]);int at=0;while(at<health_count&&!same_enemy(&e,&health_pending[at]))at++;
    if(ok&&at<HEALTH_MAX){if(at==health_count)health_pending[health_count++]=e;else if(e.health<health_pending[at].health)health_pending[at].health=e.health;}else ok=false;
    pthread_mutex_unlock(&mutex);if(!ok){control_fail(r,"Wrong session or enemy queue full");return;}control_field_bool(r,"queued",true);
}
void control_hyrulem_init(void) {
    static const ControlCmd cmds[]={
        {"hm_caps","","HyruleM native capabilities and preview availability",caps,CONTROL_PLAYER},
        {"hm_native_status","","Native actor game-thread acknowledgement",status,CONTROL_PLAYER},
        {"hm_native_open","<session>","Open an experimental native actor session",open_session,0},
        {"hm_native_remote","<session> <seq> <x> <y> <z> <yaw> <wait|move|attack>","Queue remote Link pose",remote,0},
        {"hm_native_clear","<session> <seq>","Remove only HyruleM's remote actor",clear_remote,0},
        {"hm_world_status","","Loaded map-enemy health snapshot",world_status,CONTROL_PLAYER},
        {"hm_enemy_health","<session> <hash> <actor> <sx> <sy> <sz> <life>","Queue map-enemy health convergence",receive_health,0},
    };
    control_register(cmds,(int)(sizeof cmds/sizeof *cmds));
}
