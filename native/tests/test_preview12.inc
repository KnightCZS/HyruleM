    // Body and equipped armor may have equally named bones, but their final
    // cloth palettes must not overwrite one another. All fixtures are synthetic.
    float restored=1;memcpy((void *)(uintptr_t)palette,&restored,4);
    uint64_t coat=HEAP_BASE+0xb0000,cc=coat+0x1000,cm=coat+0x1400,cr=coat+0x1a00,cp=coat+0x1c00;
    memcpy((void *)(uintptr_t)cc,(void *)(uintptr_t)component,0x1000);
    ST64(coat+0x4e0,cc);ST64(coat+0x80,owner);ST8(coat+0x64,1);ST64(coat+0x10,coat+0x800);strcpy((char *)(uintptr_t)(coat+0x800),"Armor_001_Head");
    ST64(cc+0x40,cc+0x100);ST64(cc+0x100,cc+0x200);ST64(cc+0x200,cm);ST64(cm+0x118,cc+0x900);ST64(cc+0x900,cr);ST64(cc+0x920,cp);ST16(cr+0x3c,3);ST64(cr+0x10,cc+0xb00);
    ST64(cc+0xb20,cc+0xd00);ST64(cc+0xb30,cc+0xd50);ST64(cc+0xb40,cc+0xda0);strcpy((char *)(uintptr_t)(cc+0xda2),"Hair_1");
    for(int i=0;i<3;i++){float m[16]={1,0,0,0,0,1,0,0,0,0,1,0,100+i*3,200,300,0};memcpy((void *)(uintptr_t)(cp+i*64),m,64);}
    snprintf(local_armor[0],48,"Armor_001_Head");ST32(creator+0x4c,0x7b8);ST32(creator+0x48,1);ST64(head,coat+0x7b8);ST64(coat+0x7b8,head);
    pose_capture(owner,origin,0,&captured);assert(captured.count==2);
    pose_capture_attached(&cpu,owner,origin,0,&captured);assert(captured.count==4&&pose_probe.parts[0]==2&&captured.bones[2].part==4&&captured.bones[3].name==pose_hash("Hair_1"));assert(!memcmp(&cpu,&before,sizeof cpu));
    pose_encode(&captured,encoded);assert(pose_decode(encoded,&decoded)&&decoded.count==4&&decoded.bones[2].part==4);
    skin.pose=captured;before_bones=bone_writes;assert(pose_apply(&cpu,owner,storage,&skin,0)==2&&bone_writes==before_bones+2);
    assert(pose_apply(&cpu,coat,storage,&skin,4)==3);
    // A matching piece owned by a different actor must not enter our capture.
    ST64(coat+0x80,mock_actor);pose_capture(owner,origin,0,&captured);pose_capture_attached(&cpu,owner,origin,0,&captured);assert(captured.count==2);
    ST64(coat+0x80,owner);float bad_piece=NAN;memcpy((void *)(uintptr_t)cp,&bad_piece,4);pose_capture(owner,origin,0,&captured);pose_capture_attached(&cpu,owner,origin,0,&captured);assert(captured.count==2&&pose_probe.parts[0]==0&&!strcmp(pose_probe.reason,"piece-pose-unavailable"));
    ST64(head,head);ST32(creator+0x48,0);
    // No second smoothing stage for already interpolated named-bone frames.
    puts("preview 12: separate cloth palettes, parent isolation and piece-specific replay passed");
    // Capture the actual output-handle convention, isolate local ELink users,
    // and replay starts/stops once with the native generation at Event+0x20.
    fx_reset();uint64_t xo=HEAP_BASE+0xc1000,xb=HEAP_BASE+0xc2000,xu=HEAP_BASE+0xc3000,xh=HEAP_BASE+0xc4000,xc=HEAP_BASE+0xc5000,xk=HEAP_BASE+0xc6000;
    ST64(owner+0x68,xb);ST64(xb+0x48,owner);ST32(xb+0x40,7);ST64(owner+0x568,xo);ST64(xo+8,owner);ST64(xo+0x50,xu);uint64_t owners[4]={owner,0,0,0};fx_owners_update(owners);
    ST64(xh,xc);ST32(xh+8,9);ST32(xc+0x20,9);strcpy((char *)(uintptr_t)xk,"AttackSpin");
    Cpu fx_cpu=before;fx_cpu.x[19]=xh;fx_cpu.x[20]=xu;fx_cpu.x[21]=xk;Cpu fx_saved=fx_cpu;unsigned captured_before=published.fx_captured;
    rt_hyrulem_capture_effect(&fx_cpu);rt_hyrulem_capture_effect(&fx_cpu);assert(published.fx_captured==captured_before+1&&!memcmp(&fx_cpu,&fx_saved,sizeof fx_cpu));
    fx_cpu.x[20]=xu+0x100;ST32(xh+8,10);ST32(xc+0x20,10);rt_hyrulem_capture_effect(&fx_cpu);assert(published.fx_captured==captured_before+1);ST32(xh+8,9);ST32(xc+0x20,9);
    fx_cpu.x[0]=xc;rt_hyrulem_stop_effect(&fx_cpu);assert(!fx_sources[0].event.active);char fx_text[4096];fx_json(fx_text,sizeof fx_text);assert(strstr(fx_text,"AttackSpin")&&strstr(fx_text,"\"active\":false"));
    storage=HLE_HEAP;actor=mock_actor;deny_lease=false;ST64(storage+INDEX,retained);ST32(storage+INDEX+8,9);ST8(storage+INDEX+12,1);ST32(retained+0x40,9);ST64(retained+0x48,actor);ST8(actor+0x64,1);
    uint64_t rx=HEAP_BASE+0xc7000;effect_user_expected=HEAP_BASE+0xc8000;ST64(actor+0x568,rx);ST64(rx+8,actor);ST64(rx+0x50,effect_user_expected);
    Desired effects={0};effects.effects.count=1;effects.effects.events[0]=(FxEvent){.sequence=1,.slot=0,.active=true,.age=10};strcpy(effects.effects.events[0].name,"AttackSpin");
    fx_tick(&cpu,&effects,0);fx_tick(&cpu,&effects,0);assert(effect_emits==1&&effect_stops==0&&!memcmp(&cpu,&before,sizeof cpu));
    effects.effects.events[0].active=false;fx_tick(&cpu,&effects,0);fx_tick(&cpu,&effects,0);assert(effect_stops==1&&effect_emits==1);
    effects.effects.events[0].sequence=2;effects.effects.events[0].active=true;fx_tick(&cpu,&effects,0);assert(effect_emits==2);ST32(HEAP_BASE+0xe0100+0x20,56);effects.effects.count=0;fx_tick(&cpu,&effects,0);assert(effect_stops==1); // recycled native handle cannot be stopped
    for(unsigned seq=3;seq<35;seq++){effects.effects.count=1;effects.effects.events[0].sequence=seq;fx_tick(&cpu,&effects,0);effects.effects.count=0;fx_tick(&cpu,&effects,0);}
    unsigned no_replay=effect_emits;effects.effects.count=1;effects.effects.events[0].sequence=1;fx_tick(&cpu,&effects,0);assert(effect_emits==no_replay);fx_clear(&cpu);
    puts("preview 12: native visual events, CPU preservation, source isolation, lifecycle, generation and replay checks passed");
