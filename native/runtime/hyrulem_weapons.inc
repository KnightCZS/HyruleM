enum{NATIVE_DROPS=16};
typedef struct {uint64_t actor,block,id;uint32_t generation;GroundSource source;bool active,exported,local_only,pickup_pending,broken,recovery_failed;unsigned recovery_attempts;GearVisual fallback;} NativeDrop;
static uint64_t ground_destroyed[GROUND_MAX];static unsigned ground_destroyed_count;
static NativeDrop native_drops[NATIVE_DROPS];static uint64_t native_drop_sequence,native_drop_vt;
static bool ground_weapon(const char *name){return strcmp(name,"Weapon_Sword_070")&&strcmp(name,"Weapon_Sword_502")&&(gear_name(name,0)||gear_name(name,1)||gear_name(name,2));}
static bool native_drop_live(const NativeDrop *d,uint64_t a){return d->active&&d->actor==a&&range(d->block,0x50)&&LD32(d->block+0x40)==d->generation&&LD64(d->block+0x48)==a;}
static bool weapon_data(uint64_t a,GroundSource *s){
    if(!range(a,0xfa0))return false;uint64_t vt=LD64(a),life=enemy_life(a);
    if(!range(vt,0x590)||LD64(vt+0x588)!=g_site.hm_weapon_drop||!life)return false;
    s->value=(int32_t)LD32(life);s->modifier=LD32(a+0xf98);s->bonus=(int32_t)LD32(a+0xf9c);
    return s->value>0&&s->value<=1000000&&s->bonus>=-100000&&s->bonus<=100000;
}
// Entry hooks run inside game jobs. Copy weak identity and original item data;
// the post-job scan will acquire the native reference. Inventory was already
// debited by the real game: this path never debits it a second time.
void rt_hyrulem_capture_drop(Cpu *c){
    if(!ground_supported()||!range(g_site.player_info,8))return;uint64_t a=c->x[0],info=LD64(g_site.player_info);GroundSource source={0};
    if(!range(info,0x68)||!range(a,0xfa0)||!ground_read_name(LD64(a+0x10),source.name)||!ground_weapon(source.name)||!weapon_data(a,&source))return;
    uint64_t owner=LD64(a+0x938);if(!range(owner,0x50)||LD32(owner+0x40)!=LD32(a+0x940)||LD64(owner+0x48)!=LD64(info+0x60))return;
    uint64_t block=LD64(a+0x68),model=LD64(a+0x4e0);uint32_t generation=LD32(a+0x60);if(!range(block,0x50)||generation==UINT32_MAX||LD32(block+0x40)!=generation||LD64(block+0x48)!=a||!range(model,0x88))return;
    for(int i=0;i<3;i++)source.spawn[i]=source.pos[i]=read_float(model+0x64+i*16);
    pthread_mutex_lock(&ground_mutex);
    if(!ground_origin[0]||!ground_mode.heartbeat||now_ms()-ground_mode.heartbeat>=2500||!ground_in_zone(source.pos,&ground_mode,90)){pthread_mutex_unlock(&ground_mutex);return;}
    int at=-1;for(int i=0;i<NATIVE_DROPS;i++){if(native_drop_live(&native_drops[i],a)){pthread_mutex_unlock(&ground_mutex);return;}if(!native_drops[i].active&&at<0)at=i;}
    if(at>=0){NativeDrop *d=&native_drops[at];memset(d,0,sizeof *d);d->active=true;d->actor=a;d->block=block;d->generation=generation;source.drop=source.native_drop=true;source.local=UINT64_C(0x200000000)+ ++native_drop_sequence;snprintf(source.origin,17,"%s",ground_origin);d->source=source;}
    pthread_mutex_unlock(&ground_mutex);
}
static void ground_destroy_queue(uint64_t id){if(!id)return;for(unsigned i=0;i<ground_destroyed_count;i++)if(ground_destroyed[i]==id)return;if(ground_destroyed_count<GROUND_MAX)ground_destroyed[ground_destroyed_count++]=id;}
static void ground_capture_destroy(Cpu *c){
    uint64_t a=c->x[0];if(!ground_supported()||!range(a,0xfa0))return;uint64_t life=enemy_life(a);char name[48];if(!life||LD32(life)||!ground_read_name(LD64(a+0x10),name)||!ground_weapon(name))return;
    pthread_mutex_lock(&ground_mutex);
    for(int i=0;i<NATIVE_DROPS;i++)if(native_drop_live(&native_drops[i],a)){NativeDrop *d=&native_drops[i];d->broken=true;if(!d->exported)d->active=false;else ground_destroy_queue(d->id);}
    for(int i=0;i<ground_hook_count;i++)if(ground_hooks[i].actor==a&&ground_hooks[i].id&&!ground_hooks[i].replica&&ground_index_live(ground_hooks[i].storage,a)&&ground_state[ground_hooks[i].id-1].own)ground_destroy_queue(ground_hooks[i].id);
    pthread_mutex_unlock(&ground_mutex);
}
static bool ground_native_source(uint64_t a,GroundSource *source){
    bool found=false;pthread_mutex_lock(&ground_mutex);
    for(int i=0;i<NATIVE_DROPS;i++){NativeDrop *d=&native_drops[i];if(native_drop_live(d,a)&&!d->local_only){float pos[3];memcpy(pos,source->pos,sizeof pos);int value=source->value;*source=d->source;memcpy(source->pos,pos,sizeof pos);source->value=value;d->source=*source;found=true;break;}}
    pthread_mutex_unlock(&ground_mutex);return found;
}
static bool ground_native_hook(uint64_t a,GroundHook *out,bool *vanilla){
    for(int i=0;i<NATIVE_DROPS;i++){
        NativeDrop *d=&native_drops[i];if(!d->active)continue;
        if(native_drop_live(d,a)){
            if(d->local_only){d->active=false;*vanilla=true;return true;}
            if(!d->exported&&!d->id){d->active=false;*vanilla=true;return true;}
            d->pickup_pending=true;*out=(GroundHook){a,d->id,0,d->source,true,0};return true;
        }
        if(d->fallback.storage&&(d->fallback.actor==a||ground_pending_actor(d->fallback.storage)==a)){*out=(GroundHook){a,0,0,d->source,true,1};return true;}
    }
    return false;
}
static bool weapon_initialize(uint64_t a,const GroundSource *s){
    GroundSource check={0};if(!weapon_data(a,&check))return false;uint64_t life=enemy_life(a);ST32(life,(uint32_t)s->value);ST32(a+0xf98,s->modifier);ST32(a+0xf9c,(uint32_t)s->bonus);return true;
}
// A capacity rejection hands the original item back to ordinary local play.
// If its pickup had already been deferred, recreate one exact physical item;
// never retry an inventory grant as a recovery mechanism.
static void native_fallback_tick(Cpu *c,NativeDrop *d,uint64_t vt){
    GearVisual *g=&d->fallback;
    if(g->failed){if(!remove_gear(c,g)||d->recovery_attempts>=3||now_ms()-g->started<1000)return;g->failed=false;}
    if(!g->storage){g->storage=ground_storage();if(!g->storage)return;}
    if(g->spawning){
        if(call(c,g_site.hm_handle_failed,g->storage+HANDLE,0,0,0,0,0,0)||now_ms()-g->started>15000){g->failed=true;g->started=now_ms();return;}
        if(!call(c,g_site.hm_handle_ready,g->storage+HANDLE,0,0,0,0,0,0))return;
        uint64_t a=call(c,g_site.hm_handle_get,g->storage+HANDLE,0,0,0,0,0,0);if(!range(a,0xfa0)||!call(c,g_site.actor_index_assign,g->storage+INDEX,a,1,0,0,0,0))return;
        g->actor=a;call(c,g_site.hm_handle_wake,g->storage+HANDLE,0,0,0,0,0,0);g->spawning=false;
    }
    if(!g->actor){
        uint64_t creator=LD64(g_site.hm_creator),manager=LD64(g_site.hm_heap_mgr);if(!range(creator,0x60)||!range(manager,0x30)||LD8(creator+0x59))return;uint64_t heap=LD64(manager+0x28);if(!range(heap,8))return;
        uint64_t st=g->storage;memset((void *)(uintptr_t)(st+HANDLE),0,16);memset((void *)(uintptr_t)(st+PARAMS),0,0xd8);memcpy((void *)(uintptr_t)(st+VECTOR),d->source.pos,12);ST64(st+SSTR,vt);ST64(st+SSTR+8,st+KEY);snprintf((char *)(uintptr_t)(st+NAME),64,"%s",d->source.name);
        call(c,g_site.hm_param_add,st+PARAMS+0x10,st+VECTOR,st+SSTR,12,4,0,0);d->recovery_attempts++;g->started=now_ms();if(call(c,g_site.hm_create,creator,st+NAME,heap,st+HANDLE,st+PARAMS,0,1))g->spawning=true;else g->failed=true;return;
    }
    ActorRef ref=resolve_at(c,g->storage,g->actor);if(!ref.ptr){if(ref.invalid){call(c,g_site.actor_index_release,g->storage+INDEX,0,0,0,0,0,0);g->actor=0;}return;}
    bool ready=ref.state==1&&weapon_initialize(ref.ptr,&d->source);if(!ready&&now_ms()-g->started>15000){g->failed=true;g->started=now_ms();}release_at(c,g->storage);
    if(ready){call(c,g_site.actor_index_release,g->storage+INDEX,0,0,0,0,0,0);g->actor=0;pthread_mutex_lock(&ground_mutex);d->active=false;pthread_mutex_unlock(&ground_mutex);}
}
static void ground_native_tick(Cpu *c,uint64_t vt,const GroundState states[GROUND_MAX]){
    native_drop_vt=vt;
    for(int i=0;i<NATIVE_DROPS;i++){
        NativeDrop *d=&native_drops[i];NativeDrop copy;pthread_mutex_lock(&ground_mutex);copy=*d;
        if(copy.active&&!copy.local_only)for(int j=0;j<GROUND_MAX;j++)if(states[j].id&&states[j].own&&ground_source_equal(&states[j].source,&copy.source)){d->id=copy.id=states[j].id;if(d->broken)ground_destroy_queue(d->id);if(states[j].state==2||states[j].state==5)d->active=copy.active=false;break;}
        pthread_mutex_unlock(&ground_mutex);if(!copy.active)continue;
        if(copy.local_only&&copy.pickup_pending&&!copy.broken&&!native_drop_live(&copy,copy.actor))native_fallback_tick(c,d,vt);
        pthread_mutex_lock(&ground_mutex);d->recovery_failed=d->fallback.failed;pthread_mutex_unlock(&ground_mutex);
    }
}
static bool ground_native_cleanup(Cpu *c){
    bool done=true;
    for(int i=0;i<NATIVE_DROPS;i++){
        NativeDrop *d=&native_drops[i];NativeDrop copy;pthread_mutex_lock(&ground_mutex);copy=*d;pthread_mutex_unlock(&ground_mutex);if(!copy.active)continue;
        bool live=native_drop_live(&copy,copy.actor);
        if(copy.local_only&&copy.pickup_pending&&!copy.broken&&!live){native_fallback_tick(c,d,native_drop_vt);pthread_mutex_lock(&ground_mutex);d->recovery_failed=d->fallback.failed;pthread_mutex_unlock(&ground_mutex);if(d->active)done=false;continue;}
        if(!copy.local_only&&copy.exported&&live){
            GearVisual *g=&d->fallback;if(!g->storage)g->storage=ground_storage();if(!g->storage){done=false;continue;}
            if(!g->actor){if(!call(c,g_site.actor_index_assign,g->storage+INDEX,copy.actor,1,0,0,0,0)){done=false;continue;}g->actor=copy.actor;}
            if(!remove_gear(c,g)){done=false;continue;}
        }
        pthread_mutex_lock(&ground_mutex);d->active=false;pthread_mutex_unlock(&ground_mutex);
    }return done;
}
static void ground_native_local(int argc,char **argv,ControlReply *r){
    uint64_t local;if(argc!=4||!token(argv[1])||!ground_world_token(argv[2])||!integer(argv[3],&local)||local<UINT64_C(0x200000000)){control_fail(r,"Invalid native drop acknowledgement");return;}
    pthread_mutex_lock(&ground_mutex);bool valid=!strcmp(ground_mode.session,argv[1])&&!strcmp(ground_mode.world,argv[2]);if(valid)for(int i=0;i<NATIVE_DROPS;i++)if(native_drops[i].source.local==local)native_drops[i].local_only=true;pthread_mutex_unlock(&ground_mutex);if(!valid){control_fail(r,"Wrong native drop session");return;}control_field_bool(r,"queued",true);
}
static int weapon_inventory(Cpu *c,uint64_t mgr,const char *name,uint64_t ids[420],uint64_t *created,const uint64_t *before,int nbefore,const GroundSource *expected){
    int tabs=(int32_t)LD32(mgr+0x44498),total=0;if(tabs<1||tabs>50||LD32(mgr+0x78)+LD32(mgr+0x90)!=420)return -1;
    for(int tab=0;tab<tabs;tab++){int count=(int32_t)call(c,g_site.pouch_page_count,mgr,tab,0,0,0,0,0);if(count<0||count>20)return -1;
        for(int slot=0;slot<count;slot++){uint64_t p=call(c,g_site.pouch_page_item,mgr,tab,slot,0,0,0,0);if(!range(p,0x38))return -1;uint64_t text=LD64(p+0x30);if(!range(text,48)||!memchr((void *)(uintptr_t)text,0,48))return -1;if(strcmp((const char *)(uintptr_t)text,name))continue;if(!LD8(p+0x25)||(LD32(p+0x18)!=0&&LD32(p+0x18)!=1&&LD32(p+0x18)!=3))return -1;
            bool duplicate=false;for(int j=0;j<total;j++)if(ids[j]==p)duplicate=true;if(duplicate)continue;if(total>=420)return -1;ids[total++]=p;
            if(created){bool old=false;for(int j=0;j<nbefore;j++)if(before[j]==p)old=true;if(!old){if(*created||(int32_t)LD32(p+0x20)!=expected->value)return -1;*created=p;}}
        }
    }return total;
}
static int ground_weapon_grant(Cpu *c,const GroundState *s,uint64_t vt,uint64_t st){
    uint64_t mgr=LD64(g_site.hm_pouch_mgr);if(!range(mgr,0x4449c)||!vt||!st)return -1;uint64_t before[420],after[420],created=0;int result=-1;
    call(c,g_site.pouch_lock,mgr+0x28,0,0,0,0,0,0);int n=weapon_inventory(c,mgr,s->source.name,before,NULL,NULL,0,NULL);
    if(n>=0){snprintf((char *)(uintptr_t)(st+NAME),64,"%s",s->source.name);ST64(st+SSTR,vt);ST64(st+SSTR+8,st+NAME);ST32(st+PARAMS,s->source.modifier);ST32(st+PARAMS+4,(uint32_t)s->source.bonus);
        call(c,g_site.hm_item_get,mgr,st+SSTR,s->source.value,st+PARAMS,0,0,0);int m=weapon_inventory(c,mgr,s->source.name,after,&created,before,n,&s->source);result=m==n+1&&created?1:m==n&&!created?-1:0;}
    call(c,g_site.pouch_unlock,mgr+0x28,0,0,0,0,0,0);return result;
}
