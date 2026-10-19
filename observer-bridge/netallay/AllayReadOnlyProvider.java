package org.youyuan.bridge;
import org.allaymc.api.server.Server;
import java.util.ArrayList;
import java.util.Map;
import java.util.Set;

/** Minimal actual engine provider; do not advertise unsupported entity queries. */
public final class AllayReadOnlyProvider implements YouyuanBridgeProtocol.Provider {
    public Set<String> capabilities() { return Set.of("player_online_list", "player_health"); }
    public Set<String> events() { return Set.of("swing", "punch_air", "health_changed", "entity_hurt", "entity_death", "session_join", "session_leave"); }
    public Map<String, Object> query(String operation, String target) {
        if ("player_health".equals(operation)) {
            for (var player : Server.getInstance().getPlayerManager().getPlayers().values()) {
                if (player.isConnected() && player.getOriginName().equals(target)) {
                    var entity = player.getControlledEntity();
                    if (entity == null) return Map.of("known", false, "player", target, "source", "allay_player_manager");
                    return Map.of("known", true, "player", target, "current", entity.getHealth(), "max", entity.getMaxHealth(), "source", "allay_entity_component");
                }
            }
            return Map.of("known", false, "player", target, "reason", "player_not_connected");
        }
        if (!"player_online_list".equals(operation)) throw new IllegalArgumentException("unsupported_query");
        var names = new ArrayList<String>();
        for (var player : Server.getInstance().getPlayerManager().getPlayers().values()) {
            if (player.isConnected()) names.add(player.getOriginName());
        }
        return Map.of("players", names, "count", names.size(), "source", "allay_player_manager");
    }
}
