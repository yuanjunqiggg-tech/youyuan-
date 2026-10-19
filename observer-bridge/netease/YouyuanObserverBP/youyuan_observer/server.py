# -*- coding: utf-8 -*-
"""Read-only server engine bridge. Compatible with the ModSDK Python 2.7 syntax.

Engine callbacks, not a remote spectator, provide the observations. No API in
this module changes health, damage, game mode, position or event cancellation.
"""
import math
import time
import uuid
import mod.server.extraServerApi as serverApi
from .config import ALLOWED_PLAYER_IDS

ServerSystem = serverApi.GetServerSystemCls()
try:
    string_types = (basestring,)
except NameError:
    string_types = (str,)

def exact_id(value):
    return value if isinstance(value, string_types) and 0 < len(value) <= 128 else None

def finite_number(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None
    number = float(value)
    return number if not math.isnan(number) and not math.isinf(number) else None

class ObserverServer(ServerSystem):
    def __init__(self, namespace, systemName):
        ServerSystem.__init__(self, namespace, systemName)
        self.allowed = set(exact_id(p) for p in ALLOWED_PLAYER_IDS if exact_id(p))
        self.subscribers = set()
        self.session = str(uuid.uuid4())
        self.sequence = 0
        self.bindings = []
        for event, handler in [
            ("HealthChangeServerEvent", self.on_health),
            ("PlayerAttackEntityEvent", self.on_attack),
            ("PlayerDieEvent", self.on_player_death),
            ("MobDieEvent", self.on_mob_death),
            ("DelServerPlayerEvent", self.on_leave)]:
            self._listen(serverApi.GetEngineNamespace(), serverApi.GetEngineSystemName(), event, handler)
        self._listen("YouyuanBridge", "ObserverV1", "Hello", self.on_hello)
        self._listen("YouyuanBridge", "ObserverV1", "Query", self.on_query)

    def _listen(self, namespace, system, event, handler):
        self.ListenForEvent(namespace, system, event, self, handler)
        self.bindings.append((namespace, system, event, handler))

    def _sender(self, args):
        # ModSDK injects __id__ in server callbacks for client-origin events.
        # Do not fall back to body.playerId, name, XUID, or a guessed neighbour.
        player = exact_id(args.get("__id__"))
        return player if player in self.allowed else None

    def _request(self, args):
        request = exact_id(args.get("request_id"))
        if args.get("protocol") != 1 or request is None:
            return None, None
        return self._sender(args), request

    def on_hello(self, args):
        player, request = self._request(args)
        if player is None:
            return
        self.NotifyToClient(player, "Capabilities", {
            "protocol": 1, "request_id": request,
            "provider": "netease_server_modsdk", "source": "engine_server",
            "capabilities": ["entity_health", "player_online_list"],
            "events": ["health_changed", "entity_attack", "player_death", "entity_death"],
            "swing_supported": False})
        self.subscribers.add(player)

    def health(self, entity):
        entity = exact_id(entity)
        if entity is None:
            return {"known": False, "reason": "exact_entity_id_required"}
        try:
            attr = serverApi.GetEngineCompFactory().CreateAttr(entity)
            health_type = serverApi.GetMinecraftEnum().AttrType.HEALTH
            current = finite_number(attr.GetAttrValue(health_type))
            maximum = finite_number(attr.GetAttrMaxValue(health_type))
        except Exception:
            return {"known": False, "entity_id": entity, "reason": "engine_attribute_unavailable"}
        if current is None or current < 0:
            return {"known": False, "entity_id": entity, "reason": "engine_attribute_unknown"}
        if maximum is not None and (maximum <= 0 or current > maximum):
            return {"known": False, "entity_id": entity, "reason": "inconsistent_engine_attribute"}
        return {"known": True, "entity_id": entity,
                "health": {"current": current, "max": maximum}, "source": "GetAttrValue/GetAttrMaxValue"}

    def on_query(self, args):
        player, request = self._request(args)
        if player is None:
            return
        query = args.get("query")
        if query == "entity_health":
            result = self.health(args.get("target"))
        elif query == "player_online_list":
            result = {"players": list(serverApi.GetPlayerList()), "identity_format": "engine_entity_id"}
        else:
            self.NotifyToClient(player, "Reply", {"protocol": 1, "request_id": request, "ok": False, "error": "unsupported_query"})
            return
        self.NotifyToClient(player, "Reply", {"protocol": 1, "request_id": request, "ok": True, "result": result})
        if query == "entity_health" and result.get("known"):
            self._forward("health_snapshot", result)

    def _forward(self, kind, data):
        if not self.subscribers:
            return
        self.sequence += 1
        data = dict(data)
        data.update({"event_id": self.session + ":" + str(self.sequence),
                     "timestamp_ms": int(time.time() * 1000), "source": "netease_server_modsdk"})
        payload = {"protocol": 1, "event_type": kind, "data": data}
        # Explicit destinations; no distance or dimension selector. Use SDK's
        # batched notification instead of N repeated calls when available.
        self.NotifyToMultiClients(list(self.subscribers), "Observation", payload)

    def on_health(self, args):
        entity = exact_id(args.get("entityId"))
        before, after = finite_number(args.get("from")), finite_number(args.get("to"))
        if entity is None or after is None or after < 0:
            return
        snapshot = self.health(entity)
        maximum = snapshot.get("health", {}).get("max") if snapshot.get("known") else None
        # HealthChangeServerEvent documents from/to as the before/after values.
        # Current comes from this event; max comes from the actual engine.
        self._forward("health_changed", {"entity_id": entity,
            "health": {"current": after, "previous": before, "max": maximum},
            "by_script": args.get("byScript"), "measurement": "HealthChangeServerEvent.from/to"})

    def on_attack(self, args):
        player, victim = exact_id(args.get("playerId")), exact_id(args.get("victimId"))
        if player is None or victim is None:
            return
        self._forward("entity_attack", {"player_id": player, "entity_id": victim,
            "attacker_known": True, "attack_started": True,
            "damage": None, "damage_known": False,
            "reason": "server PlayerAttackEntityEvent damage starts at 0; not final damage",
            "cancelled_at_observation": bool(args.get("cancel", False))})

    def _death(self, kind, args):
        entity, attacker = exact_id(args.get("id")), exact_id(args.get("attacker"))
        if entity:
            self._forward(kind, {"entity_id": entity, "attacker_id": attacker,
                "attacker_known": attacker is not None and attacker != "-1",
                "cause": args.get("cause"), "custom_tag": args.get("customTag")})

    def on_player_death(self, args):
        self._death("player_death", args)

    def on_mob_death(self, args):
        self._death("entity_death", args)

    def on_leave(self, args):
        self.subscribers.discard(exact_id(args.get("id")))

    def Destroy(self):
        for namespace, system, event, handler in self.bindings:
            self.UnListenForEvent(namespace, system, event, self, handler)
        self.bindings = []
        self.subscribers.clear()
