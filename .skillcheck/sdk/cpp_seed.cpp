#include "youyuan.hpp"
class Controller: public yy::Plugin {
 unsigned long long ticks_=0;
public:
 Controller(const char* context,yy::HostAPI api):Plugin(context,api){
  on("tick",[this](const yy::json&){return yy::json(++ticks_);});
  on("chat",[this](const yy::json& args){
   if(args.size()==2&&args[1]==".原生状态")
    client.game.sendTargeted(args[0],"C++ 常驻插件运行中");
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
 }
};
YY_PLUGIN(Controller)
