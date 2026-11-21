    assert(tree_has_stump("Obj_TreeApple_A_L_01")&&tree_has_stump("Obj_TreeWhiteBirch_A_04")&&tree_has_stump("Obj_TreePalm_A_01"));
    assert(!tree_has_stump("Obj_TreePalm_A_01_Trunk")&&!tree_has_stump("Obj_TreeUnknown"));
    assert(map_damage_name("Barrel")&&map_damage_name("Kibako_Contain_01")&&!map_damage_name("GameROMPlayer"));
    // Native reconstruction is deferred, identity-preserving and never a map
    // actor impersonation: no synthetic pointer is installed in map metadata.
    actor_world_reset();world_test=true;ST64(head,head);ST32(creator+0x48,0);
    ActorWorldMode missing_mode={.role=3,.epoch=70,.sequence=1,.count=1,.heartbeat=now_ms()};
    strcpy(missing_mode.session,desired.session);strcpy(missing_mode.world,"0123456789abcdef01234567");
    for(unsigned j=0;j<3;j++)missing_mode.center[j]=read_float(owner+0x3a4+j*16);
    WorldActor *missing_actor=&missing_mode.actors[0];missing_actor->id=701;missing_actor->hash=101;missing_actor->kind=2;strcpy(missing_actor->name,"Npc_Test");
    memcpy(missing_actor->matrix,host_root,48);for(unsigned j=0;j<3;j++)missing_actor->spawn[j]=host_root[j*4+3];
    unsigned lifecycle_creates=creates,lifecycle_deletes=deletes;
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);
    WorldReplica *copy=&actor_world_replicas[0];assert(copy->identity.id==701&&!copy->visual.actor&&creates==lifecycle_creates);
    copy->since=now_ms()-1600;actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);
    assert(copy->visual.spawning&&creates==lifecycle_creates+1);
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);assert(copy->visual.active&&copy->visual.actor);
    uint64_t reconstructed=copy->visual.actor;WorldActor reconstructed_state={0};
    assert(!LD64(reconstructed+0x7c8)&&actor_world_read(reconstructed,&reconstructed_state)&&reconstructed_state.replica&&reconstructed_state.hash==101);
    ST64(head,reconstructed+0x7b8);ST64(reconstructed+0x7b8,head);ST32(creator+0x48,1);
    actor_world_mode=missing_mode;actor_world_tick(&cpu,owner,0x7000001000,false);
    assert(actor_world_seen_count==1&&actor_world_seen[0].replica&&actor_world_matched==1);
    // A genuine actor returning from streaming wins over the reconstruction.
    ST64(head,wn+0x7b8);ST64(wn+0x7b8,head);ST32(creator+0x48,1);actor_world_scan_at=0;
    actor_world_tick(&cpu,owner,0x7000001000,false);
    assert(!copy->identity.id&&deletes==lifecycle_deletes+1&&LD8(wn+0x64)==2);
    assert(actor_world_seen_count==1&&!actor_world_seen[0].replica&&!memcmp(&cpu,&before,sizeof cpu));
    // Repair is restricted to a currently foreign entity and deferred to the
    // joined-jobs callback; a control request cannot call the native engine.
    char *repair[]={"hm_world_rebuild",missing_mode.session,"701"};unsigned repair_calls=calls;
    r.failed=false;actor_world_rebuild(3,repair,&r);assert(!r.failed&&calls==repair_calls&&actor_world_rebuild_request.id==701);
    actor_world_tick(&cpu,owner,0x7000001000,false);assert(actor_world_rebuilds==1&&LD8(wn+0x64)==3);
    repair[2]="999";r.failed=false;repair_calls=calls;actor_world_rebuild(3,repair,&r);assert(r.failed&&calls==repair_calls);r.failed=false;
    // Enemies wait for canonical HP. Their constructor's default life cannot
    // become damage evidence or overwrite the server's known health.
    actor_world_reset();ST64(head,head);ST32(creator+0x48,0);missing_actor->id=702;missing_actor->hash=702;missing_actor->kind=1;strcpy(missing_actor->name,"Enemy_Bokoblin_Junior");
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);copy=&actor_world_replicas[0];copy->since=now_ms()-1600;
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);
    reconstructed=copy->visual.actor;assert(reconstructed&&!copy->health_ready&&!copy->visual.active);
    ST64(reconstructed,vt);ST32(reconstructed+0x900,7);memset(health_pending,0,sizeof health_pending);health_count=1;
    health_pending[0].hash=702;strcpy(health_pending[0].name,missing_actor->name);memcpy(health_pending[0].spawn,missing_actor->spawn,12);health_pending[0].health=13;
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);
    assert(copy->health_ready&&copy->visual.active&&LD32(reconstructed+0x900)==13);
    ST64(head,reconstructed+0x7b8);ST64(reconstructed+0x7b8,head);ST32(creator+0x48,1);world_history_count=world_zero_count=0;last_world_scan=0;
    world_scan(&cpu,owner,true);assert(world_count==1&&world_seen[0].hash==702&&world_seen[0].health==13&&world_seen[0].damage==0);
    ST32(reconstructed+0x900,3);last_world_scan=0;world_scan(&cpu,owner,true);
    assert(world_count==1&&world_seen[0].health==3&&world_seen[0].damage==10);
    ST32(reconstructed+0x900,0);desired.received=now_ms();lifecycle_deletes=deletes;
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);
    assert(!copy->visual.actor&&!LD8(copy->visual.storage+INDEX+12)&&deletes==lifecycle_deletes);
    last_world_scan=0;world_scan(&cpu,owner,true);assert(world_count==1&&world_seen[0].hash==702&&world_seen[0].health==0&&world_seen[0].damage==13);
    ActorWorldMode no_world={0};actor_world_replica_tick(&cpu,&no_world,0x7000001000,false);
    missing_actor->id=703;missing_actor->hash=703;missing_actor->kind=2;strcpy(missing_actor->name,"Npc_Test");
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);copy=&actor_world_replicas[0];copy->since=now_ms()-1600;
    actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);actor_world_replica_tick(&cpu,&missing_mode,0x7000001000,false);
    assert(copy->visual.active);
    // Closing preserves ownership if the native access check temporarily fails.
    deny_lease=true;lifecycle_deletes=deletes;
    actor_world_replica_tick(&cpu,&no_world,0x7000001000,false);assert(copy->visual.actor&&deletes==lifecycle_deletes);
    deny_lease=false;actor_world_replica_tick(&cpu,&no_world,0x7000001000,false);
    assert(!copy->identity.id&&deletes==lifecycle_deletes+1&&!memcmp(&cpu,&before,sizeof cpu));
    actor_world_reset();world_test=false;ST64(head,head);ST32(creator+0x48,0);health_count=0;
    puts("world lifecycle: missing NPC creation, alias identity, canonical HP, local damage, original preference, delayed cleanup and CPU preservation passed");
