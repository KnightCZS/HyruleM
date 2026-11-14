// HyruleM's experimental remote actor adapter. Original implementation; no
// game code or resources. Guest calls run after the native actor-job barrier.
#include "hle.h"
#include "game_sites.h"
#include "control.h"
#include <errno.h>
#include <math.h>
#include <pthread.h>
#include <string.h>
#include <time.h>

typedef struct {
    char name[64];
    uint64_t sequence;
    bool force;
    unsigned slot;
} Track;
enum { GEAR_COUNT=6 };
typedef struct { char name[48];float matrix[12];uint64_t seq,received; } GearPose;
typedef struct {
    char session[17];
    uint64_t seq, received;
    float x, y, z, yaw;
    int animation;
    bool visible;
    Track tracks[2];
    GearPose gear[GEAR_COUNT];
} Desired;
typedef struct {
    char state[32], error[96];
    char first_error[96];
    uint64_t applied, received, ticks, last_tick;
    bool active, model_ready, animation_ready;
    float requested[3], observed[3];
    uint64_t animation_requests;
    unsigned spawn_attempts;
    bool resource_fallback, visual_only;
    char reference_status[48];
    int actor_state;
    uint64_t activation_wait_ticks;
    uint64_t rejected_callbacks;
    char equipment_json[1536];
    unsigned equipment_active;
} Published;
static pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;
static Desired desired;
static Published published;
// Game-thread owned; never dereferenced by command handlers.
static uint64_t storage, actor, spawning_at, applied;
static char session[17];
static int animation = -1;
static bool spawning, failed;
static bool fallback_tried;
static unsigned resource_variant, spawn_attempts;
static uint64_t tick_count;
static uint64_t activation_wait_ticks;
static uint64_t activation_wait_at;
static bool activation_waiting;
static Track local_tracks[2], remote_tracks[2];
static uint64_t capture_count;
static float local_yaw;
static GearPose local_gear[GEAR_COUNT];
static float local_position[3];
static bool local_ready;
typedef struct {uint64_t storage,actor,started;bool spawning,failed,active;char name[48],error[48];Track tracks[2];} GearVisual;
static GearVisual gear_visual[GEAR_COUNT];
static float rendered_pose[4];static uint64_t rendered_at;static bool rendered_ready;
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
       KEY=0x130, NAME=0x140, VECTOR=0x180, MATRIX=0x190, ASNAME=0x1d0, BYTES=0x240 };

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
        g_site.hm_set_mtx && g_site.hm_delete_actor && g_site.hm_play_as && g_site.hm_as_play_entry &&
        g_site.actor_index_assign && g_site.actor_index_resolve &&
        g_site.actor_index_release && g_site.actor_link_release &&
        g_site.hm_world_tick && g_site.hm_actor_mgr;
}
static bool clip_name(const char *s) {
    size_t n=strnlen(s,64);if(!n||n>=64)return false;
    for(size_t i=0;i<n;i++)if(!((s[i]>='a'&&s[i]<='z')||(s[i]>='A'&&s[i]<='Z')||(s[i]>='0'&&s[i]<='9')||s[i]=='_'||s[i]=='-'))return false;
    return true;
}
// Read-only entry hook at the actual AS request body, including direct calls.
// Remote requests have another owner and can never become local input.
void rt_hyrulem_capture_as(Cpu *c) {
    if(!supported()||!g_site.hm_as_play_entry||!range(g_site.player_info,8))return;
    uint64_t info=LD64(g_site.player_info),as=c->x[0],request=c->x[1];
    if(!range(info,0x68)||!range(as,0xe0)||!range(request,0x38)||LD32(request+0x18)>1||LD32(request+0x1c)>1)return;
    uint64_t local=LD64(info+0x60),ptr=LD64(request+0x10);
    if(!local||LD64(as+0xd8)!=local||!range(ptr,64))return;
    char name[64];memcpy(name,(void *)(uintptr_t)ptr,64);if(!memchr(name,0,64))return;
    if(!clip_name(name))return;
    pthread_mutex_lock(&mutex);
    Track *t=&local_tracks[LD32(request+0x18)];
    snprintf(t->name,sizeof t->name,"%s",name);t->sequence=++capture_count;t->force=(LD8(request+0x21)&1)!=0;t->slot=LD32(request+0x1c);
    pthread_mutex_unlock(&mutex);
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
    published.spawn_attempts=spawn_attempts;published.resource_fallback=resource_variant!=0;
    pthread_mutex_unlock(&mutex);
}
static void release_local(Cpu *c) { call(c,g_site.actor_link_release,storage+LOCAL,0,0,0,0,0,0); }
static void release_at(Cpu *c,uint64_t st) {call(c,g_site.actor_link_release,st+LOCAL,0,0,0,0,0,0);}
typedef struct { uint64_t ptr; int state; const char *status; bool invalid; } ActorRef;
static ActorRef resolve_at(Cpu *c,uint64_t st,uint64_t owner) {
    ActorRef r={0,-1,"index-not-retained",true};
    if (!owner || !st || !LD8(st+INDEX+12) || LD32(st+INDEX+8)==UINT32_MAX) return r;
    uint64_t block=LD64(st+INDEX);
    r.status="index-mismatch";
    if (!range(block,0x54) || LD32(block+0x40)!=LD32(st+INDEX+8) || LD64(block+0x48)!=owner) return r;
    uint64_t a=call(c,g_site.actor_index_resolve,st+INDEX,st+LOCAL,0,0,0,0,0);
    if (!a) {
        // A failed native access check may still populate the local link.
        release_at(c,st);r.status="lease-unavailable";r.invalid=false;return r;
    }
    if (a!=owner || !range(a,0x600) || LD64(st+LOCAL+8)!=a) {
        release_at(c,st);r.status="local-link-mismatch";return r;
    }
    r.state=LD8(a+0x64);
    if (r.state>2) { release_at(c,st);r.status="actor-not-live";return r; }
    r.ptr=a;r.invalid=false;r.status=r.state==1?"ready":"activation-pending";
    return r;
}
static ActorRef resolve(Cpu *c) {return resolve_at(c,storage,actor);}
static bool remove_gear(Cpu *c,GearVisual *g) {
    g->active=false;
    if(!g->storage)return true;
    if(g->spawning){call(c,g_site.hm_handle_delete,g->storage+HANDLE,0,0,0,0,0,0);g->spawning=false;}
    ActorRef ref=resolve_at(c,g->storage,g->actor);
    if(ref.ptr){call(c,g_site.hm_delete_actor,ref.ptr,0,0,0,0,0,0);release_at(c,g->storage);}
    else if(g->actor&&!ref.invalid)return false;
    call(c,g_site.actor_index_release,g->storage+INDEX,0,0,0,0,0,0);
    g->actor=0;g->spawning=false;memset(g->tracks,0,sizeof g->tracks);return true;
}
static bool remove_remote(Cpu *c) {
    if (!storage) return true;
    bool cleaned=true;for(int i=0;i<GEAR_COUNT;i++)if(!remove_gear(c,&gear_visual[i]))cleaned=false;
    if(!cleaned)return false;
    if (spawning) { call(c,g_site.hm_handle_delete,storage+HANDLE,0,0,0,0,0,0);spawning=false; }
    ActorRef r=resolve(c);
    if (r.ptr) { call(c,g_site.hm_delete_actor,r.ptr,0,0,0,0,0,0); release_local(c); }
    // Keep ownership while the scheduler denies access. A subsequent game
    // callback retries cleanup, without creating another actor or losing it.
    else if (actor&&!r.invalid) return false;
    call(c,g_site.actor_index_release,storage+INDEX,0,0,0,0,0,0);
    actor=0; spawning=false; animation=-1;memset(remote_tracks,0,sizeof remote_tracks);rendered_ready=false;
    return true;
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
    spawn_attempts++;
    if (!call(c,g_site.hm_create,creator,storage+NAME,heap,storage+HANDLE,storage+PARAMS,0,1)) return false;
    spawning=true; spawning_at=now_ms(); activation_wait_ticks=0;activation_waiting=false;return true;
}
static bool retry_fallback(Cpu *c,const char *reason) {
    pthread_mutex_lock(&mutex);
    if(!published.first_error[0])snprintf(published.first_error,sizeof published.first_error,"%s",reason);
    pthread_mutex_unlock(&mutex);
    fprintf(stderr,"[hyrulem] resource variant %u rejected: %s\n",resource_variant,reason);
    if(fallback_tried)return false;
    if(!remove_remote(c))return false;
    fallback_tried=true;resource_variant=1;
    strcpy((char *)(uintptr_t)(storage+NAME),"HyruleM_RemoteLink_Fallback");
    publish("retrying-visual-fallback","",applied,false);return true;
}
static void fatal(Cpu *c,const char *reason) {
    pthread_mutex_lock(&mutex);
    if(!published.first_error[0])snprintf(published.first_error,sizeof published.first_error,"%s",reason);
    pthread_mutex_unlock(&mutex);
    remove_remote(c);failed=true;publish("error",reason,applied,false);
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
static bool continuous_as(const char *s) {
    return !strcmp(s,"Wait")||!strcmp(s,"WaitUpper")||!strcmp(s,"Move")||!strcmp(s,"MoveUpper")||!strcmp(s,"Walk")||!strcmp(s,"Run");
}
static void play_tracks(Cpu *c,uint64_t st,uint64_t a,Desired *d,Track cache[2],uint64_t vt) {
    uint64_t as=LD64(a+0x560);
    if (!range(as,0xe0) || LD32(as+0xb8)<2 || !range(LD64(as+0xc0),0xa0)) return;
    for(int i=0;i<2;i++) {
        Track t=d->tracks[i];
        if(!t.name[0])snprintf(t.name,sizeof t.name,"%s",d->animation==1?(i?"Move":"MoveUpper"):(i?"Wait":"WaitUpper"));
        if(!strcmp(t.name,cache[i].name)&&t.slot==cache[i].slot&&(continuous_as(t.name)||!t.force||t.sequence==cache[i].sequence))continue;
        snprintf((char *)(uintptr_t)(st+ASNAME),64,"%s",t.name);
        ST64(st+SSTR,vt);ST64(st+SSTR+8,st+ASNAME);
        Cpu scratch=*c;
        scratch.x[0]=as;scratch.x[1]=st+SSTR;scratch.x[2]=i;scratch.x[3]=t.slot;scratch.x[4]=t.force||strcmp(t.name,cache[i].name);
        float blend=-1.f;scratch.v[0]=scratch.v[1]=0;
        memcpy(&scratch.v[0],&blend,sizeof blend);memcpy(&scratch.v[1],&blend,sizeof blend);
        guest_call(&scratch,g_site.hm_play_as);
        cache[i]=t;
        pthread_mutex_lock(&mutex);published.animation_requests++;pthread_mutex_unlock(&mutex);
    }
}
static void set_pose(Cpu *c,uint64_t a,Desired *d,uint64_t vt,bool animate) {
    float s=sinf(d->yaw),co=cosf(d->yaw);float matrix[12]={co,0,s,d->x,0,1,0,d->y,-s,0,co,d->z};
    memcpy((void *)(uintptr_t)(storage+MATRIX),matrix,sizeof matrix);
    call(c,g_site.hm_set_mtx,a,storage+MATRIX,0,0,0,0,0);call(c,g_site.hm_set_mtx,a,storage+MATRIX,1,0,0,0,0);
    if(animate)play_tracks(c,storage,a,d,remote_tracks,vt);
}
static void smooth_pose(Desired *d,uint64_t at) {
    float dx=d->x-rendered_pose[0],dy=d->y-rendered_pose[1],dz=d->z-rendered_pose[2];
    float dt=rendered_at&&at>rendered_at?(float)(at-rendered_at)/1000.f:.001f;
    float alpha=!rendered_ready||dx*dx+dy*dy+dz*dz>400||dt>.5f?1.f:1.f-expf(-dt/.035f);
    rendered_pose[0]+=dx*alpha;rendered_pose[1]+=dy*alpha;rendered_pose[2]+=dz*alpha;
    float angle=remainderf(d->yaw-rendered_pose[3],6.28318530718f);rendered_pose[3]+=angle*alpha;
    rendered_at=at;rendered_ready=true;d->x=rendered_pose[0];d->y=rendered_pose[1];d->z=rendered_pose[2];d->yaw=rendered_pose[3];
}
static bool gear_name(const char *s,int slot) {
    if(!s[0]||strlen(s)>=48||!clip_name(s))return false;
    if(slot<3)return slot==0?(!strncmp(s,"Weapon_Sword_",13)||!strncmp(s,"Weapon_Lsword_",14)||!strncmp(s,"Weapon_Spear_",13)):slot==1?!strncmp(s,"Weapon_Bow_",11):!strncmp(s,"Weapon_Shield_",14);
    const char *suffix=slot==3?"_Head":slot==4?"_Upper":"_Lower";size_t n=strlen(s),m=strlen(suffix);
    return !strncmp(s,"Armor_",6)&&n>m&&!strcmp(s+n-m,suffix);
}
static void capture_gear(Cpu *c,uint64_t local) {
    GearPose gear[GEAR_COUNT]={0};float xyz[3]={read_float(local+0x3a4),read_float(local+0x3b4),read_float(local+0x3c4)};
    if(!isfinite(xyz[0])||!isfinite(xyz[1])||!isfinite(xyz[2])||fabsf(xyz[0])>100000||fabsf(xyz[1])>50000||fabsf(xyz[2])>100000||(xyz[0]==0&&xyz[1]==0&&xyz[2]==0)) {pthread_mutex_lock(&mutex);local_ready=false;pthread_mutex_unlock(&mutex);return;}
    float yaw=atan2f(read_float(local+0x3a0),read_float(local+0x398)),s=sinf(yaw),co=cosf(yaw);
    for(int i=0;i<GEAR_COUNT;i++) {
        GearPose *g=&gear[i];g->matrix[0]=g->matrix[5]=g->matrix[10]=1;
        if(i>=3)snprintf(g->name,sizeof g->name,"Armor_Default_%s",i==3?"Head":i==4?"Upper":"Lower");
        if(!g_site.player_equipment)continue;
        uint64_t a=call(c,g_site.player_equipment,local,i,0,0,0,0,0);
        if(a==local||!range(a,0x600)||LD8(a+0x64)!=1)continue;
        uint64_t name=LD64(a+0x10);if(!range(name,48)||!memchr((void *)(uintptr_t)name,0,48)||!gear_name((char *)(uintptr_t)name,i))continue;
        snprintf(g->name,sizeof g->name,"%s",(char *)(uintptr_t)name);
        if(i>=3)continue; // Armor uses the same root and skeletal AS as the body.
        uint64_t model=LD64(a+0x4e0);if(!range(model,0x88)){g->name[0]=0;continue;}
        float m[12];for(int j=0;j<12;j++)m[j]=read_float(model+0x58+j*4);
        bool valid=true;for(int j=0;j<12;j++)if(!isfinite(m[j]))valid=false;
        if(!valid){g->name[0]=0;continue;}
        for(int j=0;j<3;j++){g->matrix[j]=co*m[j]-s*m[8+j];g->matrix[4+j]=m[4+j];g->matrix[8+j]=s*m[j]+co*m[8+j];}
        float dx=m[3]-xyz[0],dy=m[7]-xyz[1],dz=m[11]-xyz[2];
        g->matrix[3]=co*dx-s*dz;g->matrix[7]=dy;g->matrix[11]=s*dx+co*dz;
        for(int j=0;j<12;j++)if(fabsf(g->matrix[j])>((j%4)==3?32.f:8.f))valid=false;
        for(int row=0;row<3;row++){float norm=0;for(int j=0;j<3;j++)norm+=g->matrix[row*4+j]*g->matrix[row*4+j];if(norm<.0001f)valid=false;}
        if(!valid){g->name[0]=0;memset(g->matrix,0,sizeof g->matrix);g->matrix[0]=g->matrix[5]=g->matrix[10]=1;}
    }
    pthread_mutex_lock(&mutex);memcpy(local_gear,gear,sizeof gear);memcpy(local_position,xyz,sizeof xyz);local_ready=true;pthread_mutex_unlock(&mutex);
}
static void gear_tick(Cpu *c,Desired *d,uint64_t vt,bool animate) {
    for(int i=0;i<GEAR_COUNT;i++) {
        GearVisual *g=&gear_visual[i];GearPose *p=&d->gear[i];const char *want=p->received&&now_ms()-p->received<2500?p->name:"";
        g->active=false;
        if(strcmp(g->name,want)) {if(!remove_gear(c,g))continue;snprintf(g->name,sizeof g->name,"%s",want);g->failed=false;g->error[0]=0;}
        if(!want[0]||g->failed){if(g->actor)remove_gear(c,g);continue;}
        if(!g->storage) {g->storage=(uint64_t)(uintptr_t)hle_alloc(BYTES);if(!range(g->storage,BYTES)){g->failed=true;continue;}ST32(g->storage+INDEX+8,UINT32_MAX);strcpy((char *)(uintptr_t)(g->storage+KEY),"@P");}
        uint64_t st=g->storage;
        if(g->spawning) {
            if(call(c,g_site.hm_handle_failed,st+HANDLE,0,0,0,0,0,0)){snprintf(g->error,sizeof g->error,"spawn-failed");g->failed=true;remove_gear(c,g);continue;}
            if(!call(c,g_site.hm_handle_ready,st+HANDLE,0,0,0,0,0,0)){if(now_ms()-g->started>15000){g->failed=true;snprintf(g->error,sizeof g->error,"spawn-timeout");remove_gear(c,g);}continue;}
            uint64_t a=call(c,g_site.hm_handle_get,st+HANDLE,0,0,0,0,0,0);
            if(!range(a,0x600)||a==actor||!call(c,g_site.actor_index_assign,st+INDEX,a,1,0,0,0,0)){g->failed=true;snprintf(g->error,sizeof g->error,"reference-failed");remove_gear(c,g);continue;}
            g->actor=a;uint64_t awake=call(c,g_site.hm_handle_wake,st+HANDLE,0,0,0,0,0,0);g->spawning=false;
            if(awake!=a){g->failed=true;snprintf(g->error,sizeof g->error,"wake-failed");remove_gear(c,g);continue;}
            // Start all loaded skeletal pieces and the body at the same action.
            if(i>=3){memset(remote_tracks,0,sizeof remote_tracks);for(int j=3;j<GEAR_COUNT;j++)memset(gear_visual[j].tracks,0,sizeof gear_visual[j].tracks);}
        }
        if(!g->actor) {
            uint64_t creator=LD64(g_site.hm_creator),manager=LD64(g_site.hm_heap_mgr);if(!range(creator,0x60)||!range(manager,0x30)||LD8(creator+0x59))continue;
            uint64_t heap=LD64(manager+0x28);if(!range(heap,8))continue;
            memset((void *)(uintptr_t)(st+HANDLE),0,16);memset((void *)(uintptr_t)(st+PARAMS),0,0xd8);
            float xyz[3]={d->x,d->y,d->z};memcpy((void *)(uintptr_t)(st+VECTOR),xyz,sizeof xyz);ST64(st+SSTR,vt);ST64(st+SSTR+8,st+KEY);
            call(c,g_site.hm_param_add,st+PARAMS+0x10,st+VECTOR,st+SSTR,12,4,0,0);
            snprintf((char *)(uintptr_t)(st+NAME),64,"HyruleM_Equip_%s",want);
            if(!call(c,g_site.hm_create,creator,st+NAME,heap,st+HANDLE,st+PARAMS,0,1)){g->failed=true;snprintf(g->error,sizeof g->error,"unknown-visual-resource");continue;}
            g->spawning=true;g->started=now_ms();continue;
        }
        ActorRef ref=resolve_at(c,st,g->actor);
        if(!ref.ptr||ref.state!=1){if(ref.ptr)release_at(c,st);if(ref.invalid||now_ms()-g->started>15000){g->failed=true;snprintf(g->error,sizeof g->error,"activation-failed");remove_gear(c,g);}continue;}
        if(!range(LD64(ref.ptr+0x4e0),0xa4)){release_at(c,st);if(now_ms()-g->started>15000){g->failed=true;snprintf(g->error,sizeof g->error,"model-not-loaded");remove_gear(c,g);}continue;}
        float s=sinf(d->yaw),co=cosf(d->yaw),m[12];for(int j=0;j<4;j++){m[j]=co*p->matrix[j]+s*p->matrix[8+j];m[4+j]=p->matrix[4+j];m[8+j]=-s*p->matrix[j]+co*p->matrix[8+j];}m[3]+=d->x;m[7]+=d->y;m[11]+=d->z;
        memcpy((void *)(uintptr_t)(st+MATRIX),m,sizeof m);call(c,g_site.hm_set_mtx,ref.ptr,st+MATRIX,0,0,0,0,0);call(c,g_site.hm_set_mtx,ref.ptr,st+MATRIX,1,0,0,0,0);
        if(animate&&i>=3)play_tracks(c,st,ref.ptr,d,g->tracks,vt);release_at(c,st);g->active=true;g->started=now_ms();
    }
    char json[1536];size_t used=0;unsigned active=0;json[used++]='[';
    for(int i=0;i<GEAR_COUNT;i++){GearVisual *g=&gear_visual[i];active+=g->active;int n=snprintf(json+used,sizeof json-used,"%s{\"slot\":%d,\"name\":\"%s\",\"active\":%s,\"error\":\"%s\"}",i?",":"",i,g->name,g->active?"true":"false",g->error);if(n<0||(size_t)n>=sizeof json-used-2)break;used+=(size_t)n;}
    json[used++]=']';json[used]=0;pthread_mutex_lock(&mutex);snprintf(published.equipment_json,sizeof published.equipment_json,"%s",json);published.equipment_active=active;pthread_mutex_unlock(&mutex);
}

// Hooked after the engine joins actor jobs and resets their access phase.
// The previous PlayerDamage worker could retain a valid actor but the native
// scheduler consistently denied access to it (issue #6). Never disable that
// protection or fall back to using a raw pointer from a rejected resolver.
void rt_hyrulem_tick(Cpu *c, uint64_t safe_string_vtable) {
    if (!supported() || !range(g_site.player_info,8)) return;
    if(c->x[22]!=g_site.hm_actor_mgr||!range(g_site.hm_actor_mgr,8))return;
    uint64_t manager=LD64(g_site.hm_actor_mgr);
    if(!range(manager,0x178)||LD8(manager+0x160)||LD8(manager+0x164)||
        LD8(manager+0x28)!=0||LD8(manager+0x29)!=7||LD8(manager+0x2a)!=8) {
        pthread_mutex_lock(&mutex);published.rejected_callbacks++;pthread_mutex_unlock(&mutex);
        return;
    }
    uint64_t info=LD64(g_site.player_info);
    if (!range(info,0x68)) return;
    uint64_t local=LD64(info+0x60);
    if (!range(local,0x600)||LD8(local+0x64)!=1) return;
    capture_gear(c,local);
    tick_count++;
    pthread_mutex_lock(&mutex);
    local_yaw=atan2f(read_float(local+0x3a0),read_float(local+0x398));
    published.ticks=tick_count; published.last_tick=now_ms();
    pthread_mutex_unlock(&mutex);
    Desired d;
    pthread_mutex_lock(&mutex); d=desired; pthread_mutex_unlock(&mutex);
    if(range(g_site.pause_state,4)&&LD32(g_site.pause_state)) {
        if(spawning_at)spawning_at=now_ms();
        if(activation_waiting)activation_wait_at=now_ms();
        for(int i=0;i<GEAR_COUNT;i++)if(gear_visual[i].spawning||gear_visual[i].actor)gear_visual[i].started=now_ms();
        return;
    }
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
        if(!remove_remote(c)){publish("cleaning-up","",applied,false);return;}
        snprintf(session,sizeof session,"%s",d.session);
        applied=0; failed=false;
        fallback_tried=false;resource_variant=spawn_attempts=0;
        for(int i=0;i<GEAR_COUNT;i++){gear_visual[i].failed=false;gear_visual[i].name[0]=gear_visual[i].error[0]=0;}
        strcpy((char *)(uintptr_t)(storage+NAME),"HyruleM_RemoteLink");
        pthread_mutex_lock(&mutex);
        published.first_error[0]=published.error[0]=published.reference_status[0]=0;
        published.actor_state=-1;published.activation_wait_ticks=0;
        published.model_ready=published.animation_ready=published.visual_only=false;
        memset(published.requested,0,sizeof published.requested);memset(published.observed,0,sizeof published.observed);
        published.animation_requests=0;
        pthread_mutex_unlock(&mutex);
    }
    if (failed) { if(actor)remove_remote(c);return; } // Preserve the fatal result during deferred cleanup.
    bool stale=d.received && now_ms()-d.received>2500;
    if (!d.visible || stale) {
        if(!remove_remote(c)){publish("cleaning-up","",applied,false);return;}
        applied=d.seq;
        publish(stale?"expired":"idle","",applied,false); return;
    }
    if (spawning) {
        if (call(c,g_site.hm_handle_failed,storage+HANDLE,0,0,0,0,0,0)) {
            if(retry_fallback(c,"native-spawn-failed"))return;
            fatal(c,"native-spawn-failed");return;
        }
        // A ready handle wins over wall-clock timeout after a long pause.
        if (!call(c,g_site.hm_handle_ready,storage+HANDLE,0,0,0,0,0,0)) {
            if (now_ms()-spawning_at>15000) {
                if(retry_fallback(c,"native-spawn-timeout"))return;
                fatal(c,"native-spawn-timeout");return;
            }
            publish("spawning","",applied,false); return;
        }
        uint64_t a=call(c,g_site.hm_handle_get,storage+HANDLE,0,0,0,0,0,0);
        if (!range(a,0x600) || a==local ||
            !call(c,g_site.actor_index_assign,storage+INDEX,a,1,0,0,0,0)) {
            fatal(c,"native-reference-failed");return;
        }
        actor=a;
        uint64_t awake=call(c,g_site.hm_handle_wake,storage+HANDLE,0,0,0,0,0,0);
        spawning=false;
        activation_waiting=true;activation_wait_at=spawning_at;
        if (awake!=a) { fatal(c,"native-wake-failed");return; }
    }
    if (!actor) {
        if (!prepare(c,&d,safe_string_vtable)) {
            if(retry_fallback(c,"native-spawn-rejected-check-private-overlay"))return;
            fatal(c,"native-spawn-rejected-check-private-overlay");return;
        }
        publish("spawning","",applied,false); return;
    }
    ActorRef ref=resolve(c);
    pthread_mutex_lock(&mutex);
    snprintf(published.reference_status,sizeof published.reference_status,"%s",ref.status);
    published.actor_state=ref.state;
    pthread_mutex_unlock(&mutex);
    if(ref.invalid){fatal(c,ref.status);return;}
    if(!ref.ptr||ref.state!=1) {
        if(ref.ptr)release_local(c);
        if(!activation_waiting){activation_waiting=true;activation_wait_at=now_ms();}
        pthread_mutex_lock(&mutex);published.activation_wait_ticks=++activation_wait_ticks;pthread_mutex_unlock(&mutex);
        // Waking a ready handle queues activation in the engine. States 0/2
        // and a temporarily unavailable lease are not an unload notification.
        // Preserve the retained index and wait for a later game callback.
        if(now_ms()-activation_wait_at<15000){publish("activating","",applied,false);return;}
        const char *reason=ref.ptr?"native-activation-timeout":"native-access-timeout";
        if(retry_fallback(c,reason))return;
        fatal(c,reason);return;
    }
    activation_waiting=false;
    uint64_t a=ref.ptr;
    uint64_t as=LD64(a+0x560);
    bool model_ready=range(LD64(a+0x4e0),0xa4);
    bool animation_ready=range(as,0x168)&&LD32(as+0xb8)>=2&&range(LD64(as+0xc0),0xa0)&&LD32(as+0x138)>0;
    pthread_mutex_lock(&mutex);
    published.model_ready=model_ready; published.animation_ready=animation_ready;
    published.visual_only=resource_variant!=0||!animation_ready;
    pthread_mutex_unlock(&mutex);
    if (!model_ready) {
        if(now_ms()-spawning_at<15000){release_local(c);publish("loading-visuals","",applied,false);return;}
        if(!fallback_tried){release_local(c);if(retry_fallback(c,"remote-model-not-loaded"))return;}
        release_local(c);fatal(c,"remote-model-not-loaded");return;
    }
    pthread_mutex_lock(&mutex);
    published.requested[0]=d.x;published.requested[1]=d.y;published.requested[2]=d.z;
    for(int i=0;i<3;i++)published.observed[i]=read_float(a+0x3a4+i*16);
    pthread_mutex_unlock(&mutex);
    smooth_pose(&d,now_ms());
    set_pose(c,a,&d,safe_string_vtable,resource_variant==0&&animation_ready); release_local(c);
    gear_tick(c,&d,safe_string_vtable,resource_variant==0&&animation_ready);
    applied=d.seq; publish(resource_variant?"fallback-active":animation_ready?"active":"active-no-animation","",applied,true);
}

RT_ON_RESET(hyrulem_reset) {
    pthread_mutex_lock(&mutex);
    memset(&desired,0,sizeof desired); memset(&published,0,sizeof published);
    memset(local_tracks,0,sizeof local_tracks);memset(remote_tracks,0,sizeof remote_tracks);capture_count=0;local_yaw=0;
    memset(local_gear,0,sizeof local_gear);memset(local_position,0,sizeof local_position);local_ready=false;
    world_count=health_count=0;last_world_scan=0;
    pthread_mutex_unlock(&mutex);
    storage=actor=spawning_at=applied=tick_count=activation_wait_ticks=activation_wait_at=0; session[0]=0;
    activation_waiting=false;
    animation=-1; spawning=failed=false;
    fallback_tried=false;resource_variant=spawn_attempts=0;
    memset(gear_visual,0,sizeof gear_visual);memset(rendered_pose,0,sizeof rendered_pose);rendered_at=0;rendered_ready=false;
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
    control_field_int(r,"preview_abi",5); control_field_bool(r,"native_preview",supported());
    control_field_bool(r,"visual_equipment",supported()&&g_site.player_equipment);
    control_field_bool(r,"shared_environment",supported());
    control_field_bool(r,"actor_job_barrier",supported());
    control_field_bool(r,"animation_capture",supported()&&g_site.hm_as_play_entry);
    control_field_bool(r,"visual_fallback",supported());
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
    if ((argc!=16&&argc!=94) || !supported() || !token(argv[1]) || !integer(argv[2],&next.seq) ||
        !number(argv[3],&next.x,100000) || !number(argv[4],&next.y,50000) ||
        !number(argv[5],&next.z,100000) || !number(argv[6],&next.yaw,10000)) {
        control_fail(r,"Invalid native remote pose"); return;
    }
    next.animation=!strcmp(argv[7],"move")?1:!strcmp(argv[7],"attack")?2:!strcmp(argv[7],"wait")?0:-1;
    if (next.animation<0) { control_fail(r,"Unsupported animation"); return; }
    for(int i=0;i<2;i++) {
        int j=8+i*3;Track *t=&next.tracks[i];
        if(strcmp(argv[j],"-")) {
            if(!clip_name(argv[j])){control_fail(r,"Invalid animation name");return;}
            snprintf(t->name,sizeof t->name,"%s",argv[j]);
        }
        if(strcmp(argv[j+1],"0")&&!integer(argv[j+1],&t->sequence)){control_fail(r,"Invalid animation sequence");return;}
        if(strcmp(argv[j+2],"0")&&strcmp(argv[j+2],"1")){control_fail(r,"Invalid animation force");return;}
        t->force=!strcmp(argv[j+2],"1");
        if(strcmp(argv[14+i],"0")&&strcmp(argv[14+i],"1")){control_fail(r,"Invalid animation slot");return;}
        t->slot=!strcmp(argv[14+i],"1");
    }
    if(argc==94)for(int i=0;i<GEAR_COUNT;i++) {
        int at=16+i*13;GearPose *g=&next.gear[i];
        if(strcmp(argv[at],"-")){if(!gear_name(argv[at],i)){control_fail(r,"Invalid visual equipment name");return;}snprintf(g->name,sizeof g->name,"%s",argv[at]);}
        for(int j=0;j<12;j++)if(!number(argv[at+1+j],&g->matrix[j],j%4==3?32:8)){control_fail(r,"Invalid visual equipment matrix");return;}
        g->seq=next.seq;g->received=now_ms();
    }
    pthread_mutex_lock(&mutex);
    bool ok=!strcmp(desired.session,argv[1]) && next.seq>desired.seq;
    if (ok) { snprintf(next.session,sizeof next.session,"%s",argv[1]); next.visible=true; next.received=now_ms();if(argc==16)memcpy(next.gear,desired.gear,sizeof next.gear); desired=next; }
    pthread_mutex_unlock(&mutex);
    if (!ok) { control_fail(r,"Wrong session or stale sequence"); return; }
    control_field_int(r,"queued_sequence",next.seq);
}
static void receive_equipment(int argc,char **argv,ControlReply *r) {
    uint64_t seq;GearPose next={0};
    if(argc!=17||!supported()||!token(argv[1])||!integer(argv[2],&seq)||strlen(argv[3])!=1||argv[3][0]<'0'||argv[3][0]>'5'){control_fail(r,"Invalid visual equipment");return;}
    int slot=argv[3][0]-'0';
    if(strcmp(argv[4],"-")){if(!gear_name(argv[4],slot)){control_fail(r,"Invalid visual model name");return;}snprintf(next.name,sizeof next.name,"%s",argv[4]);}
    for(int i=0;i<12;i++)if(!number(argv[5+i],&next.matrix[i],i%4==3?32:8)){control_fail(r,"Invalid equipment matrix");return;}
    next.seq=seq;next.received=now_ms();pthread_mutex_lock(&mutex);
    bool ok=!strcmp(desired.session,argv[1])&&desired.visible&&seq<=desired.seq&&seq>desired.gear[slot].seq;
    if(ok)desired.gear[slot]=next;pthread_mutex_unlock(&mutex);
    if(!ok){control_fail(r,"Wrong session or stale equipment");return;}control_field_bool(r,"queued",true);
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
    control_field_num(r,"requested_x",p.requested[0]);control_field_num(r,"requested_y",p.requested[1]);control_field_num(r,"requested_z",p.requested[2]);
    control_field_num(r,"observed_x",p.observed[0]);control_field_num(r,"observed_y",p.observed[1]);control_field_num(r,"observed_z",p.observed[2]);
    control_field_int(r,"animation_requests",p.animation_requests);
    control_field_int(r,"spawn_attempts",p.spawn_attempts);
    control_field_bool(r,"resource_fallback",p.resource_fallback);control_field_bool(r,"visual_only",p.visual_only);
    control_field_str(r,"first_error",p.first_error);
    control_field_str(r,"reference_status",p.reference_status);
    control_field_int(r,"actor_state",p.reference_status[0]?p.actor_state:-1);
    control_field_int(r,"activation_wait_ticks",p.activation_wait_ticks);
    control_field_str(r,"callback_phase","post-actor-jobs");
    control_field_int(r,"rejected_callbacks",p.rejected_callbacks);
    control_field_int(r,"equipment_active",p.equipment_active);control_field_str(r,"equipment_json",p.equipment_json[0]?p.equipment_json:"[]");
}
static void local_state(int argc,char **argv,ControlReply *r) {
    Track tracks[2];GearPose gear[GEAR_COUNT];float xyz[3],yaw;uint64_t count;bool ready;
    pthread_mutex_lock(&mutex);memcpy(tracks,local_tracks,sizeof tracks);memcpy(gear,local_gear,sizeof gear);memcpy(xyz,local_position,sizeof xyz);yaw=local_yaw;count=capture_count;ready=local_ready&&published.last_tick&&now_ms()-published.last_tick<1000;pthread_mutex_unlock(&mutex);
    control_field_bool(r,"ready",ready);control_field_num(r,"x",xyz[0]);control_field_num(r,"y",xyz[1]);control_field_num(r,"z",xyz[2]);
    control_field_num(r,"yaw",yaw);control_field_int(r,"capture_count",count);
    control_field_str(r,"upper",tracks[0].name);control_field_int(r,"upper_sequence",tracks[0].sequence);control_field_bool(r,"upper_force",tracks[0].force);
    control_field_str(r,"lower",tracks[1].name);control_field_int(r,"lower_sequence",tracks[1].sequence);control_field_bool(r,"lower_force",tracks[1].force);
    control_field_int(r,"upper_slot",tracks[0].slot);control_field_int(r,"lower_slot",tracks[1].slot);
    char json[4096];size_t used=0;json[used++]='[';
    for(int i=0;i<GEAR_COUNT;i++) {
        int n=snprintf(json+used,sizeof json-used,"%s{\"slot\":%d,\"name\":\"%s\",\"matrix\":[",i?",":"",i,gear[i].name);if(n<0||(size_t)n>=sizeof json-used-2){control_fail(r,"Equipment snapshot overflow");return;}used+=(size_t)n;
        for(int j=0;j<12;j++){n=snprintf(json+used,sizeof json-used,"%s%.6f",j?",":"",gear[i].matrix[j]);if(n<0||(size_t)n>=sizeof json-used-4){control_fail(r,"Equipment snapshot overflow");return;}used+=(size_t)n;}
        json[used++]=']';json[used++]='}';
    }
    json[used++]=']';json[used]=0;control_field_str(r,"equipment_json",json);
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
        {"hm_native_remote","<session> <seq> <x> <y> <z> <yaw> <action> <upper> <event> <force> <lower> <event> <force> <upper-slot> <lower-slot>","Queue remote Link pose",remote,0},
        {"hm_native_equipment","<session> <seq> <slot> <actor-or-dash> <12 matrix values>","Queue visual equipment without copying inventory",receive_equipment,0},
        {"hm_native_clear","<session> <seq>","Remove only HyruleM's remote actor",clear_remote,0},
        {"hm_world_status","","Loaded map-enemy health snapshot",world_status,CONTROL_PLAYER},
        {"hm_local_state","","Captured local upper/lower AS requests and facing",local_state,CONTROL_PLAYER},
        {"hm_enemy_health","<session> <hash> <actor> <sx> <sy> <sz> <life>","Queue map-enemy health convergence",receive_health,0},
    };
    control_register(cmds,(int)(sizeof cmds/sizeof *cmds));
}
