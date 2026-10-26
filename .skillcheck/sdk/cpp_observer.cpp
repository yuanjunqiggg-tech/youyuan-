#include "youyuan.hpp"
class Observer: public yy::Plugin {
 yy::json counts_=yy::json::object(), last_=yy::json::object(); bool owns_capture_=false;
public:
 Observer(const char* context,yy::HostAPI host):yy::Plugin(context,host){
  for(auto kind:{"death","health","hurt","move","respawn","block_change","filtered_packet"}){
   counts_[kind]=0; std::string name(kind);
   on(name,[this,name](const yy::json& args){counts_[name]=counts_[name].get<int>()+1;last_[name]=args.at(0);return nullptr;});
  }
  client.events.onDeath("death");client.events.onHealthChange("health");client.events.onHurt("hurt");client.events.onMove("move");client.events.onRespawn("respawn");client.events.onBlockChange("block_change");
  client.events.subscribeFiltered("UpdateAttributes",yy::json{{"every_ms",250}},"filtered_packet");
  client.web.page("monitor","C++ 游戏感知",yy::json::array({{{"type","button"},{"action","scene"},{"label","读取位置与血量"}},{{"type","button"},{"action","capture_start"},{"label","开始事件抓包"}},{{"type","button"},{"action","capture_stop"},{"label","停止抓包"}}}));
  on("scene",[this](const yy::json&){return yy::json{{"scene",client.game.scene()},{"counts",counts_},{"last",last_}};});
  on("capture_start",[this](const yy::json&){auto result=client.events.captureStart(yy::json{{"capacity",256},{"filter",{{"every_ms",100}}}});owns_capture_=true;return result;});
  on("capture_stop",[this](const yy::json&){auto result=client.events.captureStop();owns_capture_=false;return result;});
  for(auto action:{"scene","capture_start","capture_stop"})client.web.action("monitor",action,action);
  expose("scene",[this](const yy::json&){return client.game.scene();});
  expose("metrics",[this](const yy::json&){return yy::json{{"counts",counts_},{"last",last_}};});
  expose("health",[this](const yy::json& args){return client.player.health(args.empty()?yy::json("self"):args.at(0));});
  expose("batch_probe",[this](const yy::json&){return client.batch(yy::json::array({{{"method","util.floor"},{"args",yy::json::array({3.9})}},{{"method","bot.self"},{"args",yy::json::array()}}}));});
 }
 ~Observer(){if(owns_capture_){try{client.events.captureStop();}catch(...){}}}
};
YY_PLUGIN(Observer)
