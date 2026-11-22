// Native requests cross actor workers. Carry copied ancestry in a bounded
// sidecar keyed by the real request object, never by position or allocation age.
// No hook calls guest code or writes to a guest task/actor.
enum { LOOT_REQUESTS=512, LOOT_DEPTH=16 };
typedef struct {
    uint64_t task,at;
    LootContext scope;
    char name[64],creator_name[64];uint32_t creator_hash;
    bool valid;
} LootRequest;
typedef struct {uint64_t actor,block;uint32_t generation;} LootJob;
typedef struct {LootRequest request;uint64_t descriptor;} LootFactory;
static LootRequest loot_requests[LOOT_REQUESTS];
static uint32_t loot_boot=1;
static _Thread_local struct {
    uint32_t boot,jobs,factories;
    LootJob job[LOOT_DEPTH];LootFactory factory[LOOT_DEPTH];
} loot_thread;
static void loot_thread_boot(void){uint32_t boot=__atomic_load_n(&loot_boot,__ATOMIC_ACQUIRE);if(loot_thread.boot!=boot){memset(&loot_thread,0,sizeof loot_thread);loot_thread.boot=boot;}}
uint64_t rt_hyrulem_loot_job_begin(uint64_t a){
    loot_thread_boot();uint32_t depth=++loot_thread.jobs;
    if(depth<=LOOT_DEPTH){LootJob *j=&loot_thread.job[depth-1];memset(j,0,sizeof *j);if(supported()&&range(a,0x7d0)){
        uint64_t block=LD64(a+0x68);uint32_t gen=LD32(a+0x60);
        if(actor_world_weak(a,block,gen))*j=(LootJob){a,block,gen};
    }}return (uint64_t)loot_thread.boot<<32|depth;
}
void rt_hyrulem_loot_job_end(uint64_t token){loot_thread_boot();if((uint32_t)(token>>32)==loot_thread.boot&&(uint32_t)token==loot_thread.jobs&&loot_thread.jobs)loot_thread.jobs--;}
void rt_hyrulem_loot_request(Cpu *c){
    if(!supported()||!range(c->x[0],0x1d8))return;
    // The copier stores request SafeString.data (+8) at task+0xc0. The
    // SafeString at +0x10 is an independent parameter, not the actor name.
    LootRequest next={.task=c->x[0],.at=now_ms()};uint64_t desc=c->x[1],text=range(desc,0x60)?LD64(desc+8):0;
    if(range(text,64)&&memchr((void *)(uintptr_t)text,0,64)&&clip_name((char *)(uintptr_t)text))snprintf(next.name,64,"%s",(char *)(uintptr_t)text);
    loot_thread_boot();
    if(next.name[0]&&loot_thread.jobs&&loot_thread.jobs<=LOOT_DEPTH){
        LootJob *j=&loot_thread.job[loot_thread.jobs-1];uint64_t a=j->actor;
        if(actor_world_weak(a,j->block,j->generation)&&birth_name(a,next.creator_name)){
            uint64_t map=LD64(a+0x7c8);if(range(map,0x70))next.creator_hash=LD32(map+0x58);
            GroundMode g;uint64_t life=enemy_life(a);
            if((ground_actor(next.name)||strstr(next.name,"_Trunk"))&&life&&!LD32(life)&&loot_scope(&g)){
                pthread_mutex_lock(&birth_mutex);LootContext *p=loot_find(a,&g);
                if(p&&p->identity.ordinal<4096&&(!strstr(next.name,"_Trunk")||(!strncmp(p->identity.name,"Obj_Tree",8)&&!strstr(p->identity.name,"_Trunk")))){next.scope=*p;
                    if(!strstr(next.name,"_Trunk"))next.scope.identity.ordinal=++p->identity.ordinal;
                    next.valid=true;p->at=next.at;
                }pthread_mutex_unlock(&birth_mutex);
            }
        }
    }
    // Every copy replaces any previous request for this address, including
    // unrelated/invalid requests. Reuse cannot inherit a dead actor's ancestry.
    pthread_mutex_lock(&birth_mutex);unsigned at=LOOT_REQUESTS,oldest=0;
    for(unsigned i=0;i<LOOT_REQUESTS;i++){if(loot_requests[i].task==next.task){at=i;break;}if(loot_requests[i].at<loot_requests[oldest].at)oldest=i;}
    if(at==LOOT_REQUESTS)at=oldest;loot_requests[at]=next;pthread_mutex_unlock(&birth_mutex);
}
uint64_t rt_hyrulem_loot_factory_begin(uint64_t task,uint64_t descriptor){
    loot_thread_boot();uint32_t depth=++loot_thread.factories;LootRequest request={0};
    pthread_mutex_lock(&birth_mutex);for(unsigned i=0;i<LOOT_REQUESTS;i++)if(loot_requests[i].task==task){
        if(now_ms()-loot_requests[i].at<15000)request=loot_requests[i];memset(&loot_requests[i],0,sizeof loot_requests[i]);break;
    }
    pthread_mutex_unlock(&birth_mutex);
    if(depth<=LOOT_DEPTH)loot_thread.factory[depth-1]=(LootFactory){request,descriptor};
    return (uint64_t)loot_thread.boot<<32|depth;
}
void rt_hyrulem_loot_factory_end(uint64_t token){loot_thread_boot();if((uint32_t)(token>>32)==loot_thread.boot&&(uint32_t)token==loot_thread.factories&&loot_thread.factories)loot_thread.factories--;}
static bool loot_factory_context(uint64_t descriptor,const char *name,LootRequest *out){
    loot_thread_boot();if(!loot_thread.factories||loot_thread.factories>LOOT_DEPTH)return false;
    LootFactory *f=&loot_thread.factory[loot_thread.factories-1];
    if(f->descriptor!=descriptor||!f->request.task||strcmp(f->request.name,name))return false;*out=f->request;return true;
}
