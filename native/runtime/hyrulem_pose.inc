typedef struct {unsigned models,skipped,parts[3];const char *reason;} PoseProbe;
static PoseProbe pose_probe;
// Final skeletal poses. Layouts are independently verified for Switch 1.9.0.
// No Player graph or event is executed on behalf of another player's input.
static bool pose_supported(void) {return g_site.hm_bone_count&&g_site.hm_bone_name&&g_site.hm_bone_get&&g_site.hm_bone_set;}
static uint32_t pose_hash(const char *s){uint32_t crc=UINT32_MAX;for(;*s;s++){crc^=(uint8_t)*s;for(int i=0;i<8;i++)crc=(crc>>1)^(0xedb88320u&-(crc&1));}return ~crc;}
static uint32_t pose_u32(const unsigned char *p){return (uint32_t)p[0]|(uint32_t)p[1]<<8|(uint32_t)p[2]<<16|(uint32_t)p[3]<<24;}
static void pose_put32(unsigned char *p,uint32_t v){for(int i=0;i<4;i++)p[i]=(unsigned char)(v>>(i*8));}
static bool pose_matrix(const float m[12]){
    for(int i=0;i<12;i++)if(!isfinite(m[i])||fabsf(m[i])>(i%4==3?128.f:8.f))return false;
    for(int r=0;r<3;r++){float norm=0;for(int c=0;c<3;c++)norm+=m[r*4+c]*m[r*4+c];if(norm<.0001f)return false;}return true;
}
// The gsys virtual methods are compared with version-aware resolved sites
// before interpreting SDK storage or invoking its native world-matrix setter.
static bool pose_model(uint64_t m,uint64_t *res,uint64_t *palette,unsigned *count){
    if(!pose_supported()||!range(m,0x178))return false;uint64_t vt=LD64(m);
    if(!range(vt,0x78)||LD64(vt+0x30)!=g_site.hm_bone_count||LD64(vt+0x40)!=g_site.hm_bone_name||LD64(vt+0x68)!=g_site.hm_bone_set||LD64(vt+0x70)!=g_site.hm_bone_get)return false;
    uint64_t sk=LD64(m+0x118);if(!range(sk,0x28))return false;*res=LD64(sk);*palette=LD64(sk+0x20);
    if(!range(*res,0x40))return false;*count=LD16(*res+0x3c);
    return *count>0&&*count<=POSE_MAX&&range(*palette,(uint64_t)*count*64);
}
static uint32_t pose_name(uint64_t res,unsigned index){
    uint64_t dict=LD64(res+0x10);if(!range(dict,0x28+(uint64_t)index*16))return 0;
    uint64_t name=LD64(dict+0x20+(uint64_t)index*16);if(!range(name,66))return 0;
    const char *s=(const char *)(uintptr_t)(name+2);size_t n=strnlen(s,64);if(!n||n==64)return 0;
    for(size_t i=0;i<n;i++)if(!((s[i]>='a'&&s[i]<='z')||(s[i]>='A'&&s[i]<='Z')||(s[i]>='0'&&s[i]<='9')||s[i]=='_'))return 0;
    return pose_hash(s);
}
static bool pose_units(uint64_t a,uint64_t *units,unsigned *count){
    if(!range(a,0x4e8))return false;uint64_t comp=LD64(a+0x4e0);if(!range(comp,0x48))return false;
    *count=LD32(comp+0x38);*units=LD64(comp+0x40);return *count>0&&*count<=32&&range(*units,(uint64_t)*count*8);
}

/* Keep the final SDK palette on the received world root. Native physics can
 * regenerate bone matrices from its own pose after Actor/ModelGroup setMtx.
 * This changes visual placement only; it does not grant physics authority. */
static bool actor_world_delta(const float from[12],const float to[12],float out[12]){
    if(!actor_world_matrix(from)||!actor_world_matrix(to))return false;
    float inv[9]={
        from[5]*from[10]-from[6]*from[9],from[2]*from[9]-from[1]*from[10],from[1]*from[6]-from[2]*from[5],
        from[6]*from[8]-from[4]*from[10],from[0]*from[10]-from[2]*from[8],from[2]*from[4]-from[0]*from[6],
        from[4]*from[9]-from[5]*from[8],from[1]*from[8]-from[0]*from[9],from[0]*from[5]-from[1]*from[4]};
    float det=from[0]*inv[0]+from[1]*inv[3]+from[2]*inv[6];
    if(!isfinite(det)||fabsf(det)<.000001f)return false;
    for(unsigned i=0;i<9;i++)inv[i]/=det;
    for(unsigned r=0;r<3;r++){
        for(unsigned j=0;j<3;j++){out[r*4+j]=0;for(unsigned k=0;k<3;k++)out[r*4+j]+=to[r*4+k]*inv[k*3+j];}
        out[r*4+3]=to[r*4+3];for(unsigned k=0;k<3;k++)out[r*4+3]-=out[r*4+k]*from[k*4+3];
    }
    return actor_world_matrix(out);
}
static void actor_world_rebase(Cpu *c,uint64_t a,uint64_t scratch,const float target[12]){
    uint64_t units;unsigned n;if(!pose_units(a,&units,&n)||!pose_supported())return;
    // ponytail: reuse the local AS pose; exact world animation phase/parameters
    // need a later pose stream. Only validated SDK models and root names enter.
    float bones[POSE_MAX][12];
    for(unsigned unit=0;unit<n;unit++){
        uint64_t link=LD64(units+unit*8);if(!range(link,8))continue;uint64_t model=LD64(link),res,palette;unsigned count;
        if(!pose_model(model,&res,&palette,&count))continue;
        uint32_t root=pose_name(res,0);if(root!=pose_hash("Root")&&root!=pose_hash("Model"))continue;
        bool valid=true;for(unsigned i=0;i<count;i++){
            for(unsigned row=0;row<3;row++)for(unsigned col=0;col<4;col++)bones[i][row*4+col]=read_float(palette+(uint64_t)i*64+(col*4+row)*4);
            if(!actor_world_matrix(bones[i])){valid=false;break;}
        }
        float delta[12];if(!valid||!actor_world_delta(bones[0],target,delta))continue;
        float error=0;for(unsigned i=0;i<12;i++)error=fmaxf(error,fabsf(bones[0][i]-target[i]));if(error<.0001f)continue;
        // Validate every output before writing any bone of this model.
        for(unsigned i=0;i<count;i++){
            float m[12];for(unsigned row=0;row<3;row++)for(unsigned col=0;col<4;col++){
                m[row*4+col]=col==3?delta[row*4+3]:0;
                for(unsigned k=0;k<3;k++)m[row*4+col]+=delta[row*4+k]*bones[i][k*4+col];
            }
            if(!actor_world_matrix(m)){valid=false;break;}memcpy(bones[i],m,48);
        }
        if(!valid)continue;
        for(unsigned i=0;i<count;i++){memcpy((void *)(uintptr_t)(scratch+MATRIX),bones[i],48);call(c,g_site.hm_bone_set,model,scratch+MATRIX,i,0,0,0,0);}
    }
    // MATRIX also stores the previous network sample for interpolation.
    memcpy((void *)(uintptr_t)(scratch+MATRIX),target,48);
}

static void pose_capture_part(uint64_t a,const float xyz[3],float yaw,PoseFrame *out,unsigned part){
    uint64_t units;unsigned n;if(!pose_units(a,&units,&n))return;pose_probe.reason="sdk-model-unavailable";
    float s=sinf(yaw),co=cosf(yaw);
    for(unsigned unit=0;unit<n;unit++){
        uint64_t link=LD64(units+unit*8);if(!range(link,8))continue;uint64_t model=LD64(link),res,palette;unsigned count;
        if(!pose_model(model,&res,&palette,&count)){pose_probe.skipped++;continue;}pose_probe.models++;pose_probe.reason="bone-names-unavailable";
        for(unsigned i=0;i<count;i++){
            uint32_t name=pose_name(res,i);if(!name){pose_probe.skipped++;continue;}bool seen=false;for(unsigned j=0;j<out->count;j++)if(out->bones[j].name==name&&out->bones[j].part==part){seen=true;break;}if(seen)continue;
            PoseBone bone={0};bone.name=name;bone.part=part;float world[12];for(int row=0;row<3;row++)for(int col=0;col<4;col++)world[row*4+col]=read_float(palette+(uint64_t)i*64+(col*4+row)*4);
            for(int j=0;j<3;j++){bone.matrix[j]=co*world[j]-s*world[8+j];bone.matrix[4+j]=world[4+j];bone.matrix[8+j]=s*world[j]+co*world[8+j];}
            float dx=world[3]-xyz[0],dy=world[7]-xyz[1],dz=world[11]-xyz[2];bone.matrix[3]=co*dx-s*dz;bone.matrix[7]=dy;bone.matrix[11]=s*dx+co*dz;
            if(!pose_matrix(bone.matrix)){pose_probe.skipped++;pose_probe.reason="invalid-bone-matrix";out->count=0;return;}
            // Shared rigid bones can use the body's palette. Send the piece's
            // own final transform whenever it differs (especially cloth/hair).
            bool inherited=false;if(part)for(unsigned j=0;j<out->count;j++)if(!out->bones[j].part&&out->bones[j].name==name){inherited=true;for(int k=0;k<12;k++)if(fabsf(out->bones[j].matrix[k]-bone.matrix[k])>.00001f){inherited=false;break;}break;}
            if(inherited)continue;if(out->count>=POSE_MAX){pose_probe.reason="bone-limit-exceeded";out->count=0;return;}out->bones[out->count++]=bone;if(part>=4&&part<=6)pose_probe.parts[part-4]++;
        }
    }
    if(out->count)pose_probe.reason=pose_probe.skipped?"partial-pose":"captured";
}
static void pose_capture(uint64_t a,const float xyz[3],float yaw,PoseFrame *out){
    memset(out,0,sizeof *out);pose_probe=(PoseProbe){.reason="model-units-unavailable"};pose_capture_part(a,xyz,yaw,out,0);
}
// Player owns six retained armor references in its armor manager. They are
// not required to have Actor::parent set. Resolve with a temporary native local
// lease at the joined-jobs callback, then release even on denied access.
// The optional layout signature and exact main build gate these offsets.
static void visibility_capture(uint64_t,unsigned,Appearance *);
static void pose_capture_attached(Cpu *c,uint64_t local,const float xyz[3],float yaw,PoseFrame *out,Appearance *appearance){
    if(!out->count||!g_site.hm_armor_layout||!range(local,0x2450))return;
    if(!pose_storage)pose_storage=(uint64_t)(uintptr_t)hle_alloc(BYTES);
    if(!range(pose_storage,BYTES))return;unsigned refs=0;bool seen[3]={0};
    for(unsigned slot=0;slot<6;slot++){
        uint64_t index=local+0x23f0+slot*16,block=LD64(index);uint32_t generation=LD32(index+8);
        if(generation==UINT32_MAX||!range(block,0x50)||LD32(block+0x40)!=generation)continue;
        uint64_t expected=LD64(block+0x48);if(!range(expected,0x600)||expected==local)continue;
        uint64_t a=call(c,g_site.actor_index_resolve,index,pose_storage+LOCAL,0,0,0,0,0);
        if(a==expected&&LD64(pose_storage+LOCAL+8)==a&&LD8(a+0x64)==1){
            uint64_t name=LD64(a+0x10);
            if(range(name,48)&&memchr((void *)(uintptr_t)name,0,48))for(unsigned part=0;part<3;part++)if(!seen[part]&&gear_name((const char *)(uintptr_t)name,(int)part+3)){
                seen[part]=true;refs++;unsigned before=out->count;pose_capture_part(a,xyz,yaw,out,part+4);visibility_capture(a,part+4,appearance);
                if(!out->count){out->count=before;pose_probe.parts[part]=0;pose_probe.reason="piece-pose-unavailable";}break;
            }
        }
        call(c,g_site.actor_link_release,pose_storage+LOCAL,0,0,0,0,0,0);
    }
    pthread_mutex_lock(&mutex);published.armor_refs=refs;pthread_mutex_unlock(&mutex);
}
static unsigned pose_apply(Cpu *c,uint64_t a,uint64_t scratch,const Desired *d,unsigned part){
    if(!d->pose.count)return 0;uint64_t units;unsigned n,applied=0;if(!pose_units(a,&units,&n))return 0;
    float s=sinf(d->yaw),co=cosf(d->yaw);
    for(unsigned unit=0;unit<n;unit++){
        uint64_t link=LD64(units+unit*8);if(!range(link,8))continue;uint64_t model=LD64(link),res,palette;unsigned count;
        if(!pose_model(model,&res,&palette,&count))continue;
        for(unsigned i=0;i<count;i++){
            uint32_t name=pose_name(res,i);if(!name)continue;
            const PoseBone *bone=NULL;for(unsigned j=0;j<d->pose.count;j++)if(d->pose.bones[j].name==name){const PoseBone *candidate=&d->pose.bones[j];if(candidate->part==part){bone=candidate;break;}if(candidate->part==0)bone=candidate;}if(!bone)continue;
            float m[12];for(int j=0;j<4;j++){m[j]=co*bone->matrix[j]+s*bone->matrix[8+j];m[4+j]=bone->matrix[4+j];m[8+j]=-s*bone->matrix[j]+co*bone->matrix[8+j];}m[3]+=d->x;m[7]+=d->y;m[11]+=d->z;
            memcpy((void *)(uintptr_t)(scratch+MATRIX),m,sizeof m);call(c,g_site.hm_bone_set,model,scratch+MATRIX,i,0,0,0,0);applied++;
        }
    }
    return applied;
}
static const char pose_b64[]="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
static size_t pose_encode(const PoseFrame *frame,char *out){
    unsigned char raw[8+POSE_MAX*56];memcpy(raw,"HMP2",4);raw[4]=frame->count&255;raw[5]=frame->count>>8;raw[6]=raw[7]=0;
    for(unsigned i=0;i<frame->count;i++){unsigned char *p=raw+8+i*56;pose_put32(p,frame->bones[i].name);pose_put32(p+4,frame->bones[i].part);for(int j=0;j<12;j++){uint32_t bits;memcpy(&bits,&frame->bones[i].matrix[j],4);pose_put32(p+8+j*4,bits);}}
    size_t n=8+(size_t)frame->count*56,w=0;for(size_t i=0;i<n;i+=3){uint32_t v=(uint32_t)raw[i]<<16;if(i+1<n)v|=(uint32_t)raw[i+1]<<8;if(i+2<n)v|=raw[i+2];out[w++]=pose_b64[(v>>18)&63];out[w++]=pose_b64[(v>>12)&63];out[w++]=i+1<n?pose_b64[(v>>6)&63]:'=';out[w++]=i+2<n?pose_b64[v&63]:'=';}out[w]=0;return w;
}
static bool pose_decode(const char *src,PoseFrame *frame){
    unsigned char raw[8+POSE_MAX*56];size_t n=strlen(src),w=0;frame->count=0;if(!n||n%4||n>POSE_B64_MAX)return false;
    for(size_t i=0;i<n;i+=4){uint32_t bits=0;int padding=0;for(int j=0;j<4;j++){const char *p=strchr(pose_b64,src[i+j]);if(src[i+j]=='='){if(i+4!=n||j<2)return false;padding++;bits<<=6;}else{if(padding||!p||!src[i+j])return false;bits=(bits<<6)|(uint32_t)(p-pose_b64);}}
        if(padding>2||(padding==2&&(bits&0xffff))||(padding==1&&(bits&255)))return false;if(w+3-padding>sizeof raw)return false;raw[w++]=(unsigned char)(bits>>16);if(padding<2)raw[w++]=(unsigned char)(bits>>8);if(!padding)raw[w++]=(unsigned char)bits;
    }
    if(w<8||(memcmp(raw,"HMP1",4)&&memcmp(raw,"HMP2",4))||raw[6]||raw[7])return false;unsigned stride=!memcmp(raw,"HMP2",4)?56:52;unsigned count=raw[4]|(unsigned)raw[5]<<8;if(!count||count>POSE_MAX||w!=8+(size_t)count*stride)return false;
    for(unsigned i=0;i<count;i++){const unsigned char *p=raw+8+i*stride;PoseBone *bone=&frame->bones[i];bone->name=pose_u32(p);bone->part=stride==56?pose_u32(p+4):0;if(bone->part&&(bone->part<4||bone->part>6))return false;if(!bone->name)return false;for(unsigned j=0;j<i;j++)if(frame->bones[j].name==bone->name&&frame->bones[j].part==bone->part)return false;for(int j=0;j<12;j++){uint32_t bits=pose_u32(p+stride-48+j*4);memcpy(&bone->matrix[j],&bits,4);}if(!pose_matrix(bone->matrix))return false;}frame->count=count;return true;
}

// Bone and material visibility are independent of the skeletal palette.
// Player uses the material setter for Mt_Belt_* and other covered skin/hair.
// Names and kind cross the network; local resource indices never do.
static uint32_t visibility_name(uint64_t model,unsigned i,unsigned kind){
    if(kind){uint64_t r,p;unsigned n;return pose_model(model,&r,&p,&n)&&i<n?pose_name(r,i):0;}
    uint64_t res=LD64(model+0xe0);if(!range(res,0x50))return 0;
    uint64_t dict=LD64(res+0x48);if(!range(dict,0x28+(uint64_t)i*16))return 0;
    uint64_t text=LD64(dict+0x20+(uint64_t)i*16);if(!range(text,98))return 0;
    const char *name=(const char *)(uintptr_t)(text+2);size_t n=strnlen(name,96);return n&&n<96?pose_hash(name):0;
}
static bool visibility_model(uint64_t m,unsigned kind,unsigned *count,uint64_t *bits){
    uint64_t r,p;unsigned bones;if(!pose_model(m,&r,&p,&bones)||!g_site.hm_material_set||!g_site.hm_bone_visible_set)return false;
    uint64_t vt=LD64(m);if(!range(vt,0x1c0)||LD64(vt+0x1b0)!=g_site.hm_material_set||LD64(vt+0x88)!=g_site.hm_bone_visible_set)return false;
    *count=kind?bones:LD16(m+0x112);*bits=LD64(m+(kind?0xe8:0xf0));return *count>0&&*count<=VIS_MAX&&range(*bits,((*count+31)/32)*4);
}
static void visibility_capture(uint64_t a,unsigned part,Appearance *out){
    uint64_t units;unsigned n;if(!pose_units(a,&units,&n))return;
    for(unsigned j=0;j<n;j++)for(unsigned kind=0;kind<2;kind++){
        uint64_t link=LD64(units+j*8);if(!range(link,8))continue;uint64_t m=LD64(link),bits;unsigned count;if(!visibility_model(m,kind,&count,&bits))continue;
        for(unsigned i=0;i<count;i++){
            uint32_t name=visibility_name(m,i,kind);if(!name)continue;bool seen=false;for(unsigned k=0;k<out->visibility;k++)if(out->mask[k].part==part&&out->mask[k].name==name&&out->mask[k].kind==kind){seen=true;break;}if(seen)continue;
            if(out->visibility==VIS_MAX)return;out->mask[out->visibility++]=(VisibilityState){part,name,(LD32(bits+(i/32)*4)>>(i%32))&1,kind};
        }
    }
}
static void visibility_apply(Cpu *c,uint64_t a,const Appearance *state,unsigned part){
    uint64_t units;unsigned n,applied=0;if(!pose_units(a,&units,&n))return;
    for(unsigned j=0;j<n;j++)for(unsigned kind=0;kind<2;kind++){
        uint64_t link=LD64(units+j*8);if(!range(link,8))continue;uint64_t m=LD64(link),bits;unsigned count;if(!visibility_model(m,kind,&count,&bits))continue;
        for(unsigned i=0;i<count;i++){uint32_t name=visibility_name(m,i,kind);if(!name)continue;for(unsigned k=0;k<state->visibility;k++)if(state->mask[k].part==part&&state->mask[k].name==name&&state->mask[k].kind==kind){if(((LD32(bits+(i/32)*4)>>(i%32))&1)!=state->mask[k].visible)call(c,kind?g_site.hm_bone_visible_set:g_site.hm_material_set,m,i,state->mask[k].visible,0,0,0,0);applied++;break;}}
    }
    pthread_mutex_lock(&mutex);published.visibility_applied+=applied;pthread_mutex_unlock(&mutex);
}
static size_t appearance_encode(const Appearance *state,char *out){
    unsigned char raw[8+16*(VIS_MAX+PROP_MAX)];memcpy(raw,"HMA1",4);raw[4]=state->visibility&255;raw[5]=state->visibility>>8;raw[6]=state->properties&255;raw[7]=state->properties>>8;
    for(unsigned i=0;i<state->visibility;i++){unsigned char *p=raw+8+16*i;const VisibilityState *v=&state->mask[i];pose_put32(p,v->part);pose_put32(p+4,v->name);pose_put32(p+8,v->visible);pose_put32(p+12,v->kind);}
    for(unsigned i=0;i<state->properties;i++){unsigned char *p=raw+8+16*(state->visibility+i);const FxProperty *v=&state->property[i];pose_put32(p,v->slot);pose_put32(p+4,v->name);pose_put32(p+8,v->type);pose_put32(p+12,v->value);}
    size_t n=8+16*(state->visibility+state->properties),w=0;for(size_t i=0;i<n;i+=3){uint32_t v=(uint32_t)raw[i]<<16;if(i+1<n)v|=(uint32_t)raw[i+1]<<8;if(i+2<n)v|=raw[i+2];out[w++]=pose_b64[(v>>18)&63];out[w++]=pose_b64[(v>>12)&63];out[w++]=i+1<n?pose_b64[(v>>6)&63]:'=';out[w++]=i+2<n?pose_b64[v&63]:'=';}out[w]=0;return w;
}
static bool appearance_decode(const char *src,Appearance *out){
    unsigned char raw[8+16*(VIS_MAX+PROP_MAX)];size_t n=strlen(src),w=0;memset(out,0,sizeof *out);if(!n||n%4||n>APPEARANCE_B64_MAX)return false;
    for(size_t i=0;i<n;i+=4){uint32_t bits=0;int padding=0;for(int j=0;j<4;j++){const char *p=strchr(pose_b64,src[i+j]);if(src[i+j]=='='){if(i+4!=n||j<2)return false;padding++;bits<<=6;}else{if(padding||!p||!src[i+j])return false;bits=(bits<<6)|(uint32_t)(p-pose_b64);}}
        if(padding>2||(padding==2&&(bits&0xffff))||(padding==1&&(bits&255)))return false;if(w+3-padding>sizeof raw)return false;raw[w++]=(unsigned char)(bits>>16);if(padding<2)raw[w++]=(unsigned char)(bits>>8);if(!padding)raw[w++]=(unsigned char)bits;
    }
    if(w<8||memcmp(raw,"HMA1",4))return false;unsigned shapes=raw[4]|(unsigned)raw[5]<<8,props=raw[6]|(unsigned)raw[7]<<8;
    if(shapes>VIS_MAX||props>PROP_MAX||w!=8+16*(shapes+props))return false;
    for(unsigned i=0;i<shapes;i++){const unsigned char *p=raw+8+16*i;VisibilityState *v=&out->mask[i];*v=(VisibilityState){pose_u32(p),pose_u32(p+4),pose_u32(p+8),pose_u32(p+12)};if((v->part&& (v->part<4||v->part>6))||!v->name||v->visible>1||v->kind>1)return false;for(unsigned j=0;j<i;j++)if(v->part==out->mask[j].part&&v->name==out->mask[j].name&&v->kind==out->mask[j].kind)return false;}
    for(unsigned i=0;i<props;i++){const unsigned char *p=raw+8+16*(shapes+i);FxProperty *v=&out->property[i];*v=(FxProperty){pose_u32(p),pose_u32(p+4),pose_u32(p+8),pose_u32(p+12)};if(v->slot>3||!v->name||v->type>3)return false;if(v->type==2){float f;memcpy(&f,&v->value,4);if(!isfinite(f))return false;}for(unsigned j=0;j<i;j++)if(v->slot==out->property[j].slot&&v->name==out->property[j].name)return false;}
    out->visibility=shapes;out->properties=props;return true;
}
