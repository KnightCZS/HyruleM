// StageInfo's fixed strings are resolved by unique 1.9.0 signatures.
// MainField section names share one world. Every interior keeps its own name.
static char local_scene[65];
static bool actor_scene_valid(const char *s){
    size_t n=strnlen(s,65);if(!n||n>=65)return false;
    for(size_t i=0;i<n;i++){char c=s[i];if(!((c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')||c=='_'||c=='-'||c=='/'))return false;}
    return true;
}
static bool actor_scene_string(uint64_t object,char out[33]){
    if(!range(object,16))return false;
    uint64_t chars=LD64(object+8);
    if(!range(chars,33)||!memchr((void *)(uintptr_t)chars,0,33))return false;
    snprintf(out,33,"%s",(char *)(uintptr_t)chars);
    return actor_scene_valid(out);
}
static bool actor_scene_read(char out[65]){
    char type[33],name[33];out[0]=0;
    if(!actor_scene_string(g_site.hm_scene_type,type)||!actor_scene_string(g_site.hm_scene_name,name))return false;
    if(!strcmp(type,"MainField")||!strcmp(type,"AocField"))snprintf(out,65,"%s",type);
    else {if(strlen(type)+strlen(name)+1>=65)return false;snprintf(out,65,"%s/%s",type,name);}
    return actor_scene_valid(out);
}
