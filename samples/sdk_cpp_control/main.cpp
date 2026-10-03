#include "youyuan.hpp"
class Controller: public yy::Plugin {
 unsigned long long ticks_=0;
public:
 Controller(const char* context,yy::HostAPI api):Plugin(context,api){
  on("tick",[this](const yy::json&){return yy::json(++ticks_);});
  on("chat",[this](const yy::json& args){
   const auto& event=args.at(0);auto text=event.value("message",std::string());
   if(text==".原生状态") client.game.sendTargeted(event.at("player"),"C++ 常驻插件运行中");
   if(text==".命令选择"||text.rfind(".链 ",0)==0) return client.chains.handleChat(event.at("player"),text);
   return yy::json{{"ticks",ticks_}};
  });
  on("web_status",[this](const yy::json&){return yy::json{{"ticks",ticks_},{"account",client.account()}};});
  client.events.onChat("chat");client.util.schedule(2.0,"tick");
  client.web.page("control","C++ 插件控制",yy::json::array({{{"type","button"},{"action","status"},{"label","查看状态"}}}));
  client.web.action("control","status","web_status");
  expose("add",[](const yy::json& args){return yy::json(args.at(0).get<double>()+args.at(1).get<double>());});
  expose("state",[this](const yy::json&){return yy::json{{"ticks",ticks_},{"account",client.account()}};});
  expose("sum_python",[this](const yy::json& args){return client.api.require("yy_sdk_python_demo").call("add",args);});
  expose("execute",[this](const yy::json& args){return client.game.execute(args.at(0));});
  expose("chains",[this](const yy::json& args){return client.invoke(args.at(0),args.size()>1?args.at(1):yy::json::array());});
 }
};
YY_PLUGIN(Controller)
