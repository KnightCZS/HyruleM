// Reconstruct missing canonical entities through the already verified native
// constructor. Copies never become simulation sources and carry no map pointer.
static uint64_t ground_storage(void);
static void actor_world_unbind(Cpu *c,uint64_t actor){
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].actor==actor)actor_world_release(c,&actor_world_bindings[i]);
}
static bool actor_world_remove_replica(Cpu *c,WorldReplica *r){
    if(r->visual.actor)actor_world_unbind(c,r->visual.actor);
    if(!remove_gear(c,&r->visual))return false;
    uint64_t storage=r->visual.storage;memset(r,0,sizeof *r);r->visual.storage=storage;return true;
}
static const WorldActor *actor_world_target(const ActorWorldMode *mode,uint64_t id){
    for(unsigned i=0;i<mode->count;i++)if(mode->actors[i].id==id)return &mode->actors[i];return NULL;
}
static bool actor_world_original(const WorldActor *state){
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){
        WorldBinding *b=&actor_world_bindings[i];
        if(b->actor&&!b->state.replica&&actor_world_index(b)&&actor_world_same(&b->state,state))return true;
    }return false;
}
static bool actor_world_release_terminal(Cpu *c,WorldReplica *r){
    uint64_t a=r->visual.actor;if(!a||(r->identity.kind!=1&&r->identity.kind!=3)||!r->health_ready||actor_world_replica(a)!=r)return false;
    uint64_t life=enemy_life(a);if(!life||LD32(life)!=0)return false;
    ActorRef ref=resolve_at(c,r->visual.storage,a);if(!ref.ptr)return false;
    // Keep terminal damage evidence before releasing the alias. The game owns
    // the death animation and drops; a retained replica must not stall it.
    Cpu evidence=*c;evidence.x[0]=a;rt_hyrulem_capture_zero(&evidence);
    release_at(c,r->visual.storage);actor_world_unbind(c,a);call(c,g_site.actor_index_release,r->visual.storage+INDEX,0,0,0,0,0,0);
    uint64_t st=r->visual.storage;memset(r,0,sizeof *r);r->visual.storage=st;return true;
}
static void actor_world_apply_rebuild(Cpu *c,const ActorWorldMode *mode){
    WorldRebuild request;pthread_mutex_lock(&actor_world_mutex);request=actor_world_rebuild_request;memset(&actor_world_rebuild_request,0,sizeof actor_world_rebuild_request);pthread_mutex_unlock(&actor_world_mutex);
    if(!request.id||mode->role!=3||request.epoch!=mode->epoch||strcmp(request.session,mode->session)||strcmp(request.world,mode->world)||strcmp(request.scene,mode->scene))return;
    const WorldActor *target=actor_world_target(mode,request.id);if(!target)return;
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){
        WorldBinding *b=&actor_world_bindings[i];if(!b->actor||b->state.replica||!actor_world_same(&b->state,target))continue;
        ActorRef ref=resolve_at(c,b->storage,b->actor);if(!ref.ptr)return;
        call(c,g_site.hm_delete_actor,ref.ptr,0,0,0,0,0,0);release_at(c,b->storage);actor_world_release(c,b);actor_world_scan_at=0;
        pthread_mutex_lock(&actor_world_mutex);actor_world_rebuilds++;pthread_mutex_unlock(&actor_world_mutex);return;
    }
}
static void actor_world_replica_tick(Cpu *c,const ActorWorldMode *mode,uint64_t vt,bool paused){
    uint64_t now=now_ms();
    if(paused){for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){actor_world_replicas[i].visual.started=now;actor_world_replicas[i].since=now;}return;}
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){
        WorldReplica *r=&actor_world_replicas[i];GearVisual *g=&r->visual;if(!r->identity.id)continue;
        if(actor_world_release_terminal(c,r))continue;
        bool same=mode->role==3&&!strcmp(r->session,mode->session)&&!strcmp(r->world,mode->world)&&!strcmp(r->scene,mode->scene);
        const WorldActor *target=same?actor_world_target(mode,r->identity.id):NULL;
        if(target&&!actor_world_same(target,&r->identity))target=NULL;
        if(target){r->last_target=now;r->identity=*target;}
        if(!same||(target&&actor_world_original(target))||(!target&&now-r->last_target>1500)){
            actor_world_remove_replica(c,r);continue;
        }
        if(!target)continue;
        if(g->failed){
            WorldActor identity=r->identity;char session[17],world[25],scene[65];memcpy(session,r->session,sizeof session);memcpy(world,r->world,sizeof world);memcpy(scene,r->scene,sizeof scene);
            if(actor_world_remove_replica(c,r)){r->identity=identity;memcpy(r->session,session,sizeof session);memcpy(r->world,world,sizeof world);memcpy(r->scene,scene,sizeof scene);r->since=r->last_target=now;r->retry=now+10000;pthread_mutex_lock(&actor_world_mutex);actor_world_replica_failures++;pthread_mutex_unlock(&actor_world_mutex);}
            continue;
        }
        uint64_t st=g->storage;if(!st)continue;
        if(g->spawning){
            if(call(c,g_site.hm_handle_failed,st+HANDLE,0,0,0,0,0,0)||now-g->started>15000){g->failed=true;continue;}
            if(!call(c,g_site.hm_handle_ready,st+HANDLE,0,0,0,0,0,0))continue;
            uint64_t a=call(c,g_site.hm_handle_get,st+HANDLE,0,0,0,0,0,0);
            if(!range(a,0x7d0)||!call(c,g_site.actor_index_assign,st+INDEX,a,1,0,0,0,0)){g->failed=true;continue;}
            g->actor=a;uint64_t awake=call(c,g_site.hm_handle_wake,st+HANDLE,0,0,0,0,0,0);g->spawning=false;
            if(awake!=a){g->failed=true;continue;}
        }
        if(g->actor){
            ActorRef ref=resolve_at(c,st,g->actor);
            if(ref.ptr){
                char name[64];bool ready=actor_world_ready(ref.ptr)&&actor_world_name(LD64(ref.ptr+0x10),name)&&!strcmp(name,r->identity.name);
                if(ready&&(target->kind==1||target->kind==3)&&!r->health_ready){
                    EnemyHealth identity={0};snprintf(identity.scene,sizeof identity.scene,"%s",r->scene);snprintf(identity.name,sizeof identity.name,"%s",target->name);identity.hash=target->hash;memcpy(identity.spawn,target->spawn,sizeof identity.spawn);
                    uint64_t life=enemy_life(ref.ptr);int health=-1;pthread_mutex_lock(&mutex);for(int j=0;j<health_count;j++)if(same_enemy(&identity,&health_pending[j])){health=health_pending[j].health;break;}pthread_mutex_unlock(&mutex);
                    if(life&&health>=0){ST32(life,(uint32_t)health);r->health_ready=true;}else if(!life&&target->kind==3)r->health_ready=true;else ready=false;
                }
                if(ready&&!g->active){memcpy((void *)(uintptr_t)(st+MATRIX),target->matrix,48);call(c,g_site.hm_set_mtx,ref.ptr,st+MATRIX,0,0,0,0,0);call(c,g_site.hm_set_mtx,ref.ptr,st+MATRIX,1,0,0,0,0);g->active=true;actor_world_scan_at=0;}
                release_at(c,st);
            }
            if(ref.invalid||(!g->active&&now-g->started>15000))g->failed=true;
        }
    }
    // One new request per joined-jobs callback limits constructor pressure.
    if(mode->role!=3)return;
    for(unsigned i=0;i<mode->count;i++){
        const WorldActor *target=&mode->actors[i];if(target->kind<1||target->kind>3||actor_world_original(target))continue;
        WorldReplica *r=NULL;for(unsigned j=0;j<ACTOR_WORLD_MAX;j++)if(actor_world_replicas[j].identity.id==target->id&&!strcmp(actor_world_replicas[j].world,mode->world)&&!strcmp(actor_world_replicas[j].scene,mode->scene)&&!strcmp(actor_world_replicas[j].session,mode->session)){r=&actor_world_replicas[j];break;}
        if(!r){for(unsigned j=0;j<ACTOR_WORLD_MAX;j++)if(!actor_world_replicas[j].identity.id){r=&actor_world_replicas[j];break;}if(!r)return;r->identity=*target;r->since=r->last_target=now;snprintf(r->session,sizeof r->session,"%s",mode->session);snprintf(r->world,sizeof r->world,"%s",mode->world);snprintf(r->scene,sizeof r->scene,"%s",mode->scene);}
        GearVisual *g=&r->visual;if(g->actor||g->spawning||g->failed||now-r->since<1500||now<r->retry)continue;
        if(!g->storage)g->storage=ground_storage();if(!g->storage)continue;uint64_t st=g->storage;
        uint64_t creator=LD64(g_site.hm_creator),manager=LD64(g_site.hm_heap_mgr);if(!range(creator,0x60)||!range(manager,0x30)||LD8(creator+0x59))return;uint64_t heap=LD64(manager+0x28);if(!range(heap,8))return;
        memset((void *)(uintptr_t)(st+HANDLE),0,16);memset((void *)(uintptr_t)(st+PARAMS),0,0xd8);for(unsigned j=0;j<3;j++){float f=target->matrix[j*4+3];memcpy((void *)(uintptr_t)(st+VECTOR+j*4),&f,4);}
        ST64(st+SSTR,vt);ST64(st+SSTR+8,st+KEY);call(c,g_site.hm_param_add,st+PARAMS+0x10,st+VECTOR,st+SSTR,12,4,0,0);snprintf((char *)(uintptr_t)(st+NAME),64,"%s",target->name);
        if(call(c,g_site.hm_create,creator,st+NAME,heap,st+HANDLE,st+PARAMS,0,1)){g->spawning=true;g->started=now;pthread_mutex_lock(&actor_world_mutex);actor_world_replica_spawns++;pthread_mutex_unlock(&actor_world_mutex);}else g->failed=true;
        return;
    }
}
