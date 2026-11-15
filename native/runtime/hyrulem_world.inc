// Map-entity damage ledger. A missing actor is never a destruction event.
static EnemyHealth world_history[HEALTH_MAX],world_zero[HEALTH_MAX];
static int world_history_count,world_zero_count;static bool world_history_reset;
static bool map_damage_name(const char *s){
    return !strncmp(s,"Enemy_",6)||(!strncmp(s,"Obj_Tree",8)&&!strstr(s,"_Stump")&&!strstr(s,"_Trunk")&&!strstr(s,"Root")&&!strstr(s,"Branch"));
}
static EnemyHealth *world_note(EnemyHealth *e){
    int at=0;while(at<world_history_count&&!same_enemy(e,&world_history[at]))at++;
    if(at==world_history_count){if(at==HEALTH_MAX)return NULL;world_history[world_history_count++]=*e;}
    EnemyHealth *old=&world_history[at];if(e->health<old->health){uint64_t delta=(uint64_t)(old->health-e->health);if(old->damage<=INT64_MAX-delta)old->damage+=delta;}
    e->damage=old->damage;return old;
}
// Called before the game's deletion method can erase the evidence. Only copy
// a confirmed native life==0 and map identity; do not call game code here.
void rt_hyrulem_capture_zero(Cpu *c){
    ground_capture_destroy(c);
    if(!world_supported())return;uint64_t a=c->x[0];if(!range(a,0x7d0))return;EnemyHealth e={0};
    if(!enemy_name(LD64(a+0x10),e.name))return;uint64_t life=enemy_life(a),map=LD64(a+0x7c8);if(!life||LD32(life)||!range(map,0x70))return;
    e.hash=LD32(map+0x58);if(!e.hash||e.hash==UINT32_MAX)return;
    for(int i=0;i<3;i++){e.spawn[i]=read_float(map+0x5c+i*4);e.position[i]=read_float(a+0x3a4+i*16);if(!isfinite(e.spawn[i])||!isfinite(e.position[i])||fabsf(e.spawn[i])>100000||fabsf(e.position[i])>100000)return;}
    pthread_mutex_lock(&mutex);
    if(desired.session[0]&&desired.received&&now_ms()-desired.received<2500){int at=0;while(at<world_zero_count&&!same_enemy(&e,&world_zero[at]))at++;if(at<HEALTH_MAX){world_zero[at]=e;if(at==world_zero_count)world_zero_count++;}}
    pthread_mutex_unlock(&mutex);
}
