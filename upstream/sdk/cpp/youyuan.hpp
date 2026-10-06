// YouYuan SDK 1.13; AGPL-3.0. nlohmann/json is MIT (see third_party/json.hpp).
#pragma once
#include "third_party/json.hpp"
#include <cstdlib>
#include <cstring>
#include <functional>
#include <map>
#include <memory>
#include <stdexcept>
#include <string>
namespace yy {
using json=nlohmann::json;
using HostAPI=char*(*)(char*,char*);
class Client;
class Namespace {
 Client* client_;std::string namespace_;
public:
 Namespace(Client* client,const char* name):client_(client),namespace_(name){}
 template<typename... A> json sendMessage(A&&... args)const{return _send("sendMessage",json::array({std::forward<A>(args)...}));}
 template<typename... A> json callPlugin(A&&... args)const{return _send("callPlugin",json::array({std::forward<A>(args)...}));}
 json _send(const std::string& method,const json& args=json::array())const;
 template<typename... A> json sendPacket(A&&... args)const{return _send("sendPacket",json::array({std::forward<A>(args)...}));}
 template<typename... A> json subscribeAll(A&&... args)const{return _send("subscribeAll",json::array({std::forward<A>(args)...}));}
 template<typename... A> json send(A&&... args)const{return _send("send",json::array({std::forward<A>(args)...}));}
 template<typename... A> json on(A&&... args)const{return _send("on",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onAll(A&&... args)const{return _send("onAll",json::array({std::forward<A>(args)...}));}
 template<typename... A> json schema(A&&... args)const{return _send("schema",json::array({std::forward<A>(args)...}));}
 template<typename... A> json describe(A&&... args)const{return _send("describe",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onSwing(A&&... args)const{return _send("onSwing",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onEntitySpawn(A&&... args)const{return _send("onEntitySpawn",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onEntityRemove(A&&... args)const{return _send("onEntityRemove",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onItemPickup(A&&... args)const{return _send("onItemPickup",json::array({std::forward<A>(args)...}));}
 template<typename... A> json scheduleHz(A&&... args)const{return _send("scheduleHz",json::array({std::forward<A>(args)...}));}
 template<typename... A> json scheduleSeconds(A&&... args)const{return _send("scheduleSeconds",json::array({std::forward<A>(args)...}));}
 template<typename... A> json scheduleTick(A&&... args)const{return _send("scheduleTick",json::array({std::forward<A>(args)...}));}
 template<typename... A> json listJobs(A&&... args)const{return _send("listJobs",json::array({std::forward<A>(args)...}));}
 template<typename... A> json pauseJob(A&&... args)const{return _send("pauseJob",json::array({std::forward<A>(args)...}));}
 template<typename... A> json resumeJob(A&&... args)const{return _send("resumeJob",json::array({std::forward<A>(args)...}));}
 template<typename... A> json stats(A&&... args)const{return _send("stats",json::array({std::forward<A>(args)...}));}
 template<typename... A> json has(A&&... args)const{return _send("has",json::array({std::forward<A>(args)...}));}
 template<typename... A> json capabilities(A&&... args)const{return _send("capabilities",json::array({std::forward<A>(args)...}));}

 template<typename... A> json onPlayerDeath(A&&... args)const{return _send("onPlayerDeath",json::array({std::forward<A>(args)...}));}
 template<typename... A> json atomicAdd(A&&... args)const{return _send("atomicAdd",json::array({std::forward<A>(args)...}));}
 template<typename... A> json flush(A&&... args)const{return _send("flush",json::array({std::forward<A>(args)...}));}
 template<typename... A> json transaction(A&&... args)const{return _send("transaction",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendCommandAsync(A&&... args)const{return _send("sendCommandAsync",json::array({std::forward<A>(args)...}));}
 template<typename... A> json commandResult(A&&... args)const{return _send("commandResult",json::array({std::forward<A>(args)...}));}
 // export is a C++ keyword; the wire API remains plugin.export.
 template<typename... A> json exportPlugin(A&&... args)const{return _send("export",json::array({std::forward<A>(args)...}));}
 template<typename... A> json reload(A&&... args)const{return _send("reload",json::array({std::forward<A>(args)...}));}
 template<typename... A> json require(A&&... args)const{return _send("require",json::array({std::forward<A>(args)...}));}
 template<typename... A> json set(A&&... args)const{return _send("set",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendCommandFast(A&&... args)const{return _send("sendCommandFast",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendCommandsFast(A&&... args)const{return _send("sendCommandsFast",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendCommandConfirmed(A&&... args)const{return _send("sendCommandConfirmed",json::array({std::forward<A>(args)...}));}
 template<typename... A> json codeDir(A&&... args)const{return _send("codeDir",json::array({std::forward<A>(args)...}));}
 template<typename... A> json configDir(A&&... args)const{return _send("configDir",json::array({std::forward<A>(args)...}));}
 template<typename... A> json resourceDir(A&&... args)const{return _send("resourceDir",json::array({std::forward<A>(args)...}));}
 template<typename... A> json tempDir(A&&... args)const{return _send("tempDir",json::array({std::forward<A>(args)...}));}
 template<typename... A> json readResource(A&&... args)const{return _send("readResource",json::array({std::forward<A>(args)...}));}
 template<typename... A> json listResources(A&&... args)const{return _send("listResources",json::array({std::forward<A>(args)...}));}
 template<typename... A> json existsResource(A&&... args)const{return _send("existsResource",json::array({std::forward<A>(args)...}));}
 template<typename... A> json extractResource(A&&... args)const{return _send("extractResource",json::array({std::forward<A>(args)...}));}

 template<typename... A> json handleChat(A&&... args)const{return _send("handleChat",json::array({std::forward<A>(args)...}));}
 template<typename... A> json export_(A&&... args)const{return _send("export",json::array({std::forward<A>(args)...}));}
 template<typename... A> json validate(A&&... args)const{return _send("validate",json::array({std::forward<A>(args)...}));}
 template<typename... A> json configure(A&&... args)const{return _send("configure",json::array({std::forward<A>(args)...}));}
 template<typename... A> json start(A&&... args)const{return _send("start",json::array({std::forward<A>(args)...}));}
 template<typename... A> json stop(A&&... args)const{return _send("stop",json::array({std::forward<A>(args)...}));}
 template<typename... A> json trigger(A&&... args)const{return _send("trigger",json::array({std::forward<A>(args)...}));}
 template<typename... A> json menu(A&&... args)const{return _send("menu",json::array({std::forward<A>(args)...}));}
 template<typename... A> json attack(A&&... args)const{return _send("attack",json::array({std::forward<A>(args)...}));}
 template<typename... A> json mineBlock(A&&... args)const{return _send("mineBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json useBlock(A&&... args)const{return _send("useBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json action(A&&... args)const{return _send("action",json::array({std::forward<A>(args)...}));}
 template<typename... A> json block(A&&... args)const{return _send("block",json::array({std::forward<A>(args)...}));}
 template<typename... A> json blockState(A&&... args)const{return _send("blockState",json::array({std::forward<A>(args)...}));}
 template<typename... A> json breakBlock(A&&... args)const{return _send("breakBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json call(A&&... args)const{return _send("call",json::array({std::forward<A>(args)...}));}
 template<typename... A> json cancel(A&&... args)const{return _send("cancel",json::array({std::forward<A>(args)...}));}
 template<typename... A> json captureClear(A&&... args)const{return _send("captureClear",json::array({std::forward<A>(args)...}));}
 template<typename... A> json captureRead(A&&... args)const{return _send("captureRead",json::array({std::forward<A>(args)...}));}
 template<typename... A> json captureStart(A&&... args)const{return _send("captureStart",json::array({std::forward<A>(args)...}));}
 template<typename... A> json captureStop(A&&... args)const{return _send("captureStop",json::array({std::forward<A>(args)...}));}
 template<typename... A> json ceil(A&&... args)const{return _send("ceil",json::array({std::forward<A>(args)...}));}
 template<typename... A> json closestColor(A&&... args)const{return _send("closestColor",json::array({std::forward<A>(args)...}));}
 template<typename... A> json count(A&&... args)const{return _send("count",json::array({std::forward<A>(args)...}));}
 template<typename... A> json current(A&&... args)const{return _send("current",json::array({std::forward<A>(args)...}));}
 template<typename... A> json dataDir(A&&... args)const{return _send("dataDir",json::array({std::forward<A>(args)...}));}
 template<typename... A> json delete_(A&&... args)const{return _send("delete",json::array({std::forward<A>(args)...}));}
 template<typename... A> json dependencies(A&&... args)const{return _send("dependencies",json::array({std::forward<A>(args)...}));}
 template<typename... A> json detect(A&&... args)const{return _send("detect",json::array({std::forward<A>(args)...}));}
 template<typename... A> json emit(A&&... args)const{return _send("emit",json::array({std::forward<A>(args)...}));}
 template<typename... A> json execute(A&&... args)const{return _send("execute",json::array({std::forward<A>(args)...}));}
 template<typename... A> json executeBatch(A&&... args)const{return _send("executeBatch",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setCommandOptions(A&&... args)const{return _send("setCommandOptions",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getCommandOptions(A&&... args)const{return _send("getCommandOptions",json::array({std::forward<A>(args)...}));}
 template<typename... A> json floor(A&&... args)const{return _send("floor",json::array({std::forward<A>(args)...}));}
 template<typename... A> json flyStart(A&&... args)const{return _send("flyStart",json::array({std::forward<A>(args)...}));}
 template<typename... A> json flyStop(A&&... args)const{return _send("flyStop",json::array({std::forward<A>(args)...}));}
 template<typename... A> json get(A&&... args)const{return _send("get",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getBlockData(A&&... args)const{return _send("getBlockData",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getBlockNBT(A&&... args)const{return _send("getBlockNBT",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getBlockState(A&&... args)const{return _send("getBlockState",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getBlockTile(A&&... args)const{return _send("getBlockTile",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getBlockTileRaw(A&&... args)const{return _send("getBlockTileRaw",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getBotPos(A&&... args)const{return _send("getBotPos",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getConfig(A&&... args)const{return _send("getConfig",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getInventory(A&&... args)const{return _send("getInventory",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getMultiScore(A&&... args)const{return _send("getMultiScore",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getPlayerItem(A&&... args)const{return _send("getPlayerItem",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getPlayerPos(A&&... args)const{return _send("getPlayerPos",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getPlayerPosXYZ(A&&... args)const{return _send("getPlayerPosXYZ",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getRegionBlockData(A&&... args)const{return _send("getRegionBlockData",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getScore(A&&... args)const{return _send("getScore",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getTarget(A&&... args)const{return _send("getTarget",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getTickingAreaList(A&&... args)const{return _send("getTickingAreaList",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getUniqueID(A&&... args)const{return _send("getUniqueID",json::array({std::forward<A>(args)...}));}
 template<typename... A> json getXUID(A&&... args)const{return _send("getXUID",json::array({std::forward<A>(args)...}));}
 template<typename... A> json health(A&&... args)const{return _send("health",json::array({std::forward<A>(args)...}));}
 template<typename... A> json httpGet(A&&... args)const{return _send("httpGet",json::array({std::forward<A>(args)...}));}
 template<typename... A> json httpPost(A&&... args)const{return _send("httpPost",json::array({std::forward<A>(args)...}));}
 template<typename... A> json id(A&&... args)const{return _send("id",json::array({std::forward<A>(args)...}));}
 template<typename... A> json imageToAscii(A&&... args)const{return _send("imageToAscii",json::array({std::forward<A>(args)...}));}
 template<typename... A> json invoke(A&&... args)const{return _send("invoke",json::array({std::forward<A>(args)...}));}
 template<typename... A> json isAir(A&&... args)const{return _send("isAir",json::array({std::forward<A>(args)...}));}
 template<typename... A> json isCmdSuccess(A&&... args)const{return _send("isCmdSuccess",json::array({std::forward<A>(args)...}));}
 template<typename... A> json isCommandBlock(A&&... args)const{return _send("isCommandBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json isLava(A&&... args)const{return _send("isLava",json::array({std::forward<A>(args)...}));}
 template<typename... A> json isOP(A&&... args)const{return _send("isOP",json::array({std::forward<A>(args)...}));}
 template<typename... A> json isWater(A&&... args)const{return _send("isWater",json::array({std::forward<A>(args)...}));}
 template<typename... A> json join(A&&... args)const{return _send("join",json::array({std::forward<A>(args)...}));}
 template<typename... A> json jsonDecode(A&&... args)const{return _send("jsonDecode",json::array({std::forward<A>(args)...}));}
 template<typename... A> json jsonEncode(A&&... args)const{return _send("jsonEncode",json::array({std::forward<A>(args)...}));}
 template<typename... A> json json_decode(A&&... args)const{return _send("json_decode",json::array({std::forward<A>(args)...}));}
 template<typename... A> json json_encode(A&&... args)const{return _send("json_encode",json::array({std::forward<A>(args)...}));}
 template<typename... A> json json_load(A&&... args)const{return _send("json_load",json::array({std::forward<A>(args)...}));}
 template<typename... A> json json_write(A&&... args)const{return _send("json_write",json::array({std::forward<A>(args)...}));}
 template<typename... A> json jump(A&&... args)const{return _send("jump",json::array({std::forward<A>(args)...}));}
 template<typename... A> json list(A&&... args)const{return _send("list",json::array({std::forward<A>(args)...}));}
 template<typename... A> json lookAt(A&&... args)const{return _send("lookAt",json::array({std::forward<A>(args)...}));}
 template<typename... A> json move(A&&... args)const{return _send("move",json::array({std::forward<A>(args)...}));}
 template<typename... A> json nearby(A&&... args)const{return _send("nearby",json::array({std::forward<A>(args)...}));}
 template<typename... A> json noteToPitch(A&&... args)const{return _send("noteToPitch",json::array({std::forward<A>(args)...}));}
 template<typename... A> json notify(A&&... args)const{return _send("notify",json::array({std::forward<A>(args)...}));}
 template<typename... A> json now(A&&... args)const{return _send("now",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onBlockChange(A&&... args)const{return _send("onBlockChange",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onBotReady(A&&... args)const{return _send("onBotReady",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onChat(A&&... args)const{return _send("onChat",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onDeath(A&&... args)const{return _send("onDeath",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onHealthChange(A&&... args)const{return _send("onHealthChange",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onHurt(A&&... args)const{return _send("onHurt",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onJoin(A&&... args)const{return _send("onJoin",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onLeave(A&&... args)const{return _send("onLeave",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onMove(A&&... args)const{return _send("onMove",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onPacket(A&&... args)const{return _send("onPacket",json::array({std::forward<A>(args)...}));}
 template<typename... A> json onRespawn(A&&... args)const{return _send("onRespawn",json::array({std::forward<A>(args)...}));}
 template<typename... A> json packetSchema(A&&... args)const{return _send("packetSchema",json::array({std::forward<A>(args)...}));}
 template<typename... A> json page(A&&... args)const{return _send("page",json::array({std::forward<A>(args)...}));}
 template<typename... A> json parse(A&&... args)const{return _send("parse",json::array({std::forward<A>(args)...}));}
 template<typename... A> json parseMidi(A&&... args)const{return _send("parseMidi",json::array({std::forward<A>(args)...}));}
 template<typename... A> json placeBlock(A&&... args)const{return _send("placeBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json placeNBTBlock(A&&... args)const{return _send("placeNBTBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json platform(A&&... args)const{return _send("platform",json::array({std::forward<A>(args)...}));}
 template<typename... A> json pos(A&&... args)const{return _send("pos",json::array({std::forward<A>(args)...}));}
 template<typename... A> json position(A&&... args)const{return _send("position",json::array({std::forward<A>(args)...}));}
 template<typename... A> json post(A&&... args)const{return _send("post",json::array({std::forward<A>(args)...}));}
 template<typename... A> json promptChoice(A&&... args)const{return _send("promptChoice",json::array({std::forward<A>(args)...}));}
 template<typename... A> json put(A&&... args)const{return _send("put",json::array({std::forward<A>(args)...}));}
 template<typename... A> json querify(A&&... args)const{return _send("querify",json::array({std::forward<A>(args)...}));}
 template<typename... A> json queryInventory(A&&... args)const{return _send("queryInventory",json::array({std::forward<A>(args)...}));}
 template<typename... A> json read_file(A&&... args)const{return _send("read_file",json::array({std::forward<A>(args)...}));}
 template<typename... A> json registerTrigger(A&&... args)const{return _send("registerTrigger",json::array({std::forward<A>(args)...}));}
 template<typename... A> json request(A&&... args)const{return _send("request",json::array({std::forward<A>(args)...}));}
 template<typename... A> json rotate(A&&... args)const{return _send("rotate",json::array({std::forward<A>(args)...}));}
 template<typename... A> json round(A&&... args)const{return _send("round",json::array({std::forward<A>(args)...}));}
 template<typename... A> json save(A&&... args)const{return _send("save",json::array({std::forward<A>(args)...}));}
 template<typename... A> json scene(A&&... args)const{return _send("scene",json::array({std::forward<A>(args)...}));}
 template<typename... A> json schedule(A&&... args)const{return _send("schedule",json::array({std::forward<A>(args)...}));}
 template<typename... A> json select(A&&... args)const{return _send("select",json::array({std::forward<A>(args)...}));}
 template<typename... A> json self(A&&... args)const{return _send("self",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendActionbar(A&&... args)const{return _send("sendActionbar",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendChat(A&&... args)const{return _send("sendChat",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendCommand(A&&... args)const{return _send("sendCommand",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendLines(A&&... args)const{return _send("sendLines",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendRichTellraw(A&&... args)const{return _send("sendRichTellraw",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendTargeted(A&&... args)const{return _send("sendTargeted",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sendTitle(A&&... args)const{return _send("sendTitle",json::array({std::forward<A>(args)...}));}
 template<typename... A> json serve(A&&... args)const{return _send("serve",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setBlock(A&&... args)const{return _send("setBlock",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setBlockState(A&&... args)const{return _send("setBlockState",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setConfigDefault(A&&... args)const{return _send("setConfigDefault",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setEffect(A&&... args)const{return _send("setEffect",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setScore(A&&... args)const{return _send("setScore",json::array({std::forward<A>(args)...}));}
 template<typename... A> json setTimeout(A&&... args)const{return _send("setTimeout",json::array({std::forward<A>(args)...}));}
 template<typename... A> json shell(A&&... args)const{return _send("shell",json::array({std::forward<A>(args)...}));}
 template<typename... A> json shizukuRequest(A&&... args)const{return _send("shizukuRequest",json::array({std::forward<A>(args)...}));}
 template<typename... A> json shizukuStatus(A&&... args)const{return _send("shizukuStatus",json::array({std::forward<A>(args)...}));}
 template<typename... A> json showMenu(A&&... args)const{return _send("showMenu",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sneakStart(A&&... args)const{return _send("sneakStart",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sneakStop(A&&... args)const{return _send("sneakStop",json::array({std::forward<A>(args)...}));}
 template<typename... A> json split(A&&... args)const{return _send("split",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sprintStart(A&&... args)const{return _send("sprintStart",json::array({std::forward<A>(args)...}));}
 template<typename... A> json sprintStop(A&&... args)const{return _send("sprintStop",json::array({std::forward<A>(args)...}));}
 template<typename... A> json startMovement(A&&... args)const{return _send("startMovement",json::array({std::forward<A>(args)...}));}
 template<typename... A> json state(A&&... args)const{return _send("state",json::array({std::forward<A>(args)...}));}
 template<typename... A> json status(A&&... args)const{return _send("status",json::array({std::forward<A>(args)...}));}
 template<typename... A> json stopMove(A&&... args)const{return _send("stopMove",json::array({std::forward<A>(args)...}));}
 template<typename... A> json stopMovement(A&&... args)const{return _send("stopMovement",json::array({std::forward<A>(args)...}));}
 template<typename... A> json strip_color(A&&... args)const{return _send("strip_color",json::array({std::forward<A>(args)...}));}
 template<typename... A> json subscribe(A&&... args)const{return _send("subscribe",json::array({std::forward<A>(args)...}));}
 template<typename... A> json subscribeFiltered(A&&... args)const{return _send("subscribeFiltered",json::array({std::forward<A>(args)...}));}
 template<typename... A> json teleport(A&&... args)const{return _send("teleport",json::array({std::forward<A>(args)...}));}
 template<typename... A> json time_fmt(A&&... args)const{return _send("time_fmt",json::array({std::forward<A>(args)...}));}
 template<typename... A> json timestamp(A&&... args)const{return _send("timestamp",json::array({std::forward<A>(args)...}));}
 template<typename... A> json trim(A&&... args)const{return _send("trim",json::array({std::forward<A>(args)...}));}
 template<typename... A> json vibrate(A&&... args)const{return _send("vibrate",json::array({std::forward<A>(args)...}));}
 template<typename... A> json waitMsg(A&&... args)const{return _send("waitMsg",json::array({std::forward<A>(args)...}));}
 template<typename... A> json write_file(A&&... args)const{return _send("write_file",json::array({std::forward<A>(args)...}));}
};
class RemotePlugin {
 Client* client_;std::string id_;
public:RemotePlugin(Client* c,std::string id):client_(c),id_(std::move(id)){}
 json call(const std::string& method,const json& args=json::array())const;
};
class API:public Namespace{
 Client* client_;
public:explicit API(Client* c):Namespace(c,"api"),client_(c){}
 RemotePlugin require(const std::string& id)const{return RemotePlugin(client_,id);}
};
class Client {
 HostAPI host_;json context_;
public:
 Namespace game,system,plugin,util,bot,building,media,world,player,events,http,web,chains,packet,storage,accounts;API api;
 Client(const char* context,HostAPI host):host_(host),context_(json::parse(context)),
 game(this,"game"),system(this,"system"),plugin(this,"plugin"),util(this,"util"),bot(this,"bot"),building(this,"building"),media(this,"media"),world(this,"world"),player(this,"player"),events(this,"events"),http(this,"http"),web(this,"web"),chains(this,"chains"),packet(this,"packet"),storage(this,"storage"),accounts(this,"accounts"),api(this){}
 std::string account()const{return context_.at("account").get<std::string>();}
 std::string id()const{return context_.at("plugin_id").get<std::string>();}
 json root(const std::string& path,const json& body=nullptr)const{
  std::string text=body.is_null()?"":body.dump();
  std::unique_ptr<char,decltype(&std::free)> raw(host_(const_cast<char*>(path.c_str()),const_cast<char*>(text.c_str())),&std::free);
  if(!raw)throw std::runtime_error("Host returned null");
  auto reply=json::parse(raw.get());
  if(reply.is_object()&&reply.value("ok",true)==false)throw std::runtime_error(reply.value("error",std::string("Host call failed")));
  return reply;
 }
 json invoke(const std::string& method,const json& args=json::array())const{
  return root("/api/youyuan/sdk/call",{{"account",account()},{"plugin_id",id()},{"method",method},{"args",args}}).at("result");
 }
 json batch(const json& calls)const{return root("/api/youyuan/sdk/batch",{{"account",account()},{"plugin_id",id()},{"calls",calls}}).at("results");}
 json console(const std::string& input)const{return root("/api/youyuan/account/action",{{"account",account()},{"action","console"},{"input",input}});}
};
inline json Namespace::_send(const std::string& method,const json& args)const{return client_->invoke(namespace_+"."+method,args);}
inline json RemotePlugin::call(const std::string& method,const json& args)const{return client_->invoke("api.call",json::array({id_,method,args}));}
class CommandBuffer {
 Client* client_;json commands_=json::array();
public:explicit CommandBuffer(Client& c):client_(&c){}
 CommandBuffer& add(const std::string& c){commands_.push_back(c);return *this;}
 void clear(){commands_=json::array();}std::size_t size()const{return commands_.size();}
 json submitFast(const json& options=json::object())const{return client_->game.sendCommandsFast(commands_,options);}
};
class Plugin {
 using Callback=std::function<json(const json&)>;
 std::map<std::string,Callback> callbacks_,exports_;
public:
 Client client;
 Plugin(const char* context,HostAPI host):client(context,host){}
 virtual ~Plugin()=default;
 void on(const std::string& name,Callback callback){callbacks_[name]=std::move(callback);}
 void expose(const std::string& name,Callback callback){exports_[name]=std::move(callback);}
 virtual json event(const json& data){return {{"ok",true}};}
 json dispatch(const json& data){
  auto type=data.value("type",std::string());
  if(type=="sdk_callback"||type=="sdk_rpc"){
   auto& handlers=type=="sdk_rpc"?exports_:callbacks_;auto name=data.at("handler").get<std::string>();
   auto callback=handlers.find(name);if(callback==handlers.end())throw std::runtime_error("Unknown handler: "+name);
   return {{"ok",true},{"result",callback->second(data.value("args",json::array()))}};
  }
  return event(data);
 }
};
inline char* reply(const json& value){auto text=value.dump();auto* out=static_cast<char*>(std::malloc(text.size()+1));if(!out)return nullptr;std::memcpy(out,text.c_str(),text.size()+1);return out;}
} // namespace yy
// Exceptions must be caught before crossing the C ABI.
#define YY_PLUGIN(TYPE) \
extern "C" __attribute__((visibility("default"))) void* yy_create_v2(const char* context,yy::HostAPI api){try{return new TYPE(context,api);}catch(...){return nullptr;}} \
extern "C" __attribute__((visibility("default"))) char* yy_event(void* p,const char* event){try{return yy::reply(static_cast<TYPE*>(p)->dispatch(yy::json::parse(event)));}catch(const std::exception& e){return yy::reply({{"ok",false},{"error",e.what()}});}catch(...){return yy::reply({{"ok",false},{"error","Unknown C++ exception"}});}} \
extern "C" __attribute__((visibility("default"))) void yy_destroy(void* p){try{delete static_cast<TYPE*>(p);}catch(...){}}
