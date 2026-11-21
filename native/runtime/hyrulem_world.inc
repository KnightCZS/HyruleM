// Map-entity damage ledger. A missing actor is never a destruction event.
// Keep cumulative damage for changed actors, reclaiming undamaged baselines
// as the map streams. The transient zero queue remains bounded independently.
enum { WORLD_HISTORY_MAX=8192 };
static EnemyHealth world_history[WORLD_HISTORY_MAX],world_zero[HEALTH_MAX];
static int world_history_count,world_zero_count;static bool world_history_reset;
static bool map_damage_name(const char *s){
    return !strncmp(s,"Enemy_",6)||(!strncmp(s,"Obj_Tree",8)&&!strstr(s,"_Stump")&&!strstr(s,"_Trunk")&&!strstr(s,"Root")&&!strstr(s,"Branch"));
}
static EnemyHealth *world_note(EnemyHealth *e){
    int at=0;while(at<world_history_count&&!same_enemy(e,&world_history[at]))at++;
    if(at==world_history_count){
        if(at<WORLD_HISTORY_MAX)world_history_count++;
        else {
            // Preserve counters for every damaged identity when recycling.
            for(at=0;at<world_history_count;at++)if(!world_history[at].damage)break;
            if(at==world_history_count)return NULL;
        }
        world_history[at]=*e;
    }
    EnemyHealth *old=&world_history[at];if(e->health<old->health){uint64_t delta=(uint64_t)(old->health-e->health);if(old->damage<=INT64_MAX-delta)old->damage+=delta;}
    e->damage=old->damage;return old;
}
// Called before the game's deletion method can erase the evidence. Only copy
// a confirmed native life==0 and map identity; do not call game code here.
void rt_hyrulem_capture_zero(Cpu *c){
    ground_capture_destroy(c);
    if(!world_supported())return;uint64_t a=c->x[0];if(!range(a,0x7d0))return;EnemyHealth e={0};snprintf(e.scene,sizeof e.scene,"%s",local_scene);
    if(!enemy_name(LD64(a+0x10),e.name))return;uint64_t life=enemy_life(a),map=LD64(a+0x7c8);if(!life||LD32(life)||!range(map,0x70))return;
    e.hash=LD32(map+0x58);if(!e.hash||e.hash==UINT32_MAX)return;
    for(int i=0;i<3;i++){e.spawn[i]=read_float(map+0x5c+i*4);e.position[i]=read_float(a+0x3a4+i*16);if(!isfinite(e.spawn[i])||!isfinite(e.position[i])||fabsf(e.spawn[i])>100000||fabsf(e.position[i])>100000)return;}
    pthread_mutex_lock(&mutex);
    if(desired.session[0]&&desired.received&&now_ms()-desired.received<2500){int at=0;while(at<world_zero_count&&!same_enemy(&e,&world_zero[at]))at++;if(at<HEALTH_MAX){world_zero[at]=e;if(at==world_zero_count)world_zero_count++;}}
    pthread_mutex_unlock(&mutex);
}

// Loaded-world authority. Only known map actors are matched; streaming absence
// never means death and no unknown native NPC is created or deleted here.
typedef struct {uint64_t actor,block,at;uint32_t generation;Track tracks[2];} WorldAS;
typedef struct {uint64_t storage,actor,id,seen,started,duration,applied_at;WorldActor state;float from[12],target[12];Track played[2];} WorldBinding;
static void actor_world_rebase(Cpu *,uint64_t,uint64_t,const float [12]);
static pthread_mutex_t actor_world_mutex=PTHREAD_MUTEX_INITIALIZER;
static ActorWorldMode actor_world_mode,actor_world_live;
static WorldBinding actor_world_bindings[ACTOR_WORLD_MAX];
static WorldAS actor_world_as[128];
static uint64_t actor_world_capture_seq,actor_world_frame_seq,actor_world_id,actor_world_scan_at,actor_world_applied_seq,actor_world_arrival;
static WorldActor actor_world_seen[ACTOR_WORLD_MAX];static unsigned actor_world_seen_count;
static unsigned actor_world_matched,actor_world_missing,actor_world_applied,actor_world_ai_skipped,actor_world_as_applied;
static bool actor_world_complete;static float actor_world_center[3];
static uint32_t actor_world_kind(const char *s){
    if(!strncmp(s,"Enemy_",6)){
        const char *exclude[]={"Ganon","SiteBoss","Giant","Dragon","Assassin_Senior","Arrow","Bullet","Beam","Projectile","Tornado"};
        for(unsigned i=0;i<sizeof exclude/sizeof *exclude;i++)if(strstr(s,exclude[i]))return 0;return 1;
    }
    if(!strncmp(s,"Npc_",4))return 2;
    const char *props[]={"Obj_Barrel","Obj_Box","Obj_LiftRock","Obj_Rock","FldObj_KorokStone","FldObj_KorokBlock","DgnObj_Small_Box","DgnObj_IronBall","Kibako","Barrel","DgnObj_StoneBarrel","Test_MetalBarrel","FldObj_SnowBallDoor_A_02"};
    for(unsigned i=0;i<sizeof props/sizeof *props;i++)if(!strncmp(s,props[i],strlen(props[i])))return 3;
    if(strcmp(s,"Weapon_Sword_070")&&strcmp(s,"Weapon_Sword_502")&&(!strncmp(s,"Weapon_Sword_",13)||!strncmp(s,"Weapon_Lsword_",14)||!strncmp(s,"Weapon_Spear_",13)||!strncmp(s,"Weapon_Bow_",11)||!strncmp(s,"Weapon_Shield_",14)))return 4;
    return 0;
}
static bool actor_world_name(uint64_t p,char name[64]){
    if(!range(p,64)||!memchr((void *)(uintptr_t)p,0,64))return false;memset(name,0,64);size_t n=strnlen((char *)(uintptr_t)p,64);if(!n||n==64)return false;
    for(size_t i=0;i<n;i++){char c=(char)LD8(p+i);if(!((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='_'))return false;name[i]=c;}return actor_world_kind(name)!=0;
}
static bool actor_world_matrix(const float m[12]){
    for(unsigned i=0;i<12;i++)if(!isfinite(m[i])||fabsf(m[i])>(i%4==3?100000.f:8.f))return false;
    if(fabsf(m[7])>50000)return false;for(unsigned r=0;r<3;r++){float n=0;for(unsigned j=0;j<3;j++)n+=m[r*4+j]*m[r*4+j];if(n<.0001f)return false;}return true;
}
static bool actor_world_same(const WorldActor *a,const WorldActor *b){
    if(a->hash!=b->hash||a->kind!=b->kind||strcmp(a->name,b->name))return false;
    for(unsigned i=0;i<3;i++)if(fabsf(a->spawn[i]-b->spawn[i])>.003f)return false;return true;
}
static bool actor_world_equipped(uint64_t a,unsigned kind){
    if(kind!=4)return false;if(!range(a,0x948))return true;
    // Weapon's retained owner link. Player-authored drops are handled by the
    // existing ground transaction layer; they must not become a second copy.
    uint64_t b=LD64(a+0x938);uint32_t g=LD32(a+0x940);return range(b,0x50)&&g!=UINT32_MAX&&LD32(b+0x40)==g&&LD64(b+0x48)!=0;
}
static bool actor_world_ready(uint64_t a){
    // Native map actors can be initialized but asleep (2) when culled; Link
    // proxies use Calc (1). Init (0) and deletion (3) are never writable.
    return range(a,0x7d0)&&(LD8(a+0x64)==1||LD8(a+0x64)==2)&&range(LD64(a+0x4e0),0xa4);
}
static bool actor_world_read(uint64_t a,WorldActor *out){
    if(!actor_world_ready(a)||!actor_world_name(LD64(a+0x10),out->name))return false;
    out->kind=actor_world_kind(out->name);if(actor_world_equipped(a,out->kind))return false;
    uint64_t map=LD64(a+0x7c8);if(!range(map,0x70))return false;out->hash=LD32(map+0x58);if(!out->hash||out->hash==UINT32_MAX)return false;
    for(unsigned i=0;i<3;i++){out->spawn[i]=read_float(map+0x5c+i*4);if(!isfinite(out->spawn[i])||fabsf(out->spawn[i])>(i==1?50000.f:100000.f))return false;}
    for(unsigned i=0;i<12;i++)out->matrix[i]=read_float(a+0x398+i*4);return actor_world_matrix(out->matrix);
}
static bool actor_world_weak(uint64_t a,uint64_t block,uint32_t generation){return range(a,0x70)&&range(block,0x50)&&generation!=UINT32_MAX&&LD32(block+0x40)==generation&&LD64(block+0x48)==a&&LD8(a+0x64)!=3;}
static bool actor_world_index(const WorldBinding *b){return b->storage&&LD8(b->storage+INDEX+12)&&actor_world_weak(b->actor,LD64(b->storage+INDEX),LD32(b->storage+INDEX+8));}
static void actor_world_capture_as(uint64_t as,uint64_t request){
    // The native request clamps out-of-range/default indices to group/slot 0.
    // NPCs need that normalization before the bounded two-track wire format.
    unsigned groups=LD32(as+0xb8),group=LD32(request+0x18);uint64_t list=LD64(as+0xc0);
    if(!groups||groups>8||!range(list,groups*0x50))return;if(group>=groups)group=0;if(group>1)return;
    unsigned slots=LD32(list+group*0x50+0x20),slot=LD32(request+0x1c);uint64_t controllers=LD64(list+group*0x50+0x28);
    if(!slots||slots>8||!range(controllers,slots*0x98))return;if(slot>=slots)slot=0;if(slot>1)return;
    uint64_t a=LD64(as+0xd8),p=LD64(request+0x10);char name[64];if(!range(a,0x7d0)||!actor_world_name(LD64(a+0x10),name)||!range(p,64)||!memchr((void *)(uintptr_t)p,0,64)||!clip_name((char *)(uintptr_t)p))return;
    uint64_t block=LD64(a+0x68);if(!range(block,0x50))return;uint32_t generation=LD32(block+0x40);if(!actor_world_weak(a,block,generation))return;
    pthread_mutex_lock(&actor_world_mutex);unsigned at=128,oldest=0;for(unsigned i=0;i<128;i++){if(actor_world_as[i].actor==a&&actor_world_as[i].block==block&&actor_world_as[i].generation==generation){at=i;break;}if(actor_world_as[i].at<actor_world_as[oldest].at)oldest=i;}
    if(at==128){at=oldest;memset(&actor_world_as[at],0,sizeof actor_world_as[at]);}WorldAS *cache=&actor_world_as[at];cache->actor=a;cache->block=block;cache->generation=generation;cache->at=now_ms();
    Track *t=&cache->tracks[group];memset(t,0,sizeof *t);snprintf(t->name,64,"%s",(char *)(uintptr_t)p);t->sequence=++actor_world_capture_seq;t->force=(LD8(request+0x21)&1)!=0;t->slot=slot;pthread_mutex_unlock(&actor_world_mutex);
}
// Gate the Actor dispatcher before its virtual RootAI call, including overrides.
// The owner backlink
// and retained generation must both agree. No guest call occurs in this hook.
bool rt_hyrulem_world_ai(Cpu *c){
    if(!supported()||!g_site.hm_world_ai_dispatch||!actor_world_ready(c->x[19]))return false;
    uint64_t a=c->x[19],root=LD64(a+0x558);if(!range(root,16)||LD64(root+8)!=a)return false;
    bool block=false;pthread_mutex_lock(&actor_world_mutex);
    if(actor_world_mode.role>=2&&actor_world_live.role==actor_world_mode.role&&actor_world_live.epoch==actor_world_mode.epoch&&!strcmp(actor_world_live.world,actor_world_mode.world)&&!strcmp(actor_world_live.session,actor_world_mode.session)&&actor_world_mode.heartbeat&&now_ms()-actor_world_mode.heartbeat<1500)for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){WorldBinding *b=&actor_world_bindings[i];if(b->actor==a&&b->id&&b->applied_at&&now_ms()-b->applied_at<500&&actor_world_index(b)){block=true;actor_world_ai_skipped++;break;}}
    pthread_mutex_unlock(&actor_world_mutex);return block;
}
static uint32_t world_get32(const unsigned char *p){return (uint32_t)p[0]|(uint32_t)p[1]<<8|(uint32_t)p[2]<<16|(uint32_t)p[3]<<24;}
static void world_put32(unsigned char *p,uint32_t n){for(unsigned i=0;i<4;i++)p[i]=(unsigned char)(n>>(8*i));}
static uint64_t world_get64(const unsigned char *p){return world_get32(p)|(uint64_t)world_get32(p+4)<<32;}
static void world_put64(unsigned char *p,uint64_t n){world_put32(p,(uint32_t)n);world_put32(p+4,(uint32_t)(n>>32));}
static const char world_b64[]="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
static size_t actor_world_encode(const WorldActor *actors,unsigned count,char *out){
    unsigned char raw[8+ACTOR_WORLD_MAX*ACTOR_WORLD_STRIDE]={0};memcpy(raw,"HMW1",4);raw[4]=count&255;raw[5]=count>>8;
    for(unsigned i=0;i<count;i++){unsigned char *p=raw+8+i*ACTOR_WORLD_STRIDE;const WorldActor *a=&actors[i];world_put64(p,a->id);world_put32(p+8,a->hash);world_put32(p+12,a->kind);memcpy(p+16,a->name,strnlen(a->name,63));
        for(unsigned j=0;j<15;j++){float f=j<3?a->spawn[j]:a->matrix[j-3];uint32_t bits;memcpy(&bits,&f,4);world_put32(p+80+j*4,bits);}
        for(unsigned j=0;j<2;j++){unsigned char *t=p+140+j*80;memcpy(t,a->tracks[j].name,strnlen(a->tracks[j].name,63));world_put64(t+64,a->tracks[j].sequence);world_put32(t+72,a->tracks[j].force);world_put32(t+76,a->tracks[j].slot);}
    }
    size_t n=8+count*ACTOR_WORLD_STRIDE,w=0;for(size_t i=0;i<n;i+=3){uint32_t v=(uint32_t)raw[i]<<16;if(i+1<n)v|=(uint32_t)raw[i+1]<<8;if(i+2<n)v|=raw[i+2];out[w++]=world_b64[v>>18&63];out[w++]=world_b64[v>>12&63];out[w++]=i+1<n?world_b64[v>>6&63]:'=';out[w++]=i+2<n?world_b64[v&63]:'=';}out[w]=0;return w;
}
static bool actor_world_decode(const char *text,ActorWorldMode *mode){
    unsigned char raw[8+ACTOR_WORLD_MAX*ACTOR_WORLD_STRIDE];size_t n=strlen(text),w=0;if(!n||n%4||n>ACTOR_WORLD_B64)return false;
    for(size_t i=0;i<n;i+=4){uint32_t bits=0;unsigned pad=0;for(unsigned j=0;j<4;j++){const char *p=strchr(world_b64,text[i+j]);if(text[i+j]=='='){if(i+4!=n||j<2)return false;pad++;bits<<=6;}else{if(pad||!p||!text[i+j])return false;bits=bits<<6|(unsigned)(p-world_b64);}}
        if(pad>2||(pad==2&&(bits&0xffff))||(pad==1&&(bits&255))||w+3-pad>sizeof raw)return false;raw[w++]=(unsigned char)(bits>>16);if(pad<2)raw[w++]=(unsigned char)(bits>>8);if(!pad)raw[w++]=(unsigned char)bits;
    }
    if(w<8||memcmp(raw,"HMW1",4)||raw[6]||raw[7])return false;unsigned count=raw[4]|(unsigned)raw[5]<<8;if(count>ACTOR_WORLD_MAX||w!=8+count*ACTOR_WORLD_STRIDE)return false;
    for(unsigned i=0;i<count;i++){const unsigned char *p=raw+8+i*ACTOR_WORLD_STRIDE;WorldActor *a=&mode->actors[i];memset(a,0,sizeof *a);a->id=world_get64(p);a->hash=world_get32(p+8);a->kind=world_get32(p+12);if(!a->id||a->id>INT64_MAX||!a->hash||a->hash==UINT32_MAX||!memchr(p+16,0,64))return false;memcpy(a->name,p+16,64);size_t z=strnlen(a->name,64);for(size_t k=z;k<64;k++)if(a->name[k])return false;if(!clip_name(a->name)||actor_world_kind(a->name)!=a->kind||!a->kind)return false;for(size_t k=0;k<z;k++)if(a->name[k]=='-')return false;
        for(unsigned j=0;j<15;j++){uint32_t bits=world_get32(p+80+j*4);float f;memcpy(&f,&bits,4);if(j<3){if(!isfinite(f)||fabsf(f)>(j==1?50000.f:100000.f))return false;a->spawn[j]=f;}else a->matrix[j-3]=f;}
        if(!actor_world_matrix(a->matrix))return false;float distance=0;for(unsigned j=0;j<3;j++){float d=a->matrix[3+j*4]-mode->center[j];distance+=d*d;}if(distance>181.f*181.f)return false;
        for(unsigned j=0;j<2;j++){const unsigned char *t=p+140+j*80;Track *track=&a->tracks[j];if(!memchr(t,0,64))return false;memcpy(track->name,t,64);z=strnlen(track->name,64);for(size_t k=z;k<64;k++)if(track->name[k])return false;track->sequence=world_get64(t+64);uint32_t force=world_get32(t+72);track->slot=world_get32(t+76);if((z&&!clip_name(track->name))||force>1||track->slot>1||track->sequence>INT64_MAX)return false;track->force=force!=0;}
        for(unsigned j=0;j<i;j++)if(a->id==mode->actors[j].id||actor_world_same(a,&mode->actors[j]))return false;
    }mode->count=count;return true;
}
static void actor_world_release(Cpu *c,WorldBinding *b){
    // Exclude it from worker hooks before releasing the retained identity.
    pthread_mutex_lock(&actor_world_mutex);b->id=0;b->actor=0;pthread_mutex_unlock(&actor_world_mutex);
    if(b->storage)call(c,g_site.actor_index_release,b->storage+INDEX,0,0,0,0,0,0);memset(b->played,0,sizeof b->played);b->seen=0;
}
static void actor_world_close(void){pthread_mutex_lock(&actor_world_mutex);actor_world_mode.role=0;actor_world_mode.heartbeat=0;pthread_mutex_unlock(&actor_world_mutex);}
static bool actor_world_integer(const char *p,uint64_t *n){if(!strcmp(p,"0")){*n=0;return true;}if(!*p)return false;uint64_t v=0;for(;*p;p++){if(*p<'0'||*p>'9'||v>(INT64_MAX-(unsigned)(*p-'0'))/10)return false;v=v*10+(unsigned)(*p-'0');}*n=v;return true;}
static bool actor_world_token(const char *s,unsigned length){if(strlen(s)!=length)return false;for(unsigned i=0;i<length;i++)if(!((s[i]>='0'&&s[i]<='9')||(s[i]>='a'&&s[i]<='f')))return false;return true;}
static void actor_world_sample(int argc,char **argv,ControlReply *r){
    ActorWorldMode next={0};uint64_t role,pause;
    if((argc!=11&&argc!=12)||!world_supported()||!g_site.hm_world_ai_dispatch||!actor_world_token(argv[1],16)||!actor_world_token(argv[2],24)||!actor_world_integer(argv[3],&next.epoch)||!next.epoch||!actor_world_integer(argv[4],&role)||role>3||!actor_world_integer(argv[5],&next.sequence)||!actor_world_integer(argv[6],&pause)||pause>1){control_fail(r,"Invalid authoritative world command");return;}
    for(unsigned i=0;i<3;i++){char *end;errno=0;next.center[i]=strtof(argv[7+i],&end);if(errno||*end||!isfinite(next.center[i])||fabsf(next.center[i])>(i==1?50000.f:100000.f)){control_fail(r,"Invalid world center");return;}}
    if(argc==12){if(!actor_scene_valid(argv[11])){control_fail(r,"Invalid world scene");return;}snprintf(next.scene,sizeof next.scene,"%s",argv[11]);}
    next.role=(unsigned)role;next.paused=pause!=0;next.heartbeat=now_ms();snprintf(next.session,17,"%s",argv[1]);snprintf(next.world,25,"%s",argv[2]);
    if(role>=2&&(!next.sequence||!actor_world_decode(argv[10],&next))){control_fail(r,"Invalid world actor frame");return;}
    if(role<2&&strcmp(argv[10],"-")){control_fail(r,"Unexpected world payload");return;}
    pthread_mutex_lock(&mutex);bool session_ok=!strcmp(desired.session,next.session);bool scene_ok=!next.scene[0]||!strcmp(next.scene,local_scene);pthread_mutex_unlock(&mutex);
    if(!scene_ok){control_field_bool(r,"ready",false);return;}
    pthread_mutex_lock(&actor_world_mutex);
    bool same=!strcmp(actor_world_mode.scene,next.scene)&&!strcmp(actor_world_mode.session,next.session)&&!strcmp(actor_world_mode.world,next.world)&&actor_world_mode.epoch==next.epoch;
    bool ok=session_ok&&(!actor_world_mode.epoch||strcmp(actor_world_mode.session,next.session)||strcmp(actor_world_mode.world,next.world)||next.epoch>=actor_world_mode.epoch);
    if(ok&&same&&role>=2&&actor_world_mode.role==role&&next.sequence<actor_world_mode.sequence)ok=false;
    if(ok){if(same&&role>=2&&actor_world_mode.role==role&&next.sequence==actor_world_mode.sequence)actor_world_mode.heartbeat=next.heartbeat;else actor_world_mode=next;}
    WorldActor copy[ACTOR_WORLD_MAX];unsigned count=actor_world_seen_count;memcpy(copy,actor_world_seen,sizeof *copy*count);uint64_t seq=actor_world_frame_seq;float center[3];memcpy(center,actor_world_center,sizeof center);bool complete=actor_world_complete;unsigned matched=actor_world_matched,missing=actor_world_missing,applied=actor_world_applied,ai=actor_world_ai_skipped,as=actor_world_as_applied;bool ready=ok&&(role==1||role==3)&&actor_world_live.role==role&&actor_world_live.epoch==next.epoch&&!strcmp(actor_world_live.scene,next.scene)&&!strcmp(actor_world_live.world,next.world)&&!strcmp(actor_world_live.session,next.session)&&actor_world_live.heartbeat&&now_ms()-actor_world_live.heartbeat<1500;bool paused=actor_world_live.paused;
    pthread_mutex_unlock(&actor_world_mutex);if(!ok){control_fail(r,"Wrong session, epoch or stale world frame");return;}
    char encoded[ACTOR_WORLD_B64+1];actor_world_encode(copy,count,encoded);control_field_str(r,"actors_b64",encoded);control_field_int(r,"sequence",seq);control_field_bool(r,"ready",ready);control_field_bool(r,"paused",paused);control_field_bool(r,"complete",complete);control_field_num(r,"x",center[0]);control_field_num(r,"y",center[1]);control_field_num(r,"z",center[2]);control_field_int(r,"captured",count);control_field_int(r,"matched",matched);control_field_int(r,"missing",missing);control_field_int(r,"applied",applied);control_field_int(r,"ai_skipped",ai);control_field_int(r,"animations_applied",as);
}

typedef struct {uint64_t actor;WorldActor state;float distance;} WorldCandidate;
static void actor_world_scan(Cpu *c,const ActorWorldMode *mode,uint64_t local){
    WorldCandidate candidates[ACTOR_WORLD_MAX];unsigned count=0,total=0;uint64_t creator=LD64(g_site.hm_creator);if(!range(creator,0xe8)||LD32(creator+0x4c)!=0x7b8)return;
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(!actor_world_bindings[i].storage){uint64_t st=(uint64_t)(uintptr_t)hle_alloc(BYTES);if(range(st,BYTES)){memset((void *)(uintptr_t)st,0,BYTES);ST32(st+INDEX+8,UINT32_MAX);pthread_mutex_lock(&actor_world_mutex);actor_world_bindings[i].storage=st;pthread_mutex_unlock(&actor_world_mutex);}}
    call(c,g_site.pouch_lock,creator+0xa8,0,0,0,0,0,0);uint64_t head=creator+0x38,node=LD64(head);unsigned limit=LD32(creator+0x48);if(limit>8192)limit=0;
    for(unsigned step=0;node!=head&&step<limit;step++){
        if(!range(node,16)||node<0x7b8)break;uint64_t a=node-0x7b8;node=LD64(node);if(a==local)continue;WorldCandidate candidate={.actor=a};if(!actor_world_read(a,&candidate.state))continue;
        for(unsigned i=0;i<3;i++){float d=candidate.state.matrix[3+i*4]-mode->center[i];candidate.distance+=d*d;}
        if(mode->role==1||mode->role==3){if(candidate.distance>180.f*180.f)continue;}else{bool wanted=false;for(unsigned i=0;i<mode->count;i++)if(actor_world_same(&candidate.state,&mode->actors[i])){wanted=true;break;}if(!wanted)continue;}
        bool duplicate=false;for(unsigned i=0;i<count;i++)if(actor_world_same(&candidate.state,&candidates[i].state)){duplicate=true;break;}if(duplicate)continue;total++;
        unsigned at=count;if(at==ACTOR_WORLD_MAX){at=0;for(unsigned i=1;i<count;i++)if(candidates[i].distance>candidates[at].distance)at=i;if(candidate.distance>=candidates[at].distance)continue;}else count++;candidates[at]=candidate;
    }
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){
        WorldBinding *b=&actor_world_bindings[i];bool keep=false;
        for(unsigned j=0;j<count;j++)if(b->actor==candidates[j].actor&&actor_world_index(b)){keep=true;break;}
        if(b->actor&&!keep)actor_world_release(c,b);
    }
    for(unsigned j=0;j<count;j++){
        WorldCandidate *candidate=&candidates[j];WorldBinding *b=NULL;
        for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].actor==candidate->actor&&actor_world_index(&actor_world_bindings[i])){b=&actor_world_bindings[i];break;}
        if(!b){for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(!actor_world_bindings[i].actor&&actor_world_bindings[i].storage){b=&actor_world_bindings[i];break;}if(!b||!call(c,g_site.actor_index_assign,b->storage+INDEX,candidate->actor,1,0,0,0,0))continue;
            pthread_mutex_lock(&actor_world_mutex);b->actor=candidate->actor;b->id=mode->role==1?++actor_world_id:0;pthread_mutex_unlock(&actor_world_mutex);b->started=0;memset(b->played,0,sizeof b->played);
        }
        b->state=candidate->state;b->seen=now_ms();
    }
    call(c,g_site.pouch_unlock,creator+0xa8,0,0,0,0,0,0);
    pthread_mutex_lock(&actor_world_mutex);actor_world_complete=total<=ACTOR_WORLD_MAX&&limit>0;pthread_mutex_unlock(&actor_world_mutex);
}
static void actor_world_rotation(const float m[12],float q[4]){
    float trace=m[0]+m[5]+m[10],s;
    if(trace>0){s=sqrtf(trace+1.f)*2.f;q[3]=s*.25f;q[0]=(m[9]-m[6])/s;q[1]=(m[2]-m[8])/s;q[2]=(m[4]-m[1])/s;}
    else if(m[0]>m[5]&&m[0]>m[10]){s=sqrtf(fmaxf(.0001f,1.f+m[0]-m[5]-m[10]))*2.f;q[3]=(m[9]-m[6])/s;q[0]=s*.25f;q[1]=(m[1]+m[4])/s;q[2]=(m[2]+m[8])/s;}
    else if(m[5]>m[10]){s=sqrtf(fmaxf(.0001f,1.f+m[5]-m[0]-m[10]))*2.f;q[3]=(m[2]-m[8])/s;q[0]=(m[1]+m[4])/s;q[1]=s*.25f;q[2]=(m[6]+m[9])/s;}
    else{s=sqrtf(fmaxf(.0001f,1.f+m[10]-m[0]-m[5]))*2.f;q[3]=(m[4]-m[1])/s;q[0]=(m[2]+m[8])/s;q[1]=(m[6]+m[9])/s;q[2]=s*.25f;}
    float norm=0;for(unsigned i=0;i<4;i++)norm+=q[i]*q[i];norm=sqrtf(fmaxf(norm,.0001f));for(unsigned i=0;i<4;i++)q[i]/=norm;
}
static void actor_world_interpolate(const float from[12],const float to[12],float t,float out[12]){
    if(t>=1){memcpy(out,to,48);return;}if(t<=0){memcpy(out,from,48);return;}
    float a[4],b[4],q[4],dot=0,norm=0;actor_world_rotation(from,a);actor_world_rotation(to,b);for(unsigned i=0;i<4;i++)dot+=a[i]*b[i];for(unsigned i=0;i<4;i++){q[i]=a[i]*(1-t)+b[i]*t*(dot<0?-1.f:1.f);norm+=q[i]*q[i];}norm=sqrtf(fmaxf(norm,.0001f));for(unsigned i=0;i<4;i++)q[i]/=norm;
    float x=q[0],y=q[1],z=q[2],w=q[3];float m[12]={1-2*(y*y+z*z),2*(x*y-z*w),2*(x*z+y*w),0,2*(x*y+z*w),1-2*(x*x+z*z),2*(y*z-x*w),0,2*(x*z-y*w),2*(y*z+x*w),1-2*(x*x+y*y),0};
    for(unsigned i=0;i<3;i++)m[3+i*4]=from[3+i*4]+(to[3+i*4]-from[3+i*4])*t;memcpy(out,m,48);
}
static void actor_world_animate(Cpu *c,WorldBinding *b,uint64_t a,const WorldActor *state,uint64_t vt){
    uint64_t as=LD64(a+0x560);if(!range(as,0xe0))return;unsigned count=LD32(as+0xb8);if(!count||count>8||!range(LD64(as+0xc0),count*8))return;
    for(unsigned i=0;i<2&&i<count;i++){
        const Track *t=&state->tracks[i];Track *old=&b->played[i];if(!t->name[0]||(!strcmp(t->name,old->name)&&t->slot==old->slot&&(!t->force||t->sequence==old->sequence)))continue;
        snprintf((char *)(uintptr_t)(b->storage+ASNAME),64,"%s",t->name);ST64(b->storage+SSTR,vt);ST64(b->storage+SSTR+8,b->storage+ASNAME);
        pthread_mutex_lock(&actor_world_mutex);uint64_t before=actor_world_capture_seq;pthread_mutex_unlock(&actor_world_mutex);
        Cpu scratch=*c;scratch.x[0]=as;scratch.x[1]=b->storage+SSTR;scratch.x[2]=i;scratch.x[3]=t->slot;scratch.x[4]=t->force||strcmp(t->name,old->name);scratch.x[5]=1;scratch.x[6]=0;float blend=-1.f;scratch.v[0]=scratch.v[1]=0;memcpy(&scratch.v[0],&blend,4);memcpy(&scratch.v[1],&blend,4);guest_call(&scratch,g_site.hm_play_as);
        // This native wrapper has no success return. Its request-entry hook is
        // reached after resource lookup; acknowledge that observation instead.
        bool accepted=false;pthread_mutex_lock(&actor_world_mutex);
        for(unsigned j=0;j<128;j++){WorldAS *observed=&actor_world_as[j];Track *track=&observed->tracks[i];if(observed->actor==a&&actor_world_weak(a,observed->block,observed->generation)&&track->sequence>before&&track->slot==t->slot&&!strcmp(track->name,t->name)){accepted=true;break;}}
        if(accepted){*old=*t;actor_world_as_applied++;}pthread_mutex_unlock(&actor_world_mutex);
    }
}
static void actor_world_tick(Cpu *c,uint64_t local,uint64_t vt,bool paused){
    ActorWorldMode mode;pthread_mutex_lock(&actor_world_mutex);mode=actor_world_mode;pthread_mutex_unlock(&actor_world_mutex);uint64_t now=now_ms();
    bool fresh=mode.role&&mode.heartbeat&&now-mode.heartbeat<1500&&(!mode.scene[0]||!strcmp(mode.scene,local_scene));bool changed=strcmp(mode.scene,actor_world_live.scene)||mode.role!=actor_world_live.role||mode.epoch!=actor_world_live.epoch||strcmp(mode.session,actor_world_live.session)||strcmp(mode.world,actor_world_live.world);
    if(!fresh||changed){for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].actor)actor_world_release(c,&actor_world_bindings[i]);actor_world_scan_at=actor_world_applied_seq=actor_world_arrival=0;pthread_mutex_lock(&actor_world_mutex);actor_world_seen_count=actor_world_matched=actor_world_missing=0;pthread_mutex_unlock(&actor_world_mutex);}
    if(!fresh){pthread_mutex_lock(&actor_world_mutex);memset(&actor_world_live,0,sizeof actor_world_live);pthread_mutex_unlock(&actor_world_mutex);return;}
    if(mode.role==1||mode.role==3){for(unsigned i=0;i<3;i++)mode.center[i]=read_float(local+0x3a4+i*16);mode.paused=paused;}
    pthread_mutex_lock(&actor_world_mutex);actor_world_live=mode;pthread_mutex_unlock(&actor_world_mutex);
    if(paused&&mode.role>=2)return;
    bool new_frame=mode.role>=2&&mode.sequence!=actor_world_applied_seq;
    bool scanned=!actor_world_scan_at||now-actor_world_scan_at>=100;if(scanned){actor_world_scan(c,&mode,local);actor_world_scan_at=now;}
    if(mode.role==1&&!scanned)return;
    WorldActor seen[ACTOR_WORLD_MAX];unsigned expected_source=0;for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].actor)expected_source++;unsigned count=0,matched=0,missing=mode.role>=2?mode.count:0;uint64_t interval=actor_world_arrival&&mode.heartbeat>actor_world_arrival?mode.heartbeat-actor_world_arrival:100;if(interval<50)interval=50;if(interval>200)interval=200;
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++){
        WorldBinding *b=&actor_world_bindings[i];if(!b->actor||!actor_world_index(b))continue;ActorRef ref=resolve_at(c,b->storage,b->actor);if(!ref.ptr||(ref.state!=1&&ref.state!=2)){if(ref.ptr)release_at(c,b->storage);continue;}
        WorldActor current={0};if(!actor_world_read(ref.ptr,&current)||!actor_world_same(&current,&b->state)){release_at(c,b->storage);actor_world_release(c,b);continue;}
        if(mode.role==1||mode.role==3){current.id=mode.role==3?i+1:b->id;pthread_mutex_lock(&actor_world_mutex);for(unsigned j=0;j<128;j++)if(actor_world_as[j].actor==b->actor&&actor_world_weak(b->actor,actor_world_as[j].block,actor_world_as[j].generation)){memcpy(current.tracks,actor_world_as[j].tracks,sizeof current.tracks);break;}pthread_mutex_unlock(&actor_world_mutex);if(count<ACTOR_WORLD_MAX)seen[count++]=current;}
        if(mode.role>=2){
            const WorldActor *target=NULL;for(unsigned j=0;j<mode.count;j++)if(actor_world_same(&current,&mode.actors[j])){target=&mode.actors[j];break;}
            if(target){matched++;missing--;if(new_frame||b->id!=target->id||!b->started){b->duration=interval;
                    // Background/culled NPC movement can rewrite the actor's
                    // matrix between barriers. Continue the network trajectory,
                    // not that local simulation, after the first binding.
                    const void *from=b->id==target->id&&b->started&&b->applied_at?(void *)(uintptr_t)(b->storage+MATRIX):(const void *)current.matrix;
                    memcpy(b->from,from,48);memcpy(b->target,target->matrix,48);b->started=now;pthread_mutex_lock(&actor_world_mutex);b->id=target->id;pthread_mutex_unlock(&actor_world_mutex);}
                // All actors use one interpolation duration for this received
                // frame; keep it with the binding, not the next heartbeat.
                float distance=0;for(unsigned j=0;j<3;j++){float d=b->from[3+j*4]-b->target[3+j*4];distance+=d*d;}float t=mode.paused||distance>400?1.f:(float)(now-b->started)/(float)b->duration;
                float m[12];actor_world_interpolate(b->from,b->target,t,m);memcpy((void *)(uintptr_t)(b->storage+MATRIX),m,48);
                call(c,g_site.hm_set_mtx,ref.ptr,b->storage+MATRIX,0,0,0,0,0);call(c,g_site.hm_set_mtx,ref.ptr,b->storage+MATRIX,1,0,0,0,0);actor_world_animate(c,b,ref.ptr,target,vt);actor_world_rebase(c,ref.ptr,b->storage,m);pthread_mutex_lock(&actor_world_mutex);actor_world_applied++;b->applied_at=now;pthread_mutex_unlock(&actor_world_mutex);
            }else if(b->id){pthread_mutex_lock(&actor_world_mutex);b->id=0;pthread_mutex_unlock(&actor_world_mutex);}
        }
        release_at(c,b->storage);
    }
    if(new_frame){actor_world_applied_seq=mode.sequence;actor_world_arrival=mode.heartbeat;}
    pthread_mutex_lock(&actor_world_mutex);actor_world_matched=matched;actor_world_missing=missing;
    if((mode.role==1||mode.role==3)&&now==actor_world_scan_at){if(count!=expected_source)actor_world_complete=false;memcpy(actor_world_seen,seen,sizeof *seen*count);actor_world_seen_count=count;actor_world_frame_seq++;memcpy(actor_world_center,mode.center,sizeof actor_world_center);}
    pthread_mutex_unlock(&actor_world_mutex);
}
static void actor_world_reset(void){
    pthread_mutex_lock(&actor_world_mutex);memset(&actor_world_mode,0,sizeof actor_world_mode);memset(&actor_world_live,0,sizeof actor_world_live);memset(actor_world_bindings,0,sizeof actor_world_bindings);memset(actor_world_as,0,sizeof actor_world_as);memset(actor_world_seen,0,sizeof actor_world_seen);actor_world_capture_seq=actor_world_frame_seq=actor_world_id=actor_world_scan_at=actor_world_applied_seq=actor_world_arrival=0;actor_world_seen_count=actor_world_matched=actor_world_missing=actor_world_applied=actor_world_ai_skipped=actor_world_as_applied=0;actor_world_complete=false;memset(actor_world_center,0,sizeof actor_world_center);pthread_mutex_unlock(&actor_world_mutex);
}

// A felled map tree is replaced by new actors. Lowering its HP alone leaves
// the original model and collider standing. Keep the original until a native
// stump has activated, then release the replacement to the game's lifecycle.
typedef struct {
    EnemyHealth tree;
    uint64_t original, block, original_storage, started, retry_at;
    uint32_t generation;
    GearVisual stump;
    float matrix[12];
    bool cancelling;
} TreeReplacement;
static TreeReplacement replacement;
static uint64_t ground_storage(void);

static bool tree_has_stump(const char *name) {
    return !strcmp(name,"Obj_TreeConiferous_A_02") || !strcmp(name,"Obj_TreeBroadleaf_A_L");
}
static void tree_queue(uint64_t actor,const EnemyHealth *tree) {
    TreeReplacement *t=&replacement;
    if(t->original || (now_ms()<t->retry_at && same_enemy(tree,&t->tree)))return;
    uint64_t block=LD64(actor+0x68);
    uint32_t gen=LD32(actor+0x60);
    if(!actor_world_weak(actor,block,gen))return;
    memcpy(t->matrix,(void *)(uintptr_t)(actor+0x398),sizeof t->matrix);
    if(!actor_world_matrix(t->matrix))return;
    t->tree=*tree;t->original=actor;t->block=block;t->generation=gen;t->started=now_ms();
}
static void tree_finish(Cpu *c,bool committed) {
    TreeReplacement *t=&replacement;
    if(!committed && !remove_gear(c,&t->stump)){t->cancelling=true;return;}
    if(committed)call(c,g_site.actor_index_release,t->stump.storage+INDEX,0,0,0,0,0,0);
    if(t->original_storage)call(c,g_site.actor_index_release,t->original_storage+INDEX,0,0,0,0,0,0);
    uint64_t st=t->stump.storage;
    memset(&t->stump,0,sizeof t->stump);t->stump.storage=st;
    t->original=0;t->cancelling=false;t->retry_at=now_ms()+(committed?2500:10000);
}
static void tree_tick(Cpu *c,uint64_t vt,bool connected) {
    TreeReplacement *t=&replacement;GearVisual *g=&t->stump;
    if(!t->original)return;
    if(t->cancelling || !connected || now_ms()-t->started>10000 ||
       !actor_world_weak(t->original,t->block,t->generation)){tree_finish(c,false);return;}
    if(!t->original_storage)t->original_storage=ground_storage();
    if(!g->storage)g->storage=ground_storage();
    if(!t->original_storage||!g->storage){tree_finish(c,false);return;}
    if(!LD8(t->original_storage+INDEX+12) &&
       !call(c,g_site.actor_index_assign,t->original_storage+INDEX,t->original,1,0,0,0,0)){tree_finish(c,false);return;}
    uint64_t st=g->storage;
    if(!g->spawning&&!g->actor) {
        uint64_t creator=LD64(g_site.hm_creator),manager=LD64(g_site.hm_heap_mgr);
        if(!range(creator,0x60)||!range(manager,0x30)||LD8(creator+0x59))return;
        uint64_t heap=LD64(manager+0x28);if(!range(heap,8))return;
        memset((void *)(uintptr_t)(st+HANDLE),0,16);memset((void *)(uintptr_t)(st+PARAMS),0,0xd8);
        memcpy((void *)(uintptr_t)(st+VECTOR),t->tree.position,12);
        ST64(st+SSTR,vt);ST64(st+SSTR+8,st+KEY);
        call(c,g_site.hm_param_add,st+PARAMS+0x10,st+VECTOR,st+SSTR,12,4,0,0);
        snprintf((char *)(uintptr_t)(st+NAME),64,"%.48s_Stump",t->tree.name);
        if(!call(c,g_site.hm_create,creator,st+NAME,heap,st+HANDLE,st+PARAMS,0,1)){tree_finish(c,false);return;}
        g->spawning=true;return;
    }
    if(g->spawning) {
        if(call(c,g_site.hm_handle_failed,st+HANDLE,0,0,0,0,0,0)){tree_finish(c,false);return;}
        if(!call(c,g_site.hm_handle_ready,st+HANDLE,0,0,0,0,0,0))return;
        uint64_t a=call(c,g_site.hm_handle_get,st+HANDLE,0,0,0,0,0,0);
        if(!range(a,0x7d0)||!call(c,g_site.actor_index_assign,st+INDEX,a,1,0,0,0,0)){tree_finish(c,false);return;}
        g->actor=a;
        uint64_t awake=call(c,g_site.hm_handle_wake,st+HANDLE,0,0,0,0,0,0);g->spawning=false;
        if(awake!=a){tree_finish(c,false);return;}
    }
    ActorRef stump=resolve_at(c,st,g->actor);
    if(!stump.ptr){if(stump.invalid)tree_finish(c,false);return;}
    bool ready=stump.state==1 && range(LD64(stump.ptr+0x4e0),0xa4);
    if(ready){memcpy((void *)(uintptr_t)(st+MATRIX),t->matrix,48);call(c,g_site.hm_set_mtx,stump.ptr,st+MATRIX,0,0,0,0,0);}
    release_at(c,st);
    if(!ready)return;
    ActorRef original=resolve_at(c,t->original_storage,t->original);
    if(!original.ptr){if(original.invalid)tree_finish(c,false);return;}
    if(original.state!=1){release_at(c,t->original_storage);return;}
    uint64_t life=enemy_life(original.ptr);if(!life){release_at(c,t->original_storage);tree_finish(c,false);return;}
    ST32(life,0);
    call(c,g_site.hm_delete_actor,original.ptr,0,0,0,0,0,0);
    release_at(c,t->original_storage);
    tree_finish(c,true);
}
