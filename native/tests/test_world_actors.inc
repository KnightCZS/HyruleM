    // Host authority uses native map actors, not remote Link proxies. All data
    // here is synthetic; no constructor or renderer is claimed by this fixture.
    actor_world_reset();world_test=true;r.failed=false;strcpy(desired.session,"0123456789abcdef");
    uint64_t wa=HEAP_BASE+0x50000,wn=HEAP_BASE+0x52000,wm=wa+0x1000,nm=wn+0x1000,was=wa+0xa00,waroot=wa+0xe00;
    float host_root[12]={1,0,0,3430,0,1,0,236,0,0,1,2100};memcpy((void *)(uintptr_t)(owner+0x398),host_root,48);
    for(unsigned i=0;i<2;i++){uint64_t a=i?wn:wa,map=i?nm:wm;ST8(a+0x64,i?2:1);ST64(a+0x4e0,a+0x1700);ST64(a+0x10,a+0x900);strcpy((char *)(uintptr_t)(a+0x900),i?"Npc_Test":"Enemy_Bokoblin_Junior");ST64(a+0x7c8,map);ST32(map+0x58,100+i);for(unsigned j=0;j<3;j++){float f=host_root[3+j*4];memcpy((void *)(uintptr_t)(map+0x5c+j*4),&f,4);}memcpy((void *)(uintptr_t)(a+0x398),host_root,48);ST64(a+0x68,a+0x800);ST32(a+0x840,7);ST64(a+0x848,a);}
    ST64(wa+0x560,was);ST64(was+0xd8,wa);ST32(was+0xb8,1);ST64(was+0xc0,wa+0xc00);ST64(wa+0x558,waroot);ST64(waroot+8,wa);
    ST32(creator+0x4c,0x7b8);ST32(creator+0x48,2);ST64(head,wa+0x7b8);ST64(wa+0x7b8,wn+0x7b8);ST64(wn+0x7b8,head);
    uint64_t wr=wa+0x1500,wkey=wa+0x1600;ST64(wr+0x10,wkey);strcpy((char *)(uintptr_t)wkey,"Walk");ST32(wr+0x18,0);ST32(wr+0x1c,0);ST8(wr+0x21,1);Cpu world_cpu=before;world_cpu.x[0]=was;world_cpu.x[1]=wr;Cpu world_saved=world_cpu;uint64_t saved_world_info=LD64(g_site.player_info);ST64(g_site.player_info,0);rt_hyrulem_capture_as(&world_cpu);ST64(g_site.player_info,saved_world_info);assert(!memcmp(&world_cpu,&world_saved,sizeof world_cpu));
    char *wcmd[]={"hm_world_sample","0123456789abcdef","0123456789abcdef01234567","1","1","0","0","0","0","0","-"};unsigned world_calls=calls,world_creates=creates,world_deletes=deletes,world_grants=inventory_grants,world_debits=inventory_debits;actor_world_sample(11,wcmd,&r);assert(!r.failed&&calls==world_calls);
    WorldActor dormant={0};assert(actor_world_read(wn,&dormant));ST8(wn+0x64,0);assert(!actor_world_read(wn,&dormant));ST8(wn+0x64,3);assert(!actor_world_read(wn,&dormant));ST8(wn+0x64,2);ST64(wn+0x4e0,0);assert(!actor_world_read(wn,&dormant));ST64(wn+0x4e0,wn+0x1700);
    actor_world_tick(&cpu,owner,0x7000001000,false);assert(actor_world_seen_count==2&&actor_world_frame_seq==1&&actor_world_seen[0].tracks[0].sequence>0&&!memcmp(&cpu,&before,sizeof cpu));
    // A map identity can disappear between the list scan and temporary lease.
    // Drop that binding so a later capture cannot publish a zero source ID.
    invalidate_world_actor=wa;actor_world_scan_at=0;actor_world_tick(&cpu,owner,0x7000001000,false);assert(actor_world_seen_count==1&&!actor_world_complete);
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)assert(actor_world_bindings[i].actor!=wa);
    ST32(wm+0x58,100);actor_world_scan_at=0;actor_world_tick(&cpu,owner,0x7000001000,false);assert(actor_world_seen_count==2);for(unsigned i=0;i<2;i++)assert(actor_world_seen[i].id);
    world_cpu=before;world_cpu.x[0]=waroot;world_cpu.x[19]=wa;world_saved=world_cpu;assert(!rt_hyrulem_world_ai(&world_cpu));
    // Move the source snapshot; receiving it queues only. No actor is touched
    // until the joined-jobs callback gets a native local reference.
    ActorWorldMode decoded_world={0};decoded_world.center[0]=3430;decoded_world.center[1]=236;decoded_world.center[2]=2100;
    WorldActor rows[2];memcpy(rows,actor_world_seen,sizeof rows);for(unsigned i=0;i<2;i++)rows[i].matrix[3]+=10;
    char world_encoded[ACTOR_WORLD_B64+1];actor_world_encode(rows,2,world_encoded);assert(actor_world_decode(world_encoded,&decoded_world)&&decoded_world.count==2);
    wcmd[3]="2";wcmd[4]="2";wcmd[5]="1";wcmd[7]="3430";wcmd[8]="236";wcmd[9]="2100";wcmd[10]=world_encoded;unsigned world_moves=moves;actor_world_sample(11,wcmd,&r);assert(!r.failed&&moves==world_moves);
    actor_world_tick(&cpu,owner,0x7000001000,false);assert(actor_world_matched==2&&actor_world_missing==0&&moves==world_moves+4&&actor_world_as_applied==1);
    assert(rt_hyrulem_world_ai(&world_cpu)&&!memcmp(&world_cpu,&world_saved,sizeof world_cpu));
    for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].actor)actor_world_bindings[i].started=now_ms()-300;
    actor_world_tick(&cpu,owner,0x7000001000,false);assert(fabsf(read_float(wa+0x3a4)-3440)<.001f&&read_float(owner+0x3a4)==3430&&!memcmp(&cpu,&before,sizeof cpu));
    ST32(wa+0x840,8);assert(!rt_hyrulem_world_ai(&world_cpu));ST32(wa+0x840,7);
    // An actor absent from the follower's loaded list is counted, not created
    // through an unsafe guessed NPC constructor or interpreted as destroyed.
    ST64(wa+0x7b8,head);ST32(creator+0x48,1);actor_world_scan_at=0;actor_world_tick(&cpu,owner,0x7000001000,false);assert(actor_world_matched==1&&actor_world_missing==1&&creates==world_creates&&deletes==world_deletes);
    // A denied lease must release the temporary link and cannot keep an actor
    // indefinitely paused from an old successful write.
    deny_lease=true;for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].actor)actor_world_bindings[i].applied_at=now_ms()-501;
    world_moves=moves;actor_world_tick(&cpu,owner,0x7000001000,false);assert(moves==world_moves&&!rt_hyrulem_world_ai(&world_cpu));for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)if(actor_world_bindings[i].storage)assert(!LD64(actor_world_bindings[i].storage+LOCAL+8));deny_lease=false;
    ActorWorldMode intact_world=actor_world_mode;wcmd[3]="1";r.failed=false;actor_world_sample(11,wcmd,&r);assert(r.failed&&!memcmp(&actor_world_mode,&intact_world,sizeof intact_world));
    // Timeout/close restores native authority by releasing references only.
    actor_world_mode.heartbeat=now_ms()-1600;actor_world_tick(&cpu,owner,0x7000001000,false);assert(!rt_hyrulem_world_ai(&world_cpu));for(unsigned i=0;i<ACTOR_WORLD_MAX;i++)assert(!actor_world_bindings[i].actor);assert(creates==world_creates&&deletes==world_deletes&&inventory_grants==world_grants&&inventory_debits==world_debits);
    // Rotation interpolation must not collapse a 180-degree rotation halfway.
    float spin_from[12]={1,0,0,0,0,1,0,0,0,0,1,0},spin_to[12]={-1,0,0,10,0,1,0,0,0,0,-1,0},spin_mid[12];
    for(unsigned rate=30;rate<=120;rate+=30)for(unsigned tick=0;tick<=rate;tick++){actor_world_interpolate(spin_from,spin_to,(float)tick/rate,spin_mid);assert(actor_world_matrix(spin_mid));for(unsigned row=0;row<3;row++){float norm=0;for(unsigned col=0;col<3;col++)norm+=spin_mid[row*4+col]*spin_mid[row*4+col];assert(fabsf(norm-1)<.001f);}}
    // An equipped weapon is never reclassified as a loose map actor.
    strcpy((char *)(uintptr_t)(wn+0x900),"Weapon_Sword_001");ST64(wn+0x938,wa+0x800);ST32(wn+0x940,7);WorldActor loose={0};assert(!actor_world_read(wn,&loose));ST64(wn+0x938,0);assert(actor_world_read(wn,&loose));
    actor_world_reset();world_test=false;ST64(head,head);ST32(creator+0x48,0);
    puts("world actors: host capture, map identity, native follower transforms/AI gate, AS, missing actors, lease/epoch/timeout isolation and 30/60/90/120 Hz rotation passed");
