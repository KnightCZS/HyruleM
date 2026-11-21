// Diagnostic capture at the native factory's successful initialization path.
// This runs in a worker: copy bounded metadata only, never call guest code.
enum { BIRTH_MAX=64 };
typedef struct {
    uint64_t sequence,actor,block,parent,parent_block;
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
void rt_hyrulem_capture_birth(Cpu *c){
    const char *trace=getenv("HYRULEM_NATIVE_BIRTH_TRACE");if(!trace||strcmp(trace,"1")||!supported())return;
    uint64_t task=c->x[21],a=c->x[22];BirthEvent e={0};
    if(!range(task,0x50)||!birth_name(a,e.name)||(!ground_actor(e.name)&&!strstr(e.name,"_Trunk")))return;
    e.actor=a;e.block=LD64(a+0x68);e.generation=LD32(a+0x60);
    if(!actor_world_weak(a,e.block,e.generation))return;
    e.parent=LD64(task+0x40);if(birth_name(e.parent,e.parent_name)){
        e.parent_block=LD64(e.parent+0x68);e.parent_generation=LD32(e.parent+0x60);
        uint64_t map=LD64(e.parent+0x7c8);if(range(map,0x70))e.parent_hash=LD32(map+0x58);
    }
    uint64_t map=LD64(a+0x7c8);if(range(map,0x70))e.hash=LD32(map+0x58);
    uint64_t params=LD64(task+0x30);if(range(params,2))e.parameters=LD16(params);
    for(unsigned i=0;i<3;i++){e.position[i]=read_float(a+0x3a4+i*16);if(!isfinite(e.position[i])||fabsf(e.position[i])>100000)return;}
    pthread_mutex_lock(&birth_mutex);e.sequence=++birth_sequence;birth_events[(e.sequence-1)%BIRTH_MAX]=e;pthread_mutex_unlock(&birth_mutex);
}
static void birth_reset(void){pthread_mutex_lock(&birth_mutex);memset(birth_events,0,sizeof birth_events);birth_sequence=0;pthread_mutex_unlock(&birth_mutex);}
static void birth_status(int argc,char **argv,ControlReply *r){
    BirthEvent rows[BIRTH_MAX];uint64_t seq;pthread_mutex_lock(&birth_mutex);memcpy(rows,birth_events,sizeof rows);seq=birth_sequence;pthread_mutex_unlock(&birth_mutex);
    char json[BIRTH_MAX*320+3];size_t used=1;json[0]='[';unsigned count=seq<BIRTH_MAX?(unsigned)seq:BIRTH_MAX;
    for(unsigned i=0;i<count;i++){
        BirthEvent *e=&rows[(seq-count+i)%BIRTH_MAX];
        int n=snprintf(json+used,sizeof json-used,"%s{\"sequence\":%llu,\"actor\":\"%s\",\"hash\":%u,\"parent\":\"%s\",\"parent_hash\":%u,\"parameters\":%u,\"position\":[%.3f,%.3f,%.3f]}",i?",":"",(unsigned long long)e->sequence,e->name,e->hash,e->parent_name,e->parent_hash,e->parameters,e->position[0],e->position[1],e->position[2]);
        if(n<0||(size_t)n>=sizeof json-used-2){control_fail(r,"Birth diagnostic overflow");return;}used+=(size_t)n;
    }json[used++]=']';json[used]=0;control_field_int(r,"sequence",seq);control_field_str(r,"births_json",json);
}
