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
static void pose_capture(uint64_t a,const float xyz[3],float yaw,PoseFrame *out){
    memset(out,0,sizeof *out);uint64_t units;unsigned n;if(!pose_units(a,&units,&n))return;
    float s=sinf(yaw),co=cosf(yaw);
    for(unsigned unit=0;unit<n;unit++){
        uint64_t link=LD64(units+unit*8);if(!range(link,8))continue;uint64_t model=LD64(link),res,palette;unsigned count;
        if(!pose_model(model,&res,&palette,&count))continue;
        for(unsigned i=0;i<count;i++){
            uint32_t name=pose_name(res,i);if(!name)continue;bool seen=false;for(unsigned j=0;j<out->count;j++)if(out->bones[j].name==name){seen=true;break;}if(seen)continue;
            PoseBone bone={0};bone.name=name;float world[12];for(int row=0;row<3;row++)for(int col=0;col<4;col++)world[row*4+col]=read_float(palette+(uint64_t)i*64+(col*4+row)*4);
            for(int j=0;j<3;j++){bone.matrix[j]=co*world[j]-s*world[8+j];bone.matrix[4+j]=world[4+j];bone.matrix[8+j]=s*world[j]+co*world[8+j];}
            float dx=world[3]-xyz[0],dy=world[7]-xyz[1],dz=world[11]-xyz[2];bone.matrix[3]=co*dx-s*dz;bone.matrix[7]=dy;bone.matrix[11]=s*dx+co*dz;
            if(!pose_matrix(bone.matrix)){out->count=0;return;}if(out->count>=POSE_MAX){out->count=0;return;}out->bones[out->count++]=bone;
        }
    }
}
static unsigned pose_apply(Cpu *c,uint64_t a,uint64_t scratch,const Desired *d){
    if(!d->pose.count)return 0;uint64_t units;unsigned n,applied=0;if(!pose_units(a,&units,&n))return 0;
    float s=sinf(d->yaw),co=cosf(d->yaw);
    for(unsigned unit=0;unit<n;unit++){
        uint64_t link=LD64(units+unit*8);if(!range(link,8))continue;uint64_t model=LD64(link),res,palette;unsigned count;
        if(!pose_model(model,&res,&palette,&count))continue;
        for(unsigned i=0;i<count;i++){
            uint32_t name=pose_name(res,i);if(!name)continue;
            const PoseBone *bone=NULL;for(unsigned j=0;j<d->pose.count;j++)if(d->pose.bones[j].name==name){bone=&d->pose.bones[j];break;}if(!bone)continue;
            float m[12];for(int j=0;j<4;j++){m[j]=co*bone->matrix[j]+s*bone->matrix[8+j];m[4+j]=bone->matrix[4+j];m[8+j]=-s*bone->matrix[j]+co*bone->matrix[8+j];}m[3]+=d->x;m[7]+=d->y;m[11]+=d->z;
            memcpy((void *)(uintptr_t)(scratch+MATRIX),m,sizeof m);call(c,g_site.hm_bone_set,model,scratch+MATRIX,i,0,0,0,0);applied++;
        }
    }
    return applied;
}
static const char pose_b64[]="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
static size_t pose_encode(const PoseFrame *frame,char *out){
    unsigned char raw[8+POSE_MAX*52];memcpy(raw,"HMP1",4);raw[4]=frame->count&255;raw[5]=frame->count>>8;raw[6]=raw[7]=0;
    for(unsigned i=0;i<frame->count;i++){unsigned char *p=raw+8+i*52;pose_put32(p,frame->bones[i].name);for(int j=0;j<12;j++){uint32_t bits;memcpy(&bits,&frame->bones[i].matrix[j],4);pose_put32(p+4+j*4,bits);}}
    size_t n=8+(size_t)frame->count*52,w=0;for(size_t i=0;i<n;i+=3){uint32_t v=(uint32_t)raw[i]<<16;if(i+1<n)v|=(uint32_t)raw[i+1]<<8;if(i+2<n)v|=raw[i+2];out[w++]=pose_b64[(v>>18)&63];out[w++]=pose_b64[(v>>12)&63];out[w++]=i+1<n?pose_b64[(v>>6)&63]:'=';out[w++]=i+2<n?pose_b64[v&63]:'=';}out[w]=0;return w;
}
static bool pose_decode(const char *src,PoseFrame *frame){
    unsigned char raw[8+POSE_MAX*52];size_t n=strlen(src),w=0;frame->count=0;if(!n||n%4||n>POSE_B64_MAX)return false;
    for(size_t i=0;i<n;i+=4){uint32_t bits=0;int padding=0;for(int j=0;j<4;j++){const char *p=strchr(pose_b64,src[i+j]);if(src[i+j]=='='){if(i+4!=n||j<2)return false;padding++;bits<<=6;}else{if(padding||!p||!src[i+j])return false;bits=(bits<<6)|(uint32_t)(p-pose_b64);}}
        if(padding>2||(padding==2&&(bits&0xffff))||(padding==1&&(bits&255)))return false;if(w+3-padding>sizeof raw)return false;raw[w++]=(unsigned char)(bits>>16);if(padding<2)raw[w++]=(unsigned char)(bits>>8);if(!padding)raw[w++]=(unsigned char)bits;
    }
    if(w<8||memcmp(raw,"HMP1",4)||raw[6]||raw[7])return false;unsigned count=raw[4]|(unsigned)raw[5]<<8;if(!count||count>POSE_MAX||w!=8+(size_t)count*52)return false;
    for(unsigned i=0;i<count;i++){const unsigned char *p=raw+8+i*52;PoseBone *bone=&frame->bones[i];bone->name=pose_u32(p);if(!bone->name)return false;for(unsigned j=0;j<i;j++)if(frame->bones[j].name==bone->name)return false;for(int j=0;j<12;j++){uint32_t bits=pose_u32(p+4+j*4);memcpy(&bone->matrix[j],&bits,4);}if(!pose_matrix(bone->matrix))return false;}frame->count=count;return true;
}
