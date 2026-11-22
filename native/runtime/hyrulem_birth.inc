// Diagnostic capture at the native factory's successful initialization path.
// This runs in a worker: copy bounded metadata only, never call guest code.
enum { BIRTH_MAX=64 };
typedef struct {
    uint64_t sequence,actor,block,parent,parent_block,task,task_words[48];
    uint32_t generation,parent_generation,hash,parent_hash;
    unsigned parameters;
    char name[64],parent_name[64];
    float position[3];
} BirthEvent;
static pthread_mutex_t birth_mutex=PTHREAD_MUTEX_INITIALIZER;
static BirthEvent birth_events[BIRTH_MAX];static uint64_t birth_sequence;
static bool birth_name(uint64_t a,char name[64]){
    if(!range(a,0x7d0))return false;uint64_t p=LD64(a+0x10);
    if(!range(p,64)||!memchr((void *)(uintptr_t)p,0,64)||!clip_name((char *)(uintptr_t)p))return false;
    snprintf(name,64,"%s",(char *)(uintptr_t)p);return true;
}
enum {LOOT_PARENTS=256};
typedef struct{uint64_t actor,block,at;uint32_t generation;char session[17],world[25],scene[65];GroundParent identity;} LootContext;
static LootContext loot_parents[LOOT_PARENTS];static uint64_t loot_captured,loot_overflow;
static bool loot_scope(GroundMode *g){
    pthread_mutex_lock(&ground_mutex);*g=ground_mode;pthread_mutex_unlock(&ground_mutex);
    if(!g->heartbeat||now_ms()-g->heartbeat>=2500)return false;
    pthread_mutex_lock(&actor_world_mutex);bool valid=actor_world_mode.role==3&&actor_world_mode.heartbeat&&now_ms()-actor_world_mode.heartbeat<1500&&!strcmp(g->world,actor_world_mode.world)&&!strcmp(g->session,actor_world_mode.session)&&!strcmp(g->scene,actor_world_mode.scene);pthread_mutex_unlock(&actor_world_mutex);return valid;
}
static bool loot_scope_same(const LootContext *p,const GroundMode *g){return !strcmp(p->session,g->session)&&!strcmp(p->world,g->world)&&!strcmp(p->scene,g->scene);}
// Caller holds birth_mutex. All identities are weak and scoped to one world;
// the table never keeps a raw guest pointer valid after its generation changes.
static LootContext *loot_cache(uint64_t a,const GroundMode *g,const GroundParent *identity){
    uint64_t block=LD64(a+0x68);uint32_t gen=LD32(a+0x60);unsigned at=LOOT_PARENTS,oldest=0;
    if(!range(block,0x50)||gen==UINT32_MAX||LD32(block+0x40)!=gen||LD64(block+0x48)!=a)return NULL;
    for(unsigned i=0;i<LOOT_PARENTS;i++){
        LootContext *p=&loot_parents[i];if(p->actor==a&&p->block==block&&p->generation==gen&&loot_scope_same(p,g)){at=i;break;}
        if(p->at<loot_parents[oldest].at)oldest=i;
    }
    if(at==LOOT_PARENTS){at=oldest;memset(&loot_parents[at],0,sizeof loot_parents[at]);}
    LootContext *p=&loot_parents[at];if(identity){uint32_t ordinal=p->identity.ordinal;p->identity=*identity;p->identity.ordinal=ordinal;}
    p->actor=a;p->block=block;p->generation=gen;p->at=now_ms();snprintf(p->session,17,"%s",g->session);snprintf(p->world,25,"%s",g->world);snprintf(p->scene,65,"%s",g->scene);return p;
}
static void loot_parent_note(uint64_t a,const EnemyHealth *e){
    GroundMode g;if(!range(a,0x7d0)||!loot_scope(&g)||strcmp(e->scene,g.scene)||!e->hash||e->hash==UINT32_MAX||!ground_parent_name(e->name))return;
    GroundParent identity={.hash=e->hash};snprintf(identity.name,64,"%s",e->name);memcpy(identity.spawn,e->spawn,12);
    for(unsigned i=0;i<3;i++)if(!isfinite(identity.spawn[i])||fabsf(identity.spawn[i])>(i==1?50000:100000))return;
    pthread_mutex_lock(&birth_mutex);loot_cache(a,&g,&identity);pthread_mutex_unlock(&birth_mutex);
}
static LootContext *loot_find(uint64_t a,const GroundMode *g){
    if(!range(a,0x7d0))return NULL;uint64_t block=LD64(a+0x68);uint32_t gen=LD32(a+0x60);
    if(!range(block,0x50)||gen==UINT32_MAX||LD32(block+0x40)!=gen||LD64(block+0x48)!=a)return NULL;
    for(unsigned i=0;i<LOOT_PARENTS;i++){LootContext *p=&loot_parents[i];if(p->actor==a&&p->block==block&&p->generation==gen&&loot_scope_same(p,g))return p;}
    // A very fast native death can precede the first post-job map scan.
    char name[64];uint64_t map=LD64(a+0x7c8);if(!birth_name(a,name)||!ground_parent_name(name)||!range(map,0x70))return NULL;
    GroundParent identity={.hash=LD32(map+0x58)};if(!identity.hash||identity.hash==UINT32_MAX)return NULL;snprintf(identity.name,64,"%s",name);
    for(unsigned i=0;i<3;i++){identity.spawn[i]=read_float(map+0x5c+i*4);if(!isfinite(identity.spawn[i])||fabsf(identity.spawn[i])>(i==1?50000:100000))return NULL;}
    return loot_cache(a,g,&identity);
}
static void loot_bind_trunk(uint64_t a,uint64_t parent,const char *name){
    GroundMode g;if(!loot_scope(&g)||!ground_parent_name(name))return;
    pthread_mutex_lock(&birth_mutex);LootContext *root=loot_find(parent,&g);
    if(root&&!strncmp(root->identity.name,"Obj_Tree",8)&&!strstr(root->identity.name,"_Trunk")){
        GroundParent identity=root->identity;identity.ordinal=0;snprintf(identity.name,64,"%s",name);loot_cache(a,&g,&identity);
    }pthread_mutex_unlock(&birth_mutex);
}
// A trunk is a native simulation source with an explicit factory ancestry.
// Its network identity uses the standing tree's placement plus the trunk name;
// no synthetic map object is installed in the actor.
static bool loot_derived(uint64_t a,WorldActor *out){
    GroundMode g;char name[64];if(!birth_name(a,name)||!loot_scope(&g))return false;bool found=false;
    pthread_mutex_lock(&birth_mutex);for(unsigned i=0;i<LOOT_PARENTS;i++){
        LootContext *p=&loot_parents[i];size_t n=strlen(p->identity.name);
        if(p->actor==a&&!strcmp(name,p->identity.name)&&n>6&&!strcmp(p->identity.name+n-6,"_Trunk")&&loot_scope_same(p,&g)&&actor_world_weak(a,p->block,p->generation)){
            out->hash=p->identity.hash;memcpy(out->spawn,p->identity.spawn,12);found=true;break;
        }
    }pthread_mutex_unlock(&birth_mutex);return found;
}
static bool loot_capture(uint64_t a,uint64_t parent,GroundSource *source){
    GroundMode g;if(!loot_scope(&g)||!range(a,0x7d0)||!ground_actor(source->name)||strlen(source->name)>47)return false;
    uint64_t life=range(parent,0x7d0)?enemy_life(parent):0;if(!life||LD32(life)!=0)return false;
    uint64_t block=LD64(a+0x68);uint32_t gen=LD32(a+0x60);if(!actor_world_weak(a,block,gen))return false;
    for(unsigned i=0;i<3;i++)if(!isfinite(source->pos[i])||fabsf(source->pos[i])>(i==1?50000:100000))return false;
    pthread_mutex_lock(&ground_mutex);int at=-1;
    for(unsigned i=0;i<NATIVE_DROPS;i++){
        NativeDrop *d=&native_drops[i];if(native_drop_live(d,a)){pthread_mutex_unlock(&ground_mutex);return true;}
        if(d->active&&d->id&&!d->pickup_pending&&!native_drop_live(d,d->actor))d->active=false;
        if(!d->active&&at<0)at=(int)i;
    }
    if(at<0){loot_overflow++;pthread_mutex_unlock(&ground_mutex);return false;}
    pthread_mutex_lock(&birth_mutex);LootContext *context=loot_find(parent,&g);
    if(!context||context->identity.ordinal>=4096){pthread_mutex_unlock(&birth_mutex);pthread_mutex_unlock(&ground_mutex);return false;}
    source->parent=context->identity;source->parent.ordinal=++context->identity.ordinal;context->at=now_ms();pthread_mutex_unlock(&birth_mutex);
    source->local=UINT64_C(0x200000000)+ ++native_drop_sequence;source->native_drop=source->drop=true;source->hash=0;snprintf(source->origin,17,"%s",ground_origin);snprintf(source->scene,65,"%s",g.scene);
    NativeDrop *d=&native_drops[at];uint64_t reuse=d->fallback.storage;memset(d,0,sizeof *d);d->fallback.storage=reuse;d->active=true;d->actor=a;d->block=block;d->generation=gen;d->source=*source;loot_captured++;pthread_mutex_unlock(&ground_mutex);return true;
}

void rt_hyrulem_capture_birth(Cpu *c){
    if(!supported())return;const char *trace=getenv("HYRULEM_NATIVE_BIRTH_TRACE");bool tracing=trace&&!strcmp(trace,"1");GroundMode mode;if(!tracing&&!loot_scope(&mode))return;
    uint64_t task=c->x[21],a=c->x[22];BirthEvent e={0};
    if(!range(task,0x58)||!birth_name(a,e.name)||(!ground_actor(e.name)&&!strstr(e.name,"_Trunk")))return;
    e.actor=a;e.block=LD64(a+0x68);e.generation=LD32(a+0x60);
    if(!actor_world_weak(a,e.block,e.generation))return;
    // The factory has already copied this weak ActorIndex to child+0x748.
    // Task+0x40 is the inline parameter buffer, not the parent actor pointer.
    e.parent_block=LD64(task+0x48);e.parent_generation=LD32(task+0x50);
    if(range(e.parent_block,0x50)&&e.parent_generation!=UINT32_MAX&&LD32(e.parent_block+0x40)==e.parent_generation){
        uint64_t parent=LD64(e.parent_block+0x48);
        if(birth_name(parent,e.parent_name)&&LD64(parent+0x68)==e.parent_block&&LD32(parent+0x60)==e.parent_generation){e.parent=parent;uint64_t map=LD64(parent+0x7c8);if(range(map,0x70))e.parent_hash=LD32(map+0x58);}else e.parent_name[0]=0;
    }
    uint64_t map=LD64(a+0x7c8);if(range(map,0x70))e.hash=LD32(map+0x58);
    uint64_t params=LD64(task+0x40);if(range(params,2))e.parameters=LD16(params);
    for(unsigned i=0;i<3;i++){e.position[i]=read_float(a+0x3a4+i*16);if(!isfinite(e.position[i])||fabsf(e.position[i])>100000)return;}
    if(e.parent){
        if(strstr(e.name,"_Trunk"))loot_bind_trunk(a,e.parent,e.name);
        else if(!ground_weapon(e.name)){GroundSource source={0};if(strlen(e.name)<sizeof source.name){snprintf(source.name,sizeof source.name,"%s",e.name);memcpy(source.spawn,e.position,12);memcpy(source.pos,e.position,12);loot_capture(a,e.parent,&source);}}
    }
    if(!tracing)return;
    if(task>=GUEST_LO+0x100&&range(task-0x100,sizeof e.task_words)){e.task=task;for(unsigned i=0;i<48;i++)e.task_words[i]=LD64(task-0x100+i*8);}
    pthread_mutex_lock(&birth_mutex);e.sequence=++birth_sequence;birth_events[(e.sequence-1)%BIRTH_MAX]=e;pthread_mutex_unlock(&birth_mutex);
}
static void birth_reset(void){pthread_mutex_lock(&birth_mutex);memset(birth_events,0,sizeof birth_events);memset(loot_parents,0,sizeof loot_parents);birth_sequence=0;pthread_mutex_unlock(&birth_mutex);pthread_mutex_lock(&ground_mutex);loot_captured=loot_overflow=0;pthread_mutex_unlock(&ground_mutex);}
static void birth_status(int argc,char **argv,ControlReply *r){
    pthread_mutex_lock(&ground_mutex);uint64_t captured=loot_captured,overflow=loot_overflow;pthread_mutex_unlock(&ground_mutex);control_field_int(r,"world_loot_captured",captured);control_field_int(r,"world_loot_overflow",overflow);
    BirthEvent rows[BIRTH_MAX];uint64_t seq;pthread_mutex_lock(&birth_mutex);memcpy(rows,birth_events,sizeof rows);seq=birth_sequence;pthread_mutex_unlock(&birth_mutex);
    if(argc==2){
        uint64_t wanted;if(!integer(argv[1],&wanted)){control_fail(r,"Invalid birth sequence");return;}
        BirthEvent *e=NULL;for(unsigned i=0;i<BIRTH_MAX;i++)if(rows[i].sequence==wanted){e=&rows[i];break;}
        if(!e||!e->task){control_fail(r,"Birth task snapshot is unavailable");return;}
        char words[48*80+3],pointer[32];size_t n=1;words[0]='[';for(unsigned i=0;i<48;i++)n+=(size_t)snprintf(words+n,sizeof words-n,"%s{\"offset\":%d,\"value\":\"0x%llx\"}",i?",":"",(int)i*8-0x100,(unsigned long long)e->task_words[i]);words[n++]=']';words[n]=0;
        snprintf(pointer,sizeof pointer,"0x%llx",(unsigned long long)e->task);control_field_str(r,"task",pointer);control_field_str(r,"actor",e->name);control_field_str(r,"words_json",words);return;
    }
    char json[BIRTH_MAX*320+3];size_t used=1;json[0]='[';unsigned count=seq<BIRTH_MAX?(unsigned)seq:BIRTH_MAX;
    for(unsigned i=0;i<count;i++){
        BirthEvent *e=&rows[(seq-count+i)%BIRTH_MAX];
        int n=snprintf(json+used,sizeof json-used,"%s{\"sequence\":%llu,\"actor\":\"%s\",\"hash\":%u,\"parent\":\"%s\",\"parent_hash\":%u,\"parameters\":%u,\"position\":[%.3f,%.3f,%.3f]}",i?",":"",(unsigned long long)e->sequence,e->name,e->hash,e->parent_name,e->parent_hash,e->parameters,e->position[0],e->position[1],e->position[2]);
        if(n<0||(size_t)n>=sizeof json-used-2){control_fail(r,"Birth diagnostic overflow");return;}used+=(size_t)n;
    }json[used++]=']';json[used]=0;control_field_int(r,"sequence",seq);control_field_str(r,"births_json",json);
}
