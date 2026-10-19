package org.youyuan.bridge;

import org.allaymc.api.player.Player;
import org.allaymc.netallay.NetAllay;
import org.allaymc.netallay.PyRpcHandler;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Predicate;

/** Install inside an authorised Allay server plugin; not inside a spectator bot. */
public final class NetAllayObserverAdapter implements AutoCloseable {
    private static final String NS = "YouyuanBridge", SYSTEM = "ObserverV1";
    private final NetAllay api;
    private final Predicate<Player> allowed;
    private final YouyuanBridgeProtocol protocol;
    private final Set<Player> observers = ConcurrentHashMap.newKeySet();
    private final PyRpcHandler hello, query;

    public NetAllayObserverAdapter(NetAllay api, Predicate<Player> allowed,
                                  YouyuanBridgeProtocol.Provider provider) {
        this.api = api;
        this.allowed = allowed;
        this.protocol = new YouyuanBridgeProtocol(provider);
        hello = (player, data) -> respond(player, "Hello", "Capabilities", data);
        query = (player, data) -> respond(player, "Query", "Reply", data);
        api.listenForEvent(NS, SYSTEM, "Hello", hello);
        api.listenForEvent(NS, SYSTEM, "Query", query);
    }

    private void respond(Player player, String event, String reply, Map<String, Object> request) {
        if (!allowed.test(player)) return;
        try {
            Map<String, Object> result = protocol.reply(event, request);
            api.notifyToClient(player, NS, SYSTEM, reply, result);
            if ("Hello".equals(event)) observers.add(player);
        } catch (IllegalArgumentException ignored) {
            // Malformed input must not enable a subscription or invoke a provider.
        }
    }

    /** Caller supplies an actual engine event, never nearest-player inference. */
    public int forwardEngineEvent(String eventType, Map<String, Object> data) {
        observers.removeIf(player -> !allowed.test(player));
        return api.notifyToMultiClients(observers, NS, SYSTEM, "Observation",
                Map.of("protocol", 1, "event_type", eventType, "source", "server_module", "data", data));
    }
    public void removePlayer(Player player) { observers.remove(player); }
    @Override public void close() {
        api.unlistenForEvent(NS, SYSTEM, "Hello", hello);
        api.unlistenForEvent(NS, SYSTEM, "Query", query);
        observers.clear();
    }
}
