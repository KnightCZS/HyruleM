// Included inside the synthetic adapter harness, using its guest callees.
    ground_reset();r.failed=false;ground_open(6,material_open,&r);assert(!r.failed);
    ST64(head,head);ST32(creator+0x48,0);ground_tick(&cpu,owner,0x7000001000);
    weapon_test=true;weapon_vt=HEAP_BASE+0xb8000;weapon_pouch=HEAP_BASE+0xb0000;weapon_entries=1;
    ST64(weapon_pouch+0x30,weapon_pouch+0x80);strcpy((char *)(uintptr_t)(weapon_pouch+0x80),"Weapon_Sword_001");ST32(weapon_pouch+0x20,55);ST8(weapon_pouch+0x25,1);
    ST32(MAIN_BASE+0x120,0x91353000);ST32(MAIN_BASE+0x124,0xd65f03c0); // synthetic add x0,x0,#0xd4c; ret
    ST64(weapon_vt+0x2b8,MAIN_BASE+0x120);ST64(weapon_vt+0x588,g_site.hm_weapon_drop);
    uint64_t weapon=HEAP_BASE+0x80000,weak=weapon+0x1800,ownership=weapon+0x1900;
    ST64(weapon,weapon_vt);ST64(weapon+0x10,weapon+0x1600);strcpy((char *)(uintptr_t)(weapon+0x1600),"Weapon_Sword_001");ST8(weapon+0x64,1);
    ST64(weapon+0x4e0,weapon+0x1700);memcpy((void *)(uintptr_t)(weapon+0x1758),player_root,sizeof player_root);memcpy((void *)(uintptr_t)(weapon+0x398),player_root,sizeof player_root);
    ST32(weapon+0xd4c,900);ST32(weapon+0xf98,8);ST32(weapon+0xf9c,7);
    ST32(weapon+0x60,9);ST64(weapon+0x68,weak);ST32(weak+0x40,9);ST64(weak+0x48,weapon);
    ST64(weapon+0x938,ownership);ST32(weapon+0x940,3);ST32(ownership+0x40,3);ST64(ownership+0x48,owner);
    Cpu drop_cpu=cpu;drop_cpu.x[0]=weapon;Cpu drop_before=drop_cpu;calls_before=calls;
    strcpy(ground_mode.scene,"MainField");
    rt_hyrulem_capture_drop(&drop_cpu);assert(!strcmp(native_drops[0].source.scene,"MainField"));ground_mode.scene[0]=0;
    assert(native_drops[0].active&&native_drop_sequence==1&&calls==calls_before&&!memcmp(&drop_cpu,&drop_before,sizeof drop_cpu));
    ST64(accessor+8,weapon);ST64(safe+8,weapon+0x1600);
    assert(!rt_hyrulem_item_pickup(&pickup)&&!native_drops[0].active); // before export remains wholly local
    ground_status(0,NULL,&r);assert(!native_drops[0].exported);
    rt_hyrulem_capture_drop(&drop_cpu);ground_status(0,NULL,&r);assert(native_drops[0].exported);
    assert(rt_hyrulem_item_pickup(&pickup)&&ground_events[0].request==1&&native_drops[0].pickup_pending);
    assert(rt_hyrulem_item_pickup(&pickup)&&ground_next_request==1&&calls==calls_before);
    ST32(weak+0x40,10); // original pickup may delete the source before the next actor scan
    char weapon_local[32];snprintf(weapon_local,sizeof weapon_local,"%llu",(unsigned long long)native_drops[0].source.local);
    char *weapon_state[]={"hm_item_state","0123456789abcdef","0123456789abcdef01234567","1","Weapon_Sword_001","0","3437.5","235.9","2101.5","3437.5","235.9","2101.5",weapon_local,"1","1","1","1",ground_origin,"900","8","7","1"};
    ground_update(22,weapon_state,&r);assert(!r.failed);grant_before=inventory_grants;debit_before=inventory_debits;
    ground_tick(&cpu,owner,0x7000001000);assert(inventory_grants==grant_before+1&&inventory_debits==debit_before&&weapon_entries==2&&granted_modifier==8&&granted_bonus==7&&ground_receipts[0].result==1);
    ground_tick(&cpu,owner,0x7000001000);assert(inventory_grants==grant_before+1&&LD32(weapon_pouch+0x120)==900);
    GroundState weapon_grant=ground_state[0];inventory_full=true;assert(ground_grant(&cpu,&weapon_grant,0x7000001000,ground_replica[0].visual.storage)==-1);inventory_full=false;
    // A refused, already deferred pickup recovers ONE physical original,
    // before ordinary inventory pickup is allowed. No refund/grant is issued.
    ground_reset();ground_open(6,material_open,&r);ground_tick(&cpu,owner,0x7000001000);ST32(weak+0x40,9);
    rt_hyrulem_capture_drop(&drop_cpu);ground_status(0,NULL,&r);assert(rt_hyrulem_item_pickup(&pickup));
    ST32(weak+0x40,10);snprintf(weapon_local,sizeof weapon_local,"%llu",(unsigned long long)native_drops[0].source.local);
    char *local_ack[]={"hm_native_drop_local","0123456789abcdef","0123456789abcdef01234567",weapon_local};ground_native_local(4,local_ack,&r);
    unsigned before_create=creates;grant_before=inventory_grants;
    ground_tick(&cpu,owner,0x7000001000);assert(native_drops[0].fallback.spawning&&creates==before_create+1);
    uint64_t recovered=ground_pending_actor(native_drops[0].fallback.storage);ST64(accessor+8,recovered);ST64(safe+8,recovered+0xa00);
    assert(rt_hyrulem_item_pickup(&pickup)); // cannot pick up the uninitialized async actor
    ground_tick(&cpu,owner,0x7000001000);assert(!native_drops[0].active&&LD32(recovered+0xd4c)==900&&LD32(recovered+0xf98)==8&&LD32(recovered+0xf9c)==7);
    ground_native_local(4,local_ack,&r);ground_tick(&cpu,owner,0x7000001000);assert(creates==before_create+1&&inventory_grants==grant_before);
    // If the physical original survives, an ordinary local pickup retires
    // recovery bookkeeping before deletion, preventing a duplicate fallback.
    ST32(weak+0x40,9);rt_hyrulem_capture_drop(&drop_cpu);ground_status(0,NULL,&r);ST64(accessor+8,weapon);ST64(safe+8,weapon+0x1600);assert(rt_hyrulem_item_pickup(&pickup));
    snprintf(weapon_local,sizeof weapon_local,"%llu",(unsigned long long)native_drops[0].source.local);ground_native_local(4,local_ack,&r);assert(!rt_hyrulem_item_pickup(&pickup)&&!native_drops[0].active);ST32(weak+0x40,10);
    ground_tick(&cpu,owner,0x7000001000);assert(creates==before_create+1);
    // Genuine life zero is a break; unloading or mere absence is not.
    ST32(weak+0x40,9);rt_hyrulem_capture_drop(&drop_cpu);ground_status(0,NULL,&r);native_drops[0].id=2;
    ground_capture_destroy(&drop_cpu);assert(!ground_destroyed_count);ST32(weapon+0xd4c,0);ground_capture_destroy(&drop_cpu);assert(ground_destroyed_count==1&&ground_destroyed[0]==2);
    // A failed asynchronous recovery is cleaned before retry, bounded to
    // three creations, and never turns into an inventory refund.
    ground_reset();ground_open(6,material_open,&r);ground_tick(&cpu,owner,0x7000001000);
    native_drops[0].active=native_drops[0].local_only=native_drops[0].pickup_pending=true;native_drops[0].actor=weapon;native_drops[0].block=weak;native_drops[0].generation=9;native_drops[0].source=weapon_grant.source;ST32(weak+0x40,10);
    fail_handle=true;before_create=creates;grant_before=inventory_grants;
    for(int j=0;j<12;j++){if(native_drops[0].fallback.failed)native_drops[0].fallback.started=now_ms()-1001;ground_tick(&cpu,owner,0x7000001000);}
    assert(creates==before_create+3&&native_drops[0].active&&native_drops[0].recovery_failed&&!native_drops[0].fallback.spawning&&inventory_grants==grant_before);fail_handle=false;
    weapon_test=false;
    // Local loss accumulates; applying a server life value does not become
    // another local hit. The deletion hook captures zero before weak release.
    world_history_count=world_zero_count=health_count=0;world_history_reset=false;last_world_scan=0;
    strcpy((char *)(uintptr_t)(HEAP_BASE+0x9000),"Obj_TreeApple_A_01");ST32(map+0x58,77);ST32(enemy+0x900,100);memcpy((void *)(uintptr_t)(enemy+0x398),player_root,sizeof player_root);ST64(head,enemy+0x7b8);ST64(enemy+0x7b8,head);ST32(creator+0x48,1);
    desired.received=now_ms();world_scan(&cpu,owner,true);assert(world_count==1&&world_seen[0].damage==0);
    ST32(enemy+0x900,70);last_world_scan=0;world_scan(&cpu,owner,true);assert(world_seen[0].damage==30);
    char *tree_hp[]={"hm_enemy_health","0123456789abcdef","77","Obj_TreeApple_A_01","0","0","0","50"};r.failed=false;receive_health(8,tree_hp,&r);assert(!r.failed);last_world_scan=0;world_scan(&cpu,owner,true);assert(LD32(enemy+0x900)==50&&world_seen[0].damage==30);
    last_world_scan=0;world_scan(&cpu,owner,true);assert(world_seen[0].damage==30);
    Cpu zero_cpu=cpu;zero_cpu.x[0]=enemy;calls_before=calls;rt_hyrulem_capture_zero(&zero_cpu);assert(world_zero_count==0&&calls==calls_before);
    ST32(enemy+0x900,0);rt_hyrulem_capture_zero(&zero_cpu);assert(world_zero_count==1&&calls==calls_before);
    ST64(head,head);ST32(creator+0x48,0);last_world_scan=0;world_scan(&cpu,owner,true);assert(world_seen[0].health==0&&world_seen[0].damage==80);
    tree_hp[7]="0";receive_health(8,tree_hp,&r);assert(world_zero_count==0);assert(!memcmp(&cpu,&before,sizeof cpu));
    puts("native weapon drops: original ownership, exact condition/modifiers, pickup once, capacity recovery, destruction and no double debit passed");
    puts("map damage: local cumulative loss, server-write exclusion, true-zero capture and acknowledgement passed");
