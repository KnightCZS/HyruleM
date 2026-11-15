// Visual ELink requests only. Never forward sound, combat events, pointers or
// Player controller calls. Native handles retain the engine's generation check.
typedef struct {uint64_t actor,block,user;uint32_t generation;} FxOwner;
typedef struct {FxEvent event;uint64_t context,at,user;uint32_t generation;} FxSource;
typedef struct {uint64_t sequence,actor;unsigned slot;bool ended,started;} FxVisual;
static FxOwner fx_owners[4];static FxSource fx_sources[FX_MAX];
static FxVisual fx_visual[FX_MAX];static uint64_t fx_sequence,fx_storage,fx_highwater;
static bool fx_key(const char *s){return s[0]&&strcmp(s,"-")&&clip_name(s);}
static uint64_t fx_user(uint64_t a){
    if(!range(a,0x570)||LD8(a+0x64)!=1)return 0;uint64_t x=LD64(a+0x568);
    if(!range(x,0x58)||LD64(x+8)!=a)return 0;uint64_t u=LD64(x+0x50);return range(u,0x110)?u:0;
}
static bool fx_owner_live(const FxOwner *o){return o->user&&range(o->actor,0x70)&&LD8(o->actor+0x64)==1&&LD64(o->actor+0x68)==o->block&&range(o->block,0x50)&&LD32(o->block+0x40)==o->generation&&LD64(o->block+0x48)==o->actor&&fx_user(o->actor)==o->user;}
static void fx_owners_update(const uint64_t actors[4]){
    FxOwner next[4]={0};for(unsigned i=0;i<4;i++)if(range(actors[i],0x70)){uint64_t block=LD64(actors[i]+0x68);if(range(block,0x50))next[i]=(FxOwner){actors[i],block,fx_user(actors[i]),LD32(block+0x40)};}
    pthread_mutex_lock(&mutex);memcpy(fx_owners,next,sizeof next);published.fx_source_users=0;for(unsigned i=0;i<4;i++)if(fx_owner_live(&next[i]))published.fx_source_users|=1u<<i;
    for(unsigned i=0;i<FX_MAX;i++){FxSource *s=&fx_sources[i];if(!s->event.sequence||!s->event.active)continue;if(next[s->event.slot].user!=s->user||!fx_owner_live(&next[s->event.slot])||!range(s->context,0x24)||LD32(s->context+0x20)!=s->generation||(LD32(s->context+8)&0x10))s->event.active=false;}
    pthread_mutex_unlock(&mutex);
}
// Hook after the native name lookup/emit returned: x19 is the output handle,
// x20 the UserInstance and x21 the key C string. Failed requests are ignored.
void rt_hyrulem_capture_effect(Cpu *c){
    if(!supported())return;uint64_t handle=c->x[19],user=c->x[20],key=c->x[21];
    if(!range(handle,12)||!range(key,64)||!memchr((void *)(uintptr_t)key,0,64))return;
    char name[64];memcpy(name,(void *)(uintptr_t)key,64);if(!fx_key(name))return;
    uint64_t context=LD64(handle);if(!range(context,0x24))return;uint32_t generation=LD32(handle+8);if(LD32(context+0x20)!=generation)return;
    pthread_mutex_lock(&mutex);int slot=-1;
    for(unsigned i=0;i<4;i++)if(fx_owners[i].user==user&&fx_owner_live(&fx_owners[i])){slot=(int)i;break;}
    if(slot<0){pthread_mutex_unlock(&mutex);return;}
    int at=-1;for(unsigned i=0;i<FX_MAX;i++){FxSource *s=&fx_sources[i];if(s->event.sequence&&s->context==context&&s->generation==generation){pthread_mutex_unlock(&mutex);return;}if(!s->event.active&&(at<0||s->at<fx_sources[at].at))at=(int)i;}
    if(at>=0){FxSource *s=&fx_sources[at];memset(s,0,sizeof *s);s->event.sequence=++fx_sequence;s->event.slot=(unsigned)slot;s->event.active=true;snprintf(s->event.name,64,"%s",name);s->context=context;s->user=user;s->generation=generation;s->at=now_ms();published.fx_captured++;snprintf(published.fx_last,64,"%s",name);}else published.fx_overflow++;
    pthread_mutex_unlock(&mutex);
}
void rt_hyrulem_stop_effect(Cpu *c){
    if(!supported()||!range(c->x[0],0x24))return;uint64_t context=c->x[0];uint32_t generation=LD32(context+0x20);
    pthread_mutex_lock(&mutex);for(unsigned i=0;i<FX_MAX;i++){FxSource *s=&fx_sources[i];if(s->event.sequence&&s->context==context&&s->generation==generation)s->event.active=false;}pthread_mutex_unlock(&mutex);
}
static void fx_json(char *out,size_t size){
    size_t used=1;out[0]='[';uint64_t now=now_ms();bool any=false;
    pthread_mutex_lock(&mutex);for(unsigned i=0;i<FX_MAX;i++){FxSource *s=&fx_sources[i];if(!s->event.sequence||(!s->event.active&&now-s->at>500))continue;uint64_t age=now-s->at;if(age>5000)age=5000;
        int n=snprintf(out+used,size-used,"%s{\"sequence\":%llu,\"slot\":%u,\"active\":%s,\"age_ms\":%u,\"name\":\"%s\"}",any?",":"",(unsigned long long)s->event.sequence,s->event.slot,s->event.active?"true":"false",(unsigned)age,s->event.name);
        if(n<0||(size_t)n>=size-used-2)break;used+=(size_t)n;any=true;
    }pthread_mutex_unlock(&mutex);out[used++]=']';out[used]=0;
}
static bool fx_decode(int argc,char **argv,FxFrame *out){
    uint64_t count;if(!strcmp(argv[95],"0"))count=0;else if(!integer(argv[95],&count))return false;
    if(count>FX_MAX||argc!=96+(int)count*5)return false;out->count=(unsigned)count;
    for(unsigned i=0;i<count;i++){int at=96+(int)i*5;FxEvent *e=&out->events[i];uint64_t slot,age;
        if(!integer(argv[at],&e->sequence))return false;
        if(!strcmp(argv[at+1],"0"))slot=0;else if(!integer(argv[at+1],&slot))return false;
        if(!strcmp(argv[at+3],"0"))age=0;else if(!integer(argv[at+3],&age))return false;
        if(slot>3||age>5000||(strcmp(argv[at+2],"0")&&strcmp(argv[at+2],"1"))||!fx_key(argv[at+4]))return false;
        for(unsigned j=0;j<i;j++)if(out->events[j].sequence==e->sequence)return false;
        e->slot=(unsigned)slot;e->age=(uint32_t)age;e->active=!strcmp(argv[at+2],"1");snprintf(e->name,64,"%s",argv[at+4]);
    }return true;
}
static void fx_end(Cpu *c,unsigned i){
    FxVisual *v=&fx_visual[i];if(v->started&&!v->ended&&range(fx_storage,FX_MAX*16)&&g_site.hm_fx_stop){uint64_t h=fx_storage+i*16,context=LD64(h);if(range(context,0x60)&&LD32(context+0x20)==LD32(h+8)){call(c,g_site.hm_fx_stop,context,0,0,0,0,0,0);pthread_mutex_lock(&mutex);published.fx_stopped++;pthread_mutex_unlock(&mutex);}}v->ended=true;
}
static void fx_clear(Cpu *c){for(unsigned i=0;i<FX_MAX;i++)fx_end(c,i);memset(fx_visual,0,sizeof fx_visual);fx_highwater=0;}
static void fx_tick(Cpu *c,const Desired *d,uint64_t vt){
    if(!g_site.hm_fx_emit||!g_site.hm_fx_stop)return;
    if(!fx_storage&&!d->effects.count)return;
    if(!fx_storage){fx_storage=(uint64_t)(uintptr_t)hle_alloc(FX_MAX*16);if(!range(fx_storage,FX_MAX*16))return;}
    for(unsigned i=0;i<FX_MAX;i++){FxVisual *v=&fx_visual[i];if(!v->sequence)continue;bool found=false;for(unsigned j=0;j<d->effects.count;j++)if(d->effects.events[j].sequence==v->sequence&&d->effects.events[j].active){found=true;break;}if(!found)fx_end(c,i);}
    FxFrame ordered=d->effects;for(unsigned j=1;j<ordered.count;j++){FxEvent e=ordered.events[j];unsigned at=j;while(at&&ordered.events[at-1].sequence>e.sequence){ordered.events[at]=ordered.events[at-1];at--;}ordered.events[at]=e;}
    for(unsigned j=0;j<ordered.count;j++){
        const FxEvent *e=&ordered.events[j];unsigned i;for(i=0;i<FX_MAX;i++)if(fx_visual[i].sequence==e->sequence)break;
        if(i==FX_MAX){if(e->sequence<=fx_highwater)continue;for(i=0;i<FX_MAX;i++)if(!fx_visual[i].sequence)break;if(i==FX_MAX){if(e->sequence<=fx_highwater)continue;for(i=0;i<FX_MAX;i++)if(fx_visual[i].ended)break;}if(i==FX_MAX)continue;fx_highwater=e->sequence;fx_visual[i]=(FxVisual){.sequence=e->sequence,.slot=e->slot};}
        FxVisual *v=&fx_visual[i];if(v->ended||v->started)continue;
        if(!e->active&&e->age>200){v->ended=true;continue;}
        uint64_t st=e->slot?gear_visual[e->slot==1?0:e->slot==2?1:2].storage:storage;
        uint64_t owner=e->slot?gear_visual[e->slot==1?0:e->slot==2?1:2].actor:actor;
        if(!st||!owner)continue;ActorRef ref=resolve_at(c,st,owner);if(!ref.ptr||ref.state!=1){if(ref.ptr)release_at(c,st);continue;}
        uint64_t user=fx_user(ref.ptr);if(!user){release_at(c,st);pthread_mutex_lock(&mutex);published.fx_missing++;pthread_mutex_unlock(&mutex);v->ended=true;continue;}
        snprintf((char *)(uintptr_t)(st+ASNAME),64,"%s",e->name);memset((void *)(uintptr_t)(fx_storage+i*16),0,16);
        call(c,g_site.hm_fx_emit,user,st+ASNAME,fx_storage+i*16,0,0,0,0);release_at(c,st);
        v->started=true;v->actor=owner;pthread_mutex_lock(&mutex);if(LD64(fx_storage+i*16))published.fx_applied++;else published.fx_missing++;pthread_mutex_unlock(&mutex);
        // A short burst can start and finish between snapshots. Let its
        // native request reach one game update before the next tick fades it.
    }
}
static void fx_reset(void){memset(fx_owners,0,sizeof fx_owners);memset(fx_sources,0,sizeof fx_sources);memset(fx_visual,0,sizeof fx_visual);fx_sequence=fx_storage=fx_highwater=0;}
