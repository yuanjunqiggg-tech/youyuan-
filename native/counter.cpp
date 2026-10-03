#include <cstdlib>
#include <cstring>
#include <string>

// C ABI：每个账号得到独立实例，调用间保留 C++ 状态。
typedef char* (*HostAPI)(char*,char*);
struct Counter { unsigned long long value=0; std::string account; HostAPI host=nullptr; };
extern "C" __attribute__((visibility("default"))) void* yy_create(const char* account,HostAPI host) {auto* state=new Counter;state->account=account;state->host=host;return state;}
extern "C" __attribute__((visibility("default"))) char* yy_event(void* instance,const char* event) {
    auto* state=static_cast<Counter*>(instance);
    if(std::strstr(event,"\"probe_api\"")) {
        auto* response=state->host(const_cast<char*>("/api/bot/status"),const_cast<char*>(""));
        return response;
    }
    const std::string output="{\"ok\":true,\"counter\":"+std::to_string(++state->value)+",\"language\":\"native_cpp\"}";
    auto* result=static_cast<char*>(std::malloc(output.size()+1));std::memcpy(result,output.c_str(),output.size()+1);return result;
}
extern "C" __attribute__((visibility("default"))) void yy_destroy(void* instance){delete static_cast<Counter*>(instance);}
