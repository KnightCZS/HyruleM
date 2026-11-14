// Original, bounded ground-material experiment. No game assets or game code.
// Command handlers enqueue data. Actor and inventory calls run only after jobs.
enum { GROUND_MAX=32, GROUND_EVENTS=32 };
typedef struct { uint64_t local;uint32_t hash;bool drop;char origin[17],name[48];float spawn[3],pos[3]; } GroundSource;
typedef struct { uint64_t id,request;GroundSource source;int state;bool own; } GroundState;
typedef struct { char session[17],world[25];float zone[3];uint64_t heartbeat; } GroundMode;
typedef struct { uint64_t request,id;GroundSource source;bool done; } GroundEvent;
typedef struct { uint64_t actor,storage,last_seen;GroundSource source; } GroundLocal;
typedef struct { GearVisual visual;uint64_t request;int result;bool granted; } GroundReplica;
typedef struct { uint64_t actor,id,storage;GroundSource source;bool replica;int state; } GroundHook;
typedef struct{uint64_t request;int result;bool failed,handle;} GroundReceipt;
static GroundReceipt ground_receipts[GROUND_MAX];
typedef struct{uint64_t storage,id;GroundSource source;int state;} GroundPending;
static GroundPending ground_pending[GROUND_MAX];
static uint64_t ground_pending_actor(uint64_t storage){if(!storage)return 0;uint64_t link=LD64(storage+HANDLE);return range(link,0x18)?LD64(link+0x10):0;}
static pthread_mutex_t ground_mutex=PTHREAD_MUTEX_INITIALIZER;
static GroundMode ground_mode,ground_live;
static GroundState ground_state[GROUND_MAX];
static GroundEvent ground_events[GROUND_EVENTS];
static GroundHook ground_hooks[GROUND_MAX*2];static int ground_hook_count;
static GroundSource ground_seen[GROUND_MAX];static int ground_seen_count;
static GroundLocal ground_local[GROUND_MAX];
static GroundReplica ground_replica[GROUND_MAX];
static uint64_t ground_next_local,ground_next_request,ground_scan_at,ground_intercepts;
static bool ground_session_ready;
enum{GROUND_DROPS=8};
typedef struct{GearVisual visual;GroundSource source;uint64_t request,id;char world[25];bool active,confirmed;int terminal;} GroundDrop;
typedef struct{uint64_t request,id;char session[17],world[25];} GroundDropCommand;
typedef struct{uint64_t actor,storage,id;GroundSource source;char world[25];bool confirmed;} GroundDropHook;
static GroundDrop ground_drops[GROUND_DROPS];static GroundDropHook ground_drop_hooks[GROUND_DROPS];
static GroundDropCommand ground_drop_queued;static uint64_t ground_drop_sequence,ground_drop_last_request,ground_drop_last_id;
static int ground_drop_last_result;static GroundSource ground_drop_last_source;static bool ground_drop_busy;
static char ground_origin[17];
static bool ground_drop_cleanup(Cpu *);
static void ground_drop_tick(Cpu *,uint64_t,uint64_t,const GroundMode *,const GroundState *);
static bool ground_drop_source(uint64_t,const GroundMode *,GroundSource *);
static bool ground_source_equal(const GroundSource *a,const GroundSource *b);
static bool ground_world_token(const char *);


static bool ground_actor(const char *name) {
    const char *names[]={"Item_Fruit_A","Item_Fruit_B","Item_Fruit_F","Item_Mushroom_A","Item_PlantGet_A"};
    for(unsigned i=0;i<sizeof names/sizeof *names;i++)if(!strcmp(name,names[i]))return true;
    return false;
}
static bool ground_supported(void) {
    return world_supported()&&g_site.hm_item_pickup&&g_site.hm_item_remove&&g_site.hm_item_get&&g_site.hm_pouch_mgr&&g_site.pouch_page_count&&g_site.pouch_page_item;
}
static bool ground_index_live(uint64_t st,uint64_t actor) {
    if(!st||!LD8(st+INDEX+12))return false;uint64_t block=LD64(st+INDEX);uint32_t generation=LD32(st+INDEX+8);
    return range(block,0x50)&&generation!=UINT32_MAX&&LD32(block+0x40)==generation&&LD64(block+0x48)==actor;
}
static bool ground_read_name(uint64_t p,char out[48]) {
    if(!range(p,48)||!memchr((void *)(uintptr_t)p,0,48))return false;
    snprintf(out,48,"%s",(char *)(uintptr_t)p);return ground_actor(out);
}
static bool ground_in_zone(const float pos[3],const GroundMode *mode,float radius) {
    float d=0;for(int i=0;i<3;i++){if(!isfinite(pos[i]))return false;float v=pos[i]-mode->zone[i];d+=v*v;}return d<=radius*radius;
}
static bool ground_observe(uint64_t a,GroundSource *out) {
    if(!range(a,0x7d0)||LD8(a+0x64)!=1||!ground_read_name(LD64(a+0x10),out->name))return false;
    uint64_t map=LD64(a+0x7c8);out->hash=0;
    for(int i=0;i<3;i++)out->spawn[i]=out->pos[i]=read_float(a+0x3a4+i*16);
    if(range(map,0x70)) {out->hash=LD32(map+0x58);for(int i=0;i<3;i++)out->spawn[i]=read_float(map+0x5c+i*4);}
    if(out->hash==UINT32_MAX)return false;
    for(int i=0;i<3;i++)if(!isfinite(out->spawn[i])||fabsf(out->spawn[i])>100000||!isfinite(out->pos[i])||fabsf(out->pos[i])>100000)return false;
    return true;
}
static bool ground_same(const GroundSource *a,const GroundSource *b) {
    if(!a->hash||a->hash!=b->hash||strcmp(a->name,b->name))return false;
    for(int i=0;i<3;i++)if(fabsf(a->spawn[i]-b->spawn[i])>.003f)return false;return true;
}
static bool ground_source_equal(const GroundSource *a,const GroundSource *b){return ground_same(a,b)||(a->drop&&b->drop&&a->local==b->local&&!strcmp(a->origin,b->origin));}
// A pickup callback is inside a game job: copy metadata only, never call guest
// code, mutate inventory, or wait for the server from here. Skipping the award
// leaves the server reservation as the only path that can grant a shared item.
bool rt_hyrulem_item_pickup(Cpu *c) {
    if(!ground_supported()||!range(c->x[0],16)||!range(c->x[1],16))return false;
    uint64_t a=LD64(c->x[0]+8);GroundSource source={0};
    if(!ground_observe(a,&source))return false;
    char name[48];if(!ground_read_name(LD64(c->x[1]+8),name)||strcmp(name,source.name))return false;
    pthread_mutex_lock(&ground_mutex);
    GroundHook *known=NULL;for(int i=0;i<ground_hook_count;i++)if(ground_hooks[i].actor==a&&ground_index_live(ground_hooks[i].storage,a)&&!strcmp(ground_hooks[i].source.name,source.name)){known=&ground_hooks[i];break;}
    GroundHook pending={0};
    if(!known)for(int i=0;i<GROUND_MAX;i++)if(ground_pending[i].id&&ground_pending_actor(ground_pending[i].storage)==a&&!strcmp(ground_pending[i].source.name,source.name)){
        GroundPending *p=&ground_pending[i];pending=(GroundHook){a,p->id,p->storage,p->source,true,p->state};known=&pending;break;
    }
    if(!known)for(int i=0;i<GROUND_DROPS;i++){
        GroundDropHook *d=&ground_drop_hooks[i];if(d->storage&&((d->actor==a&&ground_index_live(d->storage,a))||ground_pending_actor(d->storage)==a)){
            pending=(GroundHook){a,d->id,d->storage,d->source,true,d->confirmed?0:1};known=&pending;break;
        }
    }
    bool fresh=ground_session_ready&&ground_mode.heartbeat&&now_ms()-ground_mode.heartbeat<2500&&!strcmp(ground_mode.world,ground_live.world)&&!strcmp(ground_mode.session,ground_live.session);
    if(!fresh||!ground_in_zone(source.pos,&ground_mode,90.f)) {bool block=known&&known->replica;pthread_mutex_unlock(&ground_mutex);return block;}
    if(!known&&!source.hash){pthread_mutex_unlock(&ground_mutex);return false;}
    if(known)source=known->source;
    uint64_t id=known?known->id:0;
    if(known&&known->state>=1){pthread_mutex_unlock(&ground_mutex);return true;}
    for(int i=0;i<GROUND_EVENTS;i++)if(!ground_events[i].done&&ground_events[i].request&&((id&&ground_events[i].id==id)||ground_source_equal(&ground_events[i].source,&source))){pthread_mutex_unlock(&ground_mutex);return true;}
    int slot=-1;for(int i=0;i<GROUND_EVENTS;i++)if(!ground_events[i].request||ground_events[i].done){slot=i;break;}
    if(slot<0){pthread_mutex_unlock(&ground_mutex);return id!=0;} // bounded; never bypass authority when busy
    uint64_t request=++ground_next_request;if(!source.local)source.local=UINT64_C(0x100000000)+request;
    ground_events[slot]=(GroundEvent){request,id,source,false};ground_intercepts++;
    pthread_mutex_unlock(&ground_mutex);return true;
}
static void ground_reset(void) {
    pthread_mutex_lock(&ground_mutex);
    memset(&ground_mode,0,sizeof ground_mode);memset(&ground_live,0,sizeof ground_live);memset(ground_state,0,sizeof ground_state);
    memset(ground_events,0,sizeof ground_events);memset(ground_receipts,0,sizeof ground_receipts);memset(ground_hooks,0,sizeof ground_hooks);ground_hook_count=ground_seen_count=0;
    memset(ground_local,0,sizeof ground_local);memset(ground_replica,0,sizeof ground_replica);memset(ground_pending,0,sizeof ground_pending);
    memset(ground_drops,0,sizeof ground_drops);memset(ground_drop_hooks,0,sizeof ground_drop_hooks);memset(&ground_drop_queued,0,sizeof ground_drop_queued);memset(&ground_drop_last_source,0,sizeof ground_drop_last_source);
    ground_drop_sequence=ground_drop_last_request=ground_drop_last_id=0;ground_drop_last_result=0;ground_drop_busy=false;ground_origin[0]=0;
    ground_next_local=ground_next_request=ground_scan_at=ground_intercepts=0;ground_session_ready=false;
    pthread_mutex_unlock(&ground_mutex);
}
static uint64_t ground_storage(void) {
    uint64_t st=(uint64_t)(uintptr_t)hle_alloc(BYTES);if(!range(st,BYTES))return 0;
    memset((void *)(uintptr_t)st,0,BYTES);ST32(st+INDEX+8,UINT32_MAX);strcpy((char *)(uintptr_t)(st+KEY),"@P");return st;
}
static void ground_pause(void){for(int i=0;i<GROUND_DROPS;i++)if(ground_drops[i].active)ground_drops[i].visual.started=now_ms();for(int i=0;i<GROUND_MAX;i++)if(ground_replica[i].visual.actor||ground_replica[i].visual.spawning)ground_replica[i].visual.started=now_ms();}
static bool ground_cleanup(Cpu *c) {
    bool done=ground_drop_cleanup(c);
    for(int i=0;i<GROUND_MAX;i++)if(!remove_gear(c,&ground_replica[i].visual))done=false;
    if(!done)return false;
    for(int i=0;i<GROUND_MAX;i++) {
        GroundLocal *l=&ground_local[i];if(l->storage)call(c,g_site.actor_index_release,l->storage+INDEX,0,0,0,0,0,0);
        l->actor=0;l->source.local=0;
    }
    pthread_mutex_lock(&ground_mutex);ground_hook_count=ground_seen_count=0;memset(ground_pending,0,sizeof ground_pending);ground_session_ready=false;pthread_mutex_unlock(&ground_mutex);
    return true;
}
static void ground_scan(Cpu *c,uint64_t local,const GroundMode *mode) {
    uint64_t now=now_ms();if(now-ground_scan_at<200)return;ground_scan_at=now;
    uint64_t creator=LD64(g_site.hm_creator);if(!range(creator,0xe8)||LD32(creator+0x4c)!=0x7b8)return;
    // Allocate outside the creator mutex. Index assignment retains identities;
    // it does not resolve access or execute the actor while walking this list.
    for(int i=0;i<GROUND_MAX;i++)if(!ground_local[i].storage)ground_local[i].storage=ground_storage();
    call(c,g_site.pouch_lock,creator+0xa8,0,0,0,0,0,0);
    uint64_t head=creator+0x38,node=LD64(head);unsigned count=LD32(creator+0x48);if(count>8192)count=0;
    for(unsigned n=0;node!=head&&n<count;n++) {
        if(!range(node,16)||node<0x7b8)break;uint64_t a=node-0x7b8;node=LD64(node);if(a==local)continue;
        bool replica=false;for(int i=0;i<GROUND_MAX;i++)if(ground_replica[i].visual.actor==a||(ground_replica[i].visual.spawning&&ground_pending_actor(ground_replica[i].visual.storage)==a))replica=true;if(replica)continue;
        GroundSource source={0};if(!ground_observe(a,&source)||!ground_in_zone(source.pos,mode,90))continue;
        if(!source.hash&&!ground_drop_source(a,mode,&source))continue;
        GroundLocal *at=NULL;for(int i=0;i<GROUND_MAX;i++)if(ground_local[i].actor==a){if(ground_index_live(ground_local[i].storage,a)){at=&ground_local[i];break;}call(c,g_site.actor_index_release,ground_local[i].storage+INDEX,0,0,0,0,0,0);ground_local[i].actor=0;}
        if(!at) {
            for(int i=0;i<GROUND_MAX;i++)if(!ground_local[i].actor&&ground_local[i].storage){at=&ground_local[i];break;}
            if(!at||!call(c,g_site.actor_index_assign,at->storage+INDEX,a,1,0,0,0,0))continue;
            if(!source.drop)source.local=++ground_next_local;at->actor=a;
        } else if(!source.drop)source.local=at->source.local;
        at->source=source;at->last_seen=now;
    }
    call(c,g_site.pouch_unlock,creator+0xa8,0,0,0,0,0,0);
    for(int i=0;i<GROUND_MAX;i++) {
        GroundLocal *l=&ground_local[i];if(l->actor&&now-l->last_seen>3000){call(c,g_site.actor_index_release,l->storage+INDEX,0,0,0,0,0,0);l->actor=0;l->source.local=0;}
    }
}
static int ground_count(Cpu *c,uint64_t mgr,const char *name) {
    int tabs=(int32_t)LD32(mgr+0x44498);if(tabs<1||tabs>50||LD32(mgr+0x78)+LD32(mgr+0x90)!=420)return -1;
    int total=0;
    for(int tab=0;tab<tabs;tab++) {
        int count=(int)call(c,g_site.pouch_page_count,mgr,tab,0,0,0,0,0);if(count<0||count>20)return -1;
        for(int slot=0;slot<count;slot++) {
            uint64_t p=call(c,g_site.pouch_page_item,mgr,tab,slot,0,0,0,0);if(!p)continue;
            if(!range(p,0x38))return -1;char current[48];if(!ground_read_name(LD64(p+0x30),current)||strcmp(current,name))continue;
            int value=(int32_t)LD32(p+0x20);if(value<1||value>999||!LD8(p+0x25))return -1;total+=value;
        }
    }
    return total;
}
static void ground_drop_publish(GroundDrop *d,int result){
    d->terminal=result;
    pthread_mutex_lock(&ground_mutex);ground_drop_last_request=d->request;ground_drop_last_id=d->id;ground_drop_last_result=result;ground_drop_last_source=d->source;ground_drop_busy=false;pthread_mutex_unlock(&ground_mutex);
}
static void ground_drop_hooks_publish(void){
    pthread_mutex_lock(&ground_mutex);
    for(int i=0;i<GROUND_DROPS;i++){GroundDrop *d=&ground_drops[i];GroundDropHook *h=&ground_drop_hooks[i];memset(h,0,sizeof *h);if(d->active){h->actor=d->visual.actor;h->storage=d->visual.storage;h->id=d->id;h->source=d->source;h->confirmed=d->confirmed;snprintf(h->world,sizeof h->world,"%s",d->world);}}
    pthread_mutex_unlock(&ground_mutex);
}
static bool ground_drop_cleanup(Cpu *c){
    bool done=true;for(int i=0;i<GROUND_DROPS;i++)if(ground_drops[i].active){if(!remove_gear(c,&ground_drops[i].visual))done=false;else ground_drops[i].active=false;}
    if(done){pthread_mutex_lock(&ground_mutex);if(ground_drop_queued.request)ground_drop_last_result=-1;memset(&ground_drop_queued,0,sizeof ground_drop_queued);ground_drop_busy=false;pthread_mutex_unlock(&ground_mutex);}
    ground_drop_hooks_publish();return done;
}
static bool ground_drop_source(uint64_t actor,const GroundMode *mode,GroundSource *s){
    for(int i=0;i<GROUND_DROPS;i++){GroundDrop *d=&ground_drops[i];if(d->active&&d->confirmed&&d->visual.actor==actor&&!strcmp(d->world,mode->world)){
        float pos[3];memcpy(pos,s->pos,sizeof pos);*s=d->source;memcpy(s->pos,pos,sizeof pos);return true;
    }}return false;
}
static void ground_drop_tick(Cpu *c,uint64_t player,uint64_t vt,const GroundMode *mode,const GroundState *states){
    GroundDropCommand queued;pthread_mutex_lock(&ground_mutex);queued=ground_drop_queued;memset(&ground_drop_queued,0,sizeof ground_drop_queued);pthread_mutex_unlock(&ground_mutex);
    if(queued.request){
        GroundDrop *d=NULL;for(int i=0;i<GROUND_DROPS;i++)if(!ground_drops[i].active){d=&ground_drops[i];break;}
        if(d){uint64_t st=d->visual.storage;memset(d,0,sizeof *d);d->visual.storage=st?st:ground_storage();d->request=queued.request;d->id=queued.id;d->source=states[d->id-1].source;d->active=true;snprintf(d->world,sizeof d->world,"%s",queued.world);}
        else{pthread_mutex_lock(&ground_mutex);ground_drop_last_result=-1;ground_drop_busy=false;pthread_mutex_unlock(&ground_mutex);}
    }
    for(int i=0;i<GROUND_DROPS;i++){
        GroundDrop *d=&ground_drops[i];GearVisual *g=&d->visual;if(!d->active)continue;
        if(strcmp(d->world,mode->world)){if(remove_gear(c,g))d->active=false;continue;}
        if(d->terminal&&!d->confirmed){if(remove_gear(c,g))d->active=false;continue;}
        if(d->confirmed&&(states[d->id-1].state==2||states[d->id-1].state==3||states[d->id-1].state==5)){if(remove_gear(c,g))d->active=false;continue;}
        if(d->confirmed){ActorRef ref=resolve_at(c,g->storage,g->actor);if(ref.ptr)release_at(c,g->storage);else if(ref.invalid){call(c,g_site.actor_index_release,g->storage+INDEX,0,0,0,0,0,0);g->actor=0;d->active=false;}continue;}
        float pos[3];for(int j=0;j<3;j++)pos[j]=read_float(player+0x3a4+j*16);float distance=0;for(int j=0;j<3;j++){float delta=pos[j]-d->source.pos[j];distance+=delta*delta;}
        if(!g->storage||!isfinite(distance)||distance>16||!ground_in_zone(d->source.pos,mode,90)){ground_drop_publish(d,-1);if(remove_gear(c,g))d->active=false;continue;}
        if(g->spawning){
            if(call(c,g_site.hm_handle_failed,g->storage+HANDLE,0,0,0,0,0,0)||now_ms()-g->started>15000){ground_drop_publish(d,-1);if(remove_gear(c,g))d->active=false;continue;}
            if(!call(c,g_site.hm_handle_ready,g->storage+HANDLE,0,0,0,0,0,0))continue;
            uint64_t a=call(c,g_site.hm_handle_get,g->storage+HANDLE,0,0,0,0,0,0);
            if(!range(a,0x7d0)||!call(c,g_site.actor_index_assign,g->storage+INDEX,a,1,0,0,0,0)){ground_drop_publish(d,-1);if(remove_gear(c,g))d->active=false;continue;}
            g->actor=a;uint64_t awake=call(c,g_site.hm_handle_wake,g->storage+HANDLE,0,0,0,0,0,0);g->spawning=false;if(awake!=a){ground_drop_publish(d,-1);if(remove_gear(c,g))d->active=false;continue;}
        }
        if(!g->actor){
            uint64_t creator=LD64(g_site.hm_creator),manager=LD64(g_site.hm_heap_mgr);if(!range(creator,0x60)||!range(manager,0x30)||LD8(creator+0x59))continue;uint64_t heap=LD64(manager+0x28);if(!range(heap,8))continue;
            uint64_t st=g->storage;memset((void *)(uintptr_t)(st+HANDLE),0,16);memset((void *)(uintptr_t)(st+PARAMS),0,0xd8);memcpy((void *)(uintptr_t)(st+VECTOR),d->source.pos,12);ST64(st+SSTR,vt);ST64(st+SSTR+8,st+KEY);snprintf((char *)(uintptr_t)(st+NAME),64,"%s",d->source.name);
            ground_drop_hooks_publish();call(c,g_site.hm_param_add,st+PARAMS+0x10,st+VECTOR,st+SSTR,12,4,0,0);
            if(!call(c,g_site.hm_create,creator,st+NAME,heap,st+HANDLE,st+PARAMS,0,1)){ground_drop_publish(d,-1);d->active=false;continue;}g->spawning=true;g->started=now_ms();continue;
        }
        ActorRef ref=resolve_at(c,g->storage,g->actor);if(!ref.ptr||ref.state!=1){if(ref.ptr)release_at(c,g->storage);if(ref.invalid||now_ms()-g->started>15000){ground_drop_publish(d,-1);if(remove_gear(c,g))d->active=false;}continue;}
        // Confirm an active actor near the prepared location before any debit.
        float actual[3],offset=0;for(int j=0;j<3;j++){actual[j]=read_float(ref.ptr+0x3a4+j*16);float delta=actual[j]-d->source.pos[j];offset+=delta*delta;}
        if(!isfinite(offset)||offset>16){release_at(c,g->storage);if(now_ms()-g->started>15000){ground_drop_publish(d,-1);if(remove_gear(c,g))d->active=false;}continue;}
        uint64_t mgr=LD64(g_site.hm_pouch_mgr);int result=-1;
        if(range(mgr,0x4449c)){
            call(c,g_site.pouch_lock,mgr+0x28,0,0,0,0,0,0);int before=ground_count(c,mgr,"Item_Fruit_A");
            if(before>0){snprintf((char *)(uintptr_t)(g->storage+NAME),64,"Item_Fruit_A");ST64(g->storage+SSTR,vt);ST64(g->storage+SSTR+8,g->storage+NAME);call(c,g_site.hm_item_remove,mgr,g->storage+SSTR,1,0,0,0,0);int after=ground_count(c,mgr,"Item_Fruit_A");result=after==before-1?1:after==before?-1:2;}
            call(c,g_site.pouch_unlock,mgr+0x28,0,0,0,0,0,0);
        }
        if(result==1){memcpy(d->source.pos,actual,sizeof actual);d->confirmed=true;}
        release_at(c,g->storage);ground_drop_publish(d,result);
        if(result!=1){if(remove_gear(c,g))d->active=false;}
    }
    ground_drop_hooks_publish();
}
static void ground_drop_command(int argc,char **argv,ControlReply *r){
    uint64_t id,request;if(argc!=5||!token(argv[1])||!ground_world_token(argv[2])||!integer(argv[3],&id)||id>GROUND_MAX||!integer(argv[4],&request)){control_fail(r,"Invalid apple drop request");return;}
    pthread_mutex_lock(&ground_mutex);
    bool valid=!strcmp(ground_mode.session,argv[1])&&!strcmp(ground_mode.world,argv[2])&&ground_mode.heartbeat&&now_ms()-ground_mode.heartbeat<2500;
    GroundState *s=&ground_state[id-1];valid=valid&&s->id==id&&s->own&&s->state==4&&s->source.drop&&s->source.local==request&&!strcmp(s->source.origin,ground_origin)&&!strcmp(s->source.name,"Item_Fruit_A");
    if(valid&&request>ground_drop_sequence){if(ground_drop_busy||ground_drop_queued.request)valid=false;else{ground_drop_sequence=request;ground_drop_last_request=request;ground_drop_last_id=id;ground_drop_last_result=0;ground_drop_busy=true;ground_drop_queued.request=request;ground_drop_queued.id=id;snprintf(ground_drop_queued.session,sizeof ground_drop_queued.session,"%s",argv[1]);snprintf(ground_drop_queued.world,sizeof ground_drop_queued.world,"%s",argv[2]);}}
    pthread_mutex_unlock(&ground_mutex);if(!valid){control_fail(r,"Drop not prepared by the server, or another drop is pending");return;}control_field_bool(r,"queued",true);
}

static int ground_grant(Cpu *c,const GroundState *s,uint64_t vt,uint64_t st) {
    uint64_t mgr=LD64(g_site.hm_pouch_mgr);if(!range(mgr,0x4449c)||!vt||!st)return -1;
    call(c,g_site.pouch_lock,mgr+0x28,0,0,0,0,0,0);
    int before=ground_count(c,mgr,s->source.name);int result=-1;
    if(before>=0&&before<999) {
        snprintf((char *)(uintptr_t)(st+NAME),64,"%s",s->source.name);ST64(st+SSTR,vt);ST64(st+SSTR+8,st+NAME);
        call(c,g_site.hm_item_get,mgr,st+SSTR,1,0,0,0,0);
        int after=ground_count(c,mgr,s->source.name);result=after==before+1?1:after==before?-1:0; // an ambiguous post-read can never release the reservation
    }
    call(c,g_site.pouch_unlock,mgr+0x28,0,0,0,0,0,0);return result;
}
static bool ground_is_local(const GroundState *s,const GroundLocal *l) {
    return l->actor&&(ground_same(&s->source,&l->source)||(s->own&&ground_source_equal(&s->source,&l->source)));
}
static bool ground_delete_local(Cpu *c,GroundLocal *l) {
    ActorRef ref=resolve_at(c,l->storage,l->actor);
    if(ref.ptr){call(c,g_site.hm_delete_actor,ref.ptr,0,0,0,0,0,0);release_at(c,l->storage);}
    else if(!ref.invalid)return false;
    call(c,g_site.actor_index_release,l->storage+INDEX,0,0,0,0,0,0);l->actor=0;l->source.local=0;return true;
}
static void ground_tick(Cpu *c,uint64_t local,uint64_t vt) {
    if(!ground_supported())return;
    GroundMode mode;GroundState states[GROUND_MAX];
    pthread_mutex_lock(&ground_mutex);mode=ground_mode;memcpy(states,ground_state,sizeof states);pthread_mutex_unlock(&ground_mutex);
    bool fresh=mode.heartbeat&&now_ms()-mode.heartbeat<2500;
    if(!fresh||strcmp(mode.world,ground_live.world)||strcmp(mode.session,ground_live.session)) {
        if(!ground_cleanup(c))return;
        if(!fresh)return;
        pthread_mutex_lock(&ground_mutex);ground_live=mode;pthread_mutex_unlock(&ground_mutex);
        for(int i=0;i<GROUND_MAX;i++){ground_replica[i].request=0;ground_replica[i].result=0;ground_replica[i].granted=false;ground_replica[i].visual.failed=false;ground_replica[i].visual.name[0]=0;}
        pthread_mutex_lock(&ground_mutex);memset(ground_events,0,sizeof ground_events);pthread_mutex_unlock(&ground_mutex);
    }
    ground_drop_tick(c,local,vt,&mode,states);
    float pos[3];for(int i=0;i<3;i++)pos[i]=read_float(local+0x3a4+i*16);if(!ground_in_zone(pos,&mode,95))return;
    ground_scan(c,local,&mode);
    GroundHook hooks[GROUND_MAX*2];int nh=0;GroundSource seen[GROUND_MAX];int ns=0;
    for(int i=0;i<GROUND_MAX;i++) {
        GroundState *s=&states[i];GroundReplica *r=&ground_replica[i];GearVisual *g=&r->visual;bool matched=false;
        if(!s->id)continue;
        for(int j=0;j<GROUND_DROPS;j++)if(ground_drops[j].active&&ground_drops[j].confirmed&&ground_drops[j].id==s->id)matched=true;
        if(!g->storage)g->storage=ground_storage();
        pthread_mutex_lock(&ground_mutex);ground_pending[i]=(GroundPending){g->storage,s->id,s->source,s->state};pthread_mutex_unlock(&ground_mutex);
        if(s->request&&!r->granted&&r->request!=s->request) {
            bool valid=false;pthread_mutex_lock(&ground_mutex);
            for(int j=0;j<GROUND_EVENTS;j++)if(!ground_events[j].done&&ground_events[j].request==s->request&&!strcmp(ground_events[j].source.name,s->source.name)&&(ground_events[j].id==s->id||ground_same(&ground_events[j].source,&s->source)||ground_source_equal(&ground_events[j].source,&s->source))){valid=true;break;}
            pthread_mutex_unlock(&ground_mutex);
            if(valid){r->request=s->request;r->result=ground_grant(c,s,vt,g->storage);r->granted=r->result==1;}
        }
        for(int j=0;j<GROUND_MAX;j++)if(ground_is_local(s,&ground_local[j])) {
            GroundLocal *l=&ground_local[j];ActorRef ref=resolve_at(c,l->storage,l->actor);
            bool live=ref.ptr&&ref.state==1;if(ref.ptr)release_at(c,l->storage);
            if(s->state==2||s->state==3||s->state==5||r->granted){if(live)ground_delete_local(c,l);continue;}
            if(live){matched=true;if(nh<GROUND_MAX*2)hooks[nh++]=(GroundHook){l->actor,s->id,l->storage,l->source,false,(s->state==4&&l->source.drop)?0:s->state};}
        }
        if(s->state!=0||r->granted||matched) {if(g->actor||g->spawning)remove_gear(c,g);continue;}
        if(!g->storage||g->failed)continue;
        if(g->spawning) {
            if(call(c,g_site.hm_handle_failed,g->storage+HANDLE,0,0,0,0,0,0)||now_ms()-g->started>15000){g->failed=true;remove_gear(c,g);continue;}
            if(!call(c,g_site.hm_handle_ready,g->storage+HANDLE,0,0,0,0,0,0))continue;
            uint64_t a=call(c,g_site.hm_handle_get,g->storage+HANDLE,0,0,0,0,0,0);
            if(!range(a,0x7d0)||!call(c,g_site.actor_index_assign,g->storage+INDEX,a,1,0,0,0,0)){g->failed=true;remove_gear(c,g);continue;}
            g->actor=a;uint64_t awake=call(c,g_site.hm_handle_wake,g->storage+HANDLE,0,0,0,0,0,0);g->spawning=false;
            if(awake!=a){g->failed=true;remove_gear(c,g);continue;}
        }
        if(!g->actor) {
            uint64_t creator=LD64(g_site.hm_creator),manager=LD64(g_site.hm_heap_mgr);if(!range(creator,0x60)||!range(manager,0x30)||LD8(creator+0x59))continue;uint64_t heap=LD64(manager+0x28);if(!range(heap,8))continue;
            uint64_t st=g->storage;memset((void *)(uintptr_t)(st+HANDLE),0,16);memset((void *)(uintptr_t)(st+PARAMS),0,0xd8);memcpy((void *)(uintptr_t)(st+VECTOR),s->source.pos,12);ST64(st+SSTR,vt);ST64(st+SSTR+8,st+KEY);
            call(c,g_site.hm_param_add,st+PARAMS+0x10,st+VECTOR,st+SSTR,12,4,0,0);snprintf((char *)(uintptr_t)(st+NAME),64,"%s",s->source.name);
            if(!call(c,g_site.hm_create,creator,st+NAME,heap,st+HANDLE,st+PARAMS,0,1)){g->failed=true;continue;}g->spawning=true;g->started=now_ms();continue;
        }
        ActorRef ref=resolve_at(c,g->storage,g->actor);
        if(ref.ptr&&ref.state==1) {
            float matrix[12]={1,0,0,s->source.pos[0],0,1,0,s->source.pos[1],0,0,1,s->source.pos[2]};memcpy((void *)(uintptr_t)(g->storage+MATRIX),matrix,sizeof matrix);
            call(c,g_site.hm_set_mtx,ref.ptr,g->storage+MATRIX,0,0,0,0,0);call(c,g_site.hm_set_mtx,ref.ptr,g->storage+MATRIX,1,0,0,0,0);
            if(nh<GROUND_MAX*2)hooks[nh++]=(GroundHook){g->actor,s->id,g->storage,s->source,true,0};g->started=now_ms();
        }
        if(ref.ptr)release_at(c,g->storage);
        else if(ref.invalid||now_ms()-g->started>15000){remove_gear(c,g);g->failed=true;}
    }
    for(int i=0;i<GROUND_MAX;i++)if(ground_local[i].actor&&now_ms()-ground_local[i].last_seen<1000){
        GroundLocal *l=&ground_local[i];seen[ns++]=l->source;
        bool bound=false;for(int j=0;j<nh;j++)if(hooks[j].actor==l->actor)bound=true;
        if(!bound&&nh<GROUND_MAX*2)hooks[nh++]=(GroundHook){l->actor,0,l->storage,l->source,false,0};
    }
    pthread_mutex_lock(&ground_mutex);memcpy(ground_hooks,hooks,sizeof *hooks*nh);ground_hook_count=nh;memcpy(ground_seen,seen,sizeof *seen*ns);ground_seen_count=ns;for(int i=0;i<GROUND_MAX;i++)ground_receipts[i]=(GroundReceipt){ground_replica[i].request,ground_replica[i].result,ground_replica[i].visual.failed,ground_replica[i].visual.actor!=0};ground_session_ready=true;pthread_mutex_unlock(&ground_mutex);
}
static bool ground_world_token(const char *s){if(strlen(s)!=24)return false;for(int i=0;i<24;i++)if(!((s[i]>='0'&&s[i]<='9')||(s[i]>='a'&&s[i]<='f')))return false;return true;}
static void ground_open(int argc,char **argv,ControlReply *r) {
    GroundMode m={0};if(argc!=6||!ground_supported()||!token(argv[1])||!ground_world_token(argv[2])||!number(argv[3],&m.zone[0],100000)||!number(argv[4],&m.zone[1],50000)||!number(argv[5],&m.zone[2],100000)){control_fail(r,"Invalid ground-material session");return;}
    pthread_mutex_lock(&mutex);bool ok=!strcmp(desired.session,argv[1]);pthread_mutex_unlock(&mutex);if(!ok){control_fail(r,"Wrong native session");return;}
    snprintf(m.session,sizeof m.session,"%s",argv[1]);snprintf(m.world,sizeof m.world,"%s",argv[2]);m.heartbeat=now_ms();
    pthread_mutex_lock(&ground_mutex);if(strcmp(ground_mode.world,m.world)||strcmp(ground_mode.session,m.session))memset(ground_state,0,sizeof ground_state);ground_mode=m;if(!ground_origin[0])snprintf(ground_origin,sizeof ground_origin,"%s",argv[1]);pthread_mutex_unlock(&ground_mutex);control_field_bool(r,"queued",true);
}
static void ground_close(int argc,char **argv,ControlReply *r) {
    if(argc!=3||!token(argv[1])||!ground_world_token(argv[2])){control_fail(r,"Invalid ground-material session");return;}
    pthread_mutex_lock(&ground_mutex);if(!strcmp(ground_mode.session,argv[1])&&!strcmp(ground_mode.world,argv[2]))ground_mode.heartbeat=0;pthread_mutex_unlock(&ground_mutex);control_field_bool(r,"queued",true);
}
static void ground_update(int argc,char **argv,ControlReply *reply) {
    GroundState s={0};uint64_t hash,local,request;
    if(argc!=18||!token(argv[1])||!ground_world_token(argv[2])||!integer(argv[3],&s.id)||s.id>GROUND_MAX||!ground_actor(argv[4])||(!integer(argv[5],&hash)&&strcmp(argv[5],"0"))||hash>=UINT32_MAX||(!integer(argv[12],&local))||(!integer(argv[15],&request)&&strcmp(argv[15],"0"))||strlen(argv[13])!=1||argv[13][0]<'0'||argv[13][0]>'1'||strlen(argv[14])!=1||argv[14][0]<'0'||argv[14][0]>'5'||strlen(argv[16])!=1||argv[16][0]<'0'||argv[16][0]>'1'||(strcmp(argv[17],"-")&&!token(argv[17]))){control_fail(reply,"Invalid ground-material state");return;}
    for(int i=0;i<6;i++)if(!number(argv[6+i],i<3?&s.source.spawn[i]:&s.source.pos[i-3],100000)){control_fail(reply,"Invalid ground-material position");return;}
    s.source.drop=argv[16][0]=='1';if(s.source.drop){if(hash||!token(argv[17])){control_fail(reply,"Invalid drop identity");return;}snprintf(s.source.origin,sizeof s.source.origin,"%s",argv[17]);}else if(!hash){control_fail(reply,"Mapless natural material is outside this experiment");return;}
    s.source.hash=(uint32_t)hash;s.source.local=local;snprintf(s.source.name,sizeof s.source.name,"%s",argv[4]);s.own=argv[13][0]=='1';s.state=argv[14][0]-'0';s.request=request;
    pthread_mutex_lock(&ground_mutex);bool ok=!strcmp(ground_mode.session,argv[1])&&!strcmp(ground_mode.world,argv[2]);GroundState *old=&ground_state[s.id-1];if(ok&&old->id&&strcmp(old->source.name,s.source.name))ok=false;if(ok)*old=s;pthread_mutex_unlock(&ground_mutex);if(!ok){control_fail(reply,"Wrong material identity/session");return;}control_field_bool(reply,"queued",true);
}
static void ground_deny(int argc,char **argv,ControlReply *r) {
    uint64_t request;if(argc!=4||!token(argv[1])||!ground_world_token(argv[2])||!integer(argv[3],&request)){control_fail(r,"Invalid material request");return;}
    pthread_mutex_lock(&ground_mutex);if(!strcmp(ground_mode.session,argv[1])&&!strcmp(ground_mode.world,argv[2]))for(int i=0;i<GROUND_EVENTS;i++)if(ground_events[i].request==request)ground_events[i].done=true;pthread_mutex_unlock(&ground_mutex);control_field_bool(r,"queued",true);
}
static int ground_source_json(char *out,size_t n,const GroundSource *s) {
    return snprintf(out,n,"{\"local\":%llu,\"hash\":%u,\"drop\":%s,\"origin\":\"%s\",\"actor\":\"%s\",\"spawn\":{\"X\":%.3f,\"Y\":%.3f,\"Z\":%.3f},\"position\":{\"X\":%.3f,\"Y\":%.3f,\"Z\":%.3f}}",(unsigned long long)s->local,s->hash,s->drop?"true":"false",s->origin,s->name,s->spawn[0],s->spawn[1],s->spawn[2],s->pos[0],s->pos[1],s->pos[2]);
}
static void ground_status(int argc,char **argv,ControlReply *r) {
    GroundSource seen[GROUND_MAX];GroundEvent events[GROUND_EVENTS];int n;uint64_t count;GroundSource drop_source;uint64_t drop_request,drop_id,drop_sequence;int drop_result;char origin[17];GroundReceipt receipts[GROUND_MAX];
    pthread_mutex_lock(&ground_mutex);n=ground_seen_count;memcpy(seen,ground_seen,sizeof seen);memcpy(events,ground_events,sizeof events);count=ground_intercepts;memcpy(receipts,ground_receipts,sizeof receipts);drop_request=ground_drop_last_request;drop_id=ground_drop_last_id;drop_result=ground_drop_last_result;drop_source=ground_drop_last_source;drop_sequence=ground_drop_sequence;snprintf(origin,sizeof origin,"%s",ground_origin);pthread_mutex_unlock(&ground_mutex);
    char json[20000];size_t used=1;json[0]='[';
    for(int i=0;i<n;i++){if(i)json[used++]=',';int k=ground_source_json(json+used,sizeof json-used,&seen[i]);if(k<0||(size_t)k>=sizeof json-used-2){control_fail(r,"Materials overflow");return;}used+=(size_t)k;}json[used++]=']';json[used]=0;control_field_str(r,"items_json",json);
    used=1;json[0]='[';bool comma=false;
    for(int i=0;i<GROUND_EVENTS;i++)if(events[i].request&&!events[i].done){int k=snprintf(json+used,sizeof json-used,"%s{\"request\":%llu,\"item\":%llu,\"source\":",comma?",":"",(unsigned long long)events[i].request,(unsigned long long)events[i].id);if(k<0||(size_t)k>=sizeof json-used-2){control_fail(r,"Events overflow");return;}used+=(size_t)k;k=ground_source_json(json+used,sizeof json-used,&events[i].source);if(k<0||(size_t)k>=sizeof json-used-3){control_fail(r,"Events overflow");return;}used+=(size_t)k;json[used++]='}';comma=true;}
    json[used++]=']';json[used]=0;control_field_str(r,"events_json",json);
    used=1;json[0]='[';comma=false;
    for(int i=0;i<GROUND_MAX;i++)if(receipts[i].request&&receipts[i].result){int k=snprintf(json+used,sizeof json-used,"%s{\"item\":%d,\"request\":%llu,\"applied\":%s}",comma?",":"",i+1,(unsigned long long)receipts[i].request,receipts[i].result==1?"true":"false");if(k<0||(size_t)k>=sizeof json-used-2){control_fail(r,"Receipts overflow");return;}used+=(size_t)k;comma=true;}
    json[used++]=']';json[used]=0;control_field_str(r,"receipts_json",json);control_field_int(r,"pickup_intercepts",count);control_field_bool(r,"experimental",true);
    int pending=0,failed=0,handles=0;for(int i=0;i<GROUND_MAX;i++){if(receipts[i].request&&!receipts[i].result)pending++;failed+=receipts[i].failed;handles+=receipts[i].handle;}
    control_field_int(r,"drop_request",drop_request);control_field_int(r,"drop_item",drop_id);control_field_int(r,"drop_result",drop_result);control_field_int(r,"drop_sequence",drop_sequence);control_field_str(r,"drop_origin",origin);
    if(drop_source.local){ground_source_json(json,sizeof json,&drop_source);control_field_str(r,"drop_source_json",json);}else control_field_str(r,"drop_source_json","{}");
    control_field_int(r,"pending_inventory_checks",pending);control_field_int(r,"replica_failures",failed);control_field_int(r,"replica_handles",handles);control_field_int(r,"source_materials",n);
}
