    // Factory-created materials are held before export, then pass through the
    // existing one-credit reservation. No game code is called by either hook.
    ground_reset();birth_reset();strcpy(local_scene,"MainField");r.failed=false;
    char *loot_open[]={"hm_items_open",desired.session,"0123456789abcdef01234567","3437.5","235.9","2101.5","player","MainField"};ground_open(8,loot_open,&r);assert(!r.failed);
    actor_world_mode=(ActorWorldMode){.role=3,.heartbeat=now_ms()};strcpy(actor_world_mode.session,desired.session);strcpy(actor_world_mode.world,ground_mode.world);strcpy(actor_world_mode.scene,local_scene);
    ground_live=ground_mode;ground_session_ready=true;
    ST64(parent,HEAP_BASE+0xfbc00);ST64(HEAP_BASE+0xfbeb8,MAIN_BASE+0x140);ST32(MAIN_BASE+0x140,0x91240000);ST32(MAIN_BASE+0x144,0xd65f03c0); // add x0,x0,#0x900; ret
    ST32(parent+0x900,13);for(unsigned i=0;i<3;i++)memcpy((void *)(uintptr_t)(parent+0x95c+i*4),&player_root[i*4+3],4);
    memcpy((void *)(uintptr_t)(born+0x398),player_root,48);ST32(birth_weak+0x40,9);ST8(born+0x64,1);
    calls_before_birth=calls;rt_hyrulem_capture_birth(&job);assert(!loot_captured&&calls==calls_before_birth);
    EnemyHealth loot_parent={.hash=123};strcpy(loot_parent.scene,local_scene);strcpy(loot_parent.name,"Enemy_Bokoblin_Junior");memcpy(loot_parent.spawn,(void *)(uintptr_t)(parent+0x95c),12);loot_parent_note(parent,&loot_parent);
    // Preserve the alias even if the parent has no native map object.
    ST64(parent+0x7c8,0);ST32(parent+0x900,0);rt_hyrulem_capture_birth(&job);
    assert(loot_captured==1&&native_drops[0].active&&!native_drops[0].exported&&native_drops[0].source.parent.hash==123&&native_drops[0].source.parent.ordinal==1&&calls==calls_before_birth);
    ST64(accessor+8,born);ST64(safe+8,born+0x800);assert(rt_hyrulem_item_pickup(&pickup));
    assert(native_drops[0].active&&native_drops[0].pickup_pending&&ground_events[0].request==1&&calls==calls_before_birth&&!memcmp(&job,&job_before,sizeof job));
    ground_seen_count=GROUND_MAX;for(unsigned i=0;i<GROUND_MAX;i++){GroundSource *m=&ground_seen[i];memset(m,0,sizeof *m);m->hash=1000+i;m->local=i+1;strcpy(m->name,"Item_Fruit_A");}
    ground_status(0,NULL,&r);assert(native_drops[0].exported); // full map window cannot starve a pending birth
    char loot_local[32];snprintf(loot_local,sizeof loot_local,"%llu",(unsigned long long)native_drops[0].source.local);
    char *loot_state[]={"hm_item_state",desired.session,ground_mode.world,"1","Item_Enemy_00","0","3437.5","235.9","2101.5","3437.5","235.9","2101.5",loot_local,"1","1","1","1",ground_origin,"0","0","0","1","123","Enemy_Bokoblin_Junior","3437.5","235.9","2101.5","1"};
    char *enrolled[]={"hm_world_drop_enrolled",desired.session,ground_mode.world,loot_local};calls_before_birth=calls;ground_world_enrolled(4,enrolled,&r);assert(!r.failed&&native_drops[0].enrolled&&calls==calls_before_birth);
    ground_update(28,loot_state,&r);assert(!r.failed&&ground_state[0].source.parent.hash==123);
    ST32(birth_weak+0x40,10);weapon_test=false;empty_inventory=false;ST32(ground_pouch_item+0x20,2);ST8(ground_pouch_item+0x25,1);strcpy((char *)(uintptr_t)LD64(ground_pouch_item+0x30),"Item_Enemy_00");
    grant_before=inventory_grants;ground_tick(&cpu,owner,0x7000001000);assert(inventory_grants==grant_before+1&&ground_receipts[0].result==1);
    ground_tick(&cpu,owner,0x7000001000);assert(inventory_grants==grant_before+1);
    uint64_t second_loot=HEAP_BASE+0xfa000;ST64(second_loot+0x10,second_loot+0x800);strcpy((char *)(uintptr_t)(second_loot+0x800),"Item_Enemy_01");ST8(second_loot+0x64,1);ST64(second_loot+0x68,second_loot+0x1000);ST32(second_loot+0x60,9);ST32(second_loot+0x1040,9);ST64(second_loot+0x1048,second_loot);memcpy((void *)(uintptr_t)(second_loot+0x398),player_root,48);
    Cpu second_job=job;second_job.x[22]=second_loot;rt_hyrulem_capture_birth(&second_job);assert(loot_captured==2&&native_drops[1].source.parent.ordinal==2);
    // An original outside GroundLocal[32] must still suppress duplicate
    // reconstruction and be deleted after a terminal server state.
    char second_local[32];snprintf(second_local,sizeof second_local,"%llu",(unsigned long long)native_drops[1].source.local);char *second_state[28];memcpy(second_state,loot_state,sizeof second_state);second_state[3]="2";second_state[4]="Item_Enemy_01";second_state[12]=second_local;second_state[14]="0";second_state[15]="0";second_state[27]="2";
    ground_update(28,second_state,&r);unsigned before_native_match=creates;ground_tick(&cpu,owner,0x7000001000);assert(creates==before_native_match&&native_drops[1].enrolled);
    unsigned terminal_deletes=deletes;second_state[14]="2";ground_update(28,second_state,&r);ground_tick(&cpu,owner,0x7000001000);assert(!native_drops[1].active&&deletes==terminal_deletes+1&&LD8(second_loot+0x64)==3);
    // Discard acknowledgements enqueue only; a worker/control callback must not delete.
    ST8(second_loot+0x64,1);ST32(second_loot+0x60,11);ST32(second_loot+0x1040,11);rt_hyrulem_capture_birth(&second_job);assert(native_drops[1].active&&native_drops[1].source.parent.ordinal==3);
    snprintf(loot_local,sizeof loot_local,"%llu",(unsigned long long)native_drops[1].source.local);char *discard[]={"hm_world_drop_discard",desired.session,ground_mode.world,loot_local};calls_before_birth=calls;unsigned discarded_before=deletes;
    ground_world_discard(4,discard,&r);assert(!r.failed&&native_drops[1].discard&&calls==calls_before_birth);
    GroundState no_loot[GROUND_MAX]={0};ground_native_tick(&cpu,0x7000001000,no_loot);assert(!native_drops[1].active&&deletes==discarded_before+1&&LD8(second_loot+0x64)==3);
    // The original native drop hook also captures a dead enemy's weapon,
    // preserving its condition without debiting either player's inventory.
    ST32(MAIN_BASE+0x120,0x91353000);ST32(MAIN_BASE+0x124,0xd65f03c0);
    ST8(weapon+0x64,1);ST32(weapon+0xd4c,900);ST32(weak+0x40,9);ST64(weapon+0x938,birth_weak+0x100);ST32(weapon+0x940,6);
    calls_before_birth=calls;rt_hyrulem_capture_drop(&drop_cpu);bool enemy_weapon_seen=false;for(unsigned i=0;i<NATIVE_DROPS;i++)if(native_drops[i].active&&!strcmp(native_drops[i].source.name,"Weapon_Sword_001")&&native_drops[i].source.parent.hash==123&&native_drops[i].source.value==900)enemy_weapon_seen=true;
    assert(enemy_weapon_seen&&calls==calls_before_birth);
    // A factory-derived trunk keeps its own network identity and can recover
    // native simulation after reload, only when the server explicitly owns it.
    ST8(second_loot+0x64,2);ST64(second_loot+0x4e0,second_loot+0xa00);ST64(second_loot,HEAP_BASE+0xfbc00);ST32(second_loot+0x900,40);
    strcpy((char *)(uintptr_t)(second_loot+0x800),"Obj_TreeBroadleaf_A_L_Trunk");ST32(second_loot+0x60,10);ST32(second_loot+0x1040,10);
    strcpy(loot_parent.name,"Obj_TreeBroadleaf_A_L");loot_parent_note(parent,&loot_parent);loot_bind_trunk(second_loot,parent,"Obj_TreeBroadleaf_A_L_Trunk");
    WorldActor trunk_state={0};assert(actor_world_read(second_loot,&trunk_state)&&trunk_state.hash==123&&!trunk_state.replica&&actor_world_is_trunk(&trunk_state));
    WorldReplica *trunk_copy=&actor_world_replicas[0];trunk_copy->identity=trunk_state;trunk_copy->identity.id=701;trunk_copy->health_ready=true;trunk_copy->visual.storage=ground_storage();trunk_copy->visual.actor=second_loot;trunk_copy->visual.active=true;trunk_copy->last_target=now_ms()-1600;
    strcpy(trunk_copy->session,ground_mode.session);strcpy(trunk_copy->world,ground_mode.world);strcpy(trunk_copy->scene,local_scene);call(&cpu,g_site.actor_index_assign,trunk_copy->visual.storage+INDEX,second_loot,1,0,0,0,0);
    ActorWorldMode trunk_mode=actor_world_mode;trunk_mode.owned_count=1;trunk_mode.owned[0]=701;for(unsigned j=0;j<3;j++)trunk_mode.center[j]=player_root[j*4+3];actor_world_mode=actor_world_live=trunk_mode;
    unsigned before_trunk_delete=deletes;actor_world_replica_tick(&cpu,&trunk_mode,0x7000001000,false);assert(trunk_copy->visual.actor==second_loot&&deletes==before_trunk_delete);
    ST64(second_loot+0x558,second_loot+0xc00);ST64(second_loot+0xc08,second_loot);Cpu trunk_ai=cpu;trunk_ai.x[19]=second_loot;assert(!rt_hyrulem_world_ai(&trunk_ai));
    actor_world_mode.owned_count=0;assert(rt_hyrulem_world_ai(&trunk_ai));trunk_mode.owned_count=0;trunk_copy->last_target=now_ms()-1600;actor_world_replica_tick(&cpu,&trunk_mode,0x7000001000,false);assert(!trunk_copy->identity.id&&deletes==before_trunk_delete+1);
    char *trunk_lease[]={"hm_world_sample",desired.session,ground_mode.world,"901","3","1","0","3437.5","235.9","2101.5",empty_world,"MainField","701"};
    calls_before_birth=calls;r.failed=false;actor_world_sample(13,trunk_lease,&r);assert(!r.failed&&actor_world_mode.owned_count==1&&actor_world_mode.owned[0]==701&&calls==calls_before_birth);
    trunk_lease[12]="701,701";actor_world_sample(13,trunk_lease,&r);assert(r.failed&&actor_world_mode.owned_count==1&&calls==calls_before_birth);r.failed=false;
    // Enrollment survives removal from the 32-slot visual window. A stale
    // accepted source with no current visual ID can release its pool record.
    for(unsigned i=0;i<NATIVE_DROPS;i++)native_drops[i].active=false;
    native_drops[0]=(NativeDrop){.active=true,.enrolled=true,.actor=second_loot,.block=second_loot+0x1000,.generation=9};
    strcpy((char *)(uintptr_t)(second_loot+0x800),"Item_Material_04");
    ST8(second_loot+0x64,1);
    GroundSource reclaimed={0};strcpy(reclaimed.name,"Item_Material_04");memcpy(reclaimed.pos,loot_parent.spawn,12);memcpy(reclaimed.spawn,loot_parent.spawn,12);
    assert(loot_capture(second_loot,parent,&reclaimed)&&native_drops[0].active&&native_drops[0].generation==10&&!native_drops[1].active);
    // A detached asynchronous creation keeps copied ancestry after the weak
    // parent dies. Reused requests, nested jobs and resets cannot inherit it.
    uint64_t request_task=HEAP_BASE+0xf7400,request_desc=HEAP_BASE+0xf7000;
    // The request copier reads the actor's SafeString at +0/+8. The next
    // SafeString (+0x10/+0x18) is a separate creation parameter, often empty.
    ST64(request_desc+8,request_desc+0x200);strcpy((char *)(uintptr_t)(request_desc+0x200),"Item_Material_04");
    ST64(request_desc+0x18,request_desc+0x280);strcpy((char *)(uintptr_t)(request_desc+0x280),"");
    Cpu enqueue=cpu;enqueue.x[0]=request_task;enqueue.x[1]=request_desc;Cpu enqueue_before=enqueue;
    ST64(job.x[21]+0x48,0);ST32(parent+0x900,0);ST32(birth_weak+0x140,6);
    for(unsigned i=0;i<NATIVE_DROPS;i++)native_drops[i].active=false;
    uint64_t scope_token=rt_hyrulem_loot_job_begin(parent);calls_before_birth=calls;rt_hyrulem_loot_request(&enqueue);rt_hyrulem_loot_job_end(scope_token);
    assert(calls==calls_before_birth&&!memcmp(&enqueue,&enqueue_before,sizeof enqueue));
    ST32(birth_weak+0x140,7); // the native parent may disappear before its child initializes
    uint64_t factory_token;unsigned captured_before_async=loot_captured;AsyncBirthTest async={.task=request_task,.birth=job};async.birth.x[22]=second_loot;pthread_t birth_thread;
    assert(!pthread_create(&birth_thread,NULL,async_birth_test,&async)&&!pthread_join(birth_thread,NULL));
    assert(loot_captured==captured_before_async+1&&native_drops[0].source.parent.hash==123&&!strcmp(native_drops[0].source.parent.name,"Obj_TreeBroadleaf_A_L")&&calls==calls_before_birth);
    LootRequest request_copy;assert(!loot_factory_context(job.x[21],"Item_Material_04",&request_copy));
    ST32(birth_weak+0x140,6);scope_token=rt_hyrulem_loot_job_begin(parent);uint64_t nested=rt_hyrulem_loot_job_begin(0);
    rt_hyrulem_loot_request(&enqueue);rt_hyrulem_loot_job_end(nested);rt_hyrulem_loot_job_end(scope_token);
    factory_token=rt_hyrulem_loot_factory_begin(request_task,job.x[21]);assert(loot_factory_context(job.x[21],"Item_Material_04",&request_copy)&&!request_copy.valid&&!request_copy.creator_name[0]);
    assert(!loot_factory_context(job.x[21]+8,"Item_Material_04",&request_copy)&&!loot_factory_context(job.x[21],"Item_Fruit_A",&request_copy));rt_hyrulem_loot_factory_end(factory_token);
    scope_token=rt_hyrulem_loot_job_begin(parent);rt_hyrulem_loot_request(&enqueue);rt_hyrulem_loot_job_end(scope_token);
    rt_hyrulem_loot_request(&enqueue); // pool reuse outside any actor job erases the old link
    factory_token=rt_hyrulem_loot_factory_begin(request_task,job.x[21]);assert(loot_factory_context(job.x[21],"Item_Material_04",&request_copy)&&!request_copy.valid);rt_hyrulem_loot_factory_end(factory_token);
    scope_token=rt_hyrulem_loot_job_begin(parent);birth_reset();rt_hyrulem_loot_job_end(scope_token);assert(!loot_thread.jobs&&!loot_thread.factories);
    puts("world loot: exact parent alias, dead-parent gate, held pre-export pickup, one-credit grant and deferred duplicate disposal passed");
    ground_reset();birth_reset();actor_world_reset();
