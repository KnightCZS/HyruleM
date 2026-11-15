    // The source body's hidden skin/hair shapes must remain hidden remotely.
    // All names/layout data and property conditions here are synthetic.
    ST64(model_vt+0x1b0,g_site.hm_material_set);ST64(model_vt+0x88,g_site.hm_bone_visible_set);ST16(model+0x112,2);
    uint64_t shape_res=HEAP_BASE+0xd1000,shape_dict=shape_res+0x100,shape_bits=shape_res+0x200;
    ST64(model+0xe0,shape_res);ST64(shape_res+0x48,shape_dict);ST64(model+0xf0,shape_bits);
    ST64(shape_dict+0x20,shape_res+0x300);ST64(shape_dict+0x30,shape_res+0x400);strcpy((char *)(uintptr_t)(shape_res+0x302),"Skin_Upper");strcpy((char *)(uintptr_t)(shape_res+0x402),"Face");ST32(shape_bits,2);
    Appearance shape_frame={0};visibility_capture(owner,0,&shape_frame);assert(shape_frame.visibility==2&&shape_frame.mask[0].visible==0&&shape_frame.mask[1].visible==1);
    ST32(shape_bits,3);visibility_apply(&cpu,owner,&shape_frame,0);assert(LD32(shape_bits)==2&&shape_writes==1&&!memcmp(&cpu,&before,sizeof cpu));
    shape_frame.mask[0].part=4;ST32(shape_bits,3);visibility_apply(&cpu,owner,&shape_frame,0);assert(LD32(shape_bits)==3);shape_frame.mask[0].part=0;
    uint64_t bone_bits=shape_res+0x800;ST64(model+0xe8,bone_bits);ST32(bone_bits,2);Appearance bone_visibility={0};visibility_capture(owner,0,&bone_visibility);assert(bone_visibility.visibility==4&&bone_visibility.mask[2].kind==1);ST32(bone_bits,3);visibility_apply(&cpu,owner,&bone_visibility,0);assert(LD32(bone_bits)==2&&!memcmp(&cpu,&before,sizeof cpu));ST64(model+0xe8,0);
    // Capture numeric effect conditions; receiver definition order may differ.
    uint64_t pr=HEAP_BASE+0xd5000,pdefs=pr+0x100,pvalues=pr+0x200,pd1=pr+0x300,pd2=pr+0x400;
    ST64(xu+0x30,pr);ST16(pr+0x44,2);ST64(pr+0x48,pdefs);ST64(xu+0x70,pvalues);ST64(pdefs,pd1);ST64(pdefs+8,pd2);
    ST64(pd1+0x10,pr+0x500);ST64(pd2+0x10,pr+0x600);strcpy((char *)(uintptr_t)(pr+0x500),"WeaponKind");strcpy((char *)(uintptr_t)(pr+0x600),"AttackPower");ST32(pd1+0x60,0);ST32(pd2+0x60,2);ST32(pvalues,3);ST32(pvalues+4,0x3f400000);
    fx_properties_capture(owners,&shape_frame);assert(shape_frame.properties==2&&shape_frame.property[0].value==3);
    ST32(pvalues,0);ST32(pvalues+4,0);ST64(pdefs,pd2);ST64(pdefs+8,pd1);fx_properties_apply(&cpu,owner,&shape_frame,0,true);
    assert(LD32(pvalues)==0x3f400000&&LD32(pvalues+4)==3&&LD64(xu+0x68)==3&&property_writes==2&&!memcmp(&cpu,&before,sizeof cpu));
    char app_encoded[APPEARANCE_B64_MAX+1];Appearance app_decoded;appearance_encode(&shape_frame,app_encoded);assert(appearance_decode(app_encoded,&app_decoded)&&app_decoded.visibility==2&&app_decoded.properties==2);
    shape_frame.mask[1].name=shape_frame.mask[0].name;appearance_encode(&shape_frame,app_encoded);assert(!appearance_decode(app_encoded,&app_decoded));shape_frame.mask[1].name=pose_hash("Face");
    shape_frame.property[1].value=0x7fc00000;appearance_encode(&shape_frame,app_encoded);assert(!appearance_decode(app_encoded,&app_decoded));shape_frame.property[1].value=0x3f400000;
    appearance_encode(&shape_frame,app_encoded);char *appearance_args[97]={0};appearance_args[95]="0";appearance_args[96]=app_encoded;FxFrame frame14={0};assert(fx_decode(97,appearance_args,&frame14,&app_decoded)&&app_decoded.visibility==2);assert(!fx_decode(98,appearance_args,&frame14,&app_decoded));
    // Direct descriptor requests do not require a caller output handle. The
    // common initialized-Event hook must still capture them, once, locally.
    fx_reset();fx_owners_update(owners);uint64_t direct=HEAP_BASE+0xda000,definition=direct+0x100;
    ST64(direct+0x28,xu);ST64(direct+0x30,definition);ST64(definition,xk);ST32(direct+0x20,19);ST32(direct+8,0);
    fx_cpu=before;fx_cpu.x[19]=0;fx_cpu.x[20]=xu;fx_cpu.x[21]=0;fx_cpu.x[23]=direct;fx_saved=fx_cpu;captured_before=published.fx_captured;
    rt_hyrulem_created_effect(&fx_cpu);rt_hyrulem_created_effect(&fx_cpu);assert(published.fx_captured==captured_before+1&&!memcmp(&fx_cpu,&fx_saved,sizeof fx_cpu));
    fx_cpu.x[20]=xu+0x100;rt_hyrulem_created_effect(&fx_cpu);assert(published.fx_captured==captured_before+1);
    ST16(model+0x112,0);ST64(owner+0x23f0,0);fx_reset();
    puts("preview 14: visibility, named effect conditions, reordered properties, direct descriptors, validation and CPU preservation passed");
