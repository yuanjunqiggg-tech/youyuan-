package org.youyuan.bridge;

import org.allaymc.api.entity.Entity;
import org.allaymc.api.entity.interfaces.EntityLiving;
import org.allaymc.api.eventbus.EventHandler;
import org.allaymc.api.eventbus.event.entity.EntityHealthChangeEvent;
import org.allaymc.api.eventbus.event.entity.EntityDamageEvent;
import org.allaymc.api.eventbus.event.entity.EntityDieEvent;
import org.allaymc.api.eventbus.event.server.PlayerJoinEvent;
import org.allaymc.api.eventbus.event.server.PlayerQuitEvent;
import org.allaymc.api.eventbus.event.player.PlayerSwingArmEvent;
import org.allaymc.api.eventbus.event.player.PlayerPunchAirEvent;
import org.allaymc.api.plugin.Plugin;
import org.allaymc.api.server.Server;
import org.allaymc.netallay.NetAllay;
import java.nio.file.Files;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;
import java.util.Properties;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;

/** Optional Allay server module. No inference of the nearest player as attacker. */
public final class YouyuanObserverPlugin extends Plugin {
    private NetAllayObserverAdapter bridge;
    private final AtomicLong sequence = new AtomicLong();
    private final String session = UUID.randomUUID().toString();

    @Override public void onEnable() {
        try {
            var folder = getPluginContainer().dataFolder();
            Files.createDirectories(folder);
            var file = folder.resolve("observer.properties");
            if (!Files.exists(file)) Files.writeString(file,
                    "# Comma-separated authenticated bot UUIDs. Empty disables access.\nallowed_uuids=\n");
            var config = new Properties();
            try (var stream = Files.newInputStream(file)) { config.load(stream); }
            Set<String> ids = Arrays.stream(config.getProperty("allowed_uuids", "").split(","))
                    .map(String::trim).filter(s -> !s.isEmpty()).collect(Collectors.toUnmodifiableSet());
            var api = NetAllay.getInstance();
            if (api == null) throw new IllegalStateException("Enable NetAllay before YouyuanObserver");
            bridge = new NetAllayObserverAdapter(api, player -> player.isConnected()
                    && player.getLoginData().isAuthed()
                    && ids.contains(player.getLoginData().getUuid().toString()), new AllayReadOnlyProvider());
            Server.getInstance().getEventBus().registerListener(this);
        } catch (Exception error) { throw new IllegalStateException("Observer module did not start", error); }
    }
    @Override public void onDisable() {
        Server.getInstance().getEventBus().unregisterListener(this);
        if (bridge != null) bridge.close();
        bridge = null;
    }
    private Map<String,Object> entity(Entity entity) {
        var data = new HashMap<String,Object>();
        data.put("runtime_id", Long.toString(entity.getRuntimeId()));
        data.put("entity_id", Long.toString(entity.getRuntimeId()));
        data.put("uuid", entity.getUniqueId().toString());
        data.put("name", entity.getDisplayName());
        if (entity.getDimension() != null) {
            data.put("dimension", entity.getDimension().getDimensionType().getId());
            data.put("world", entity.getWorld().getName());
        }
        return data;
    }
    private void forward(String type, Map<String,Object> data) {
        data.put("event_id", session + ":" + sequence.incrementAndGet());
        data.put("timestamp_ms", System.currentTimeMillis());
        data.put("server_tick", Server.getInstance().getTick());
        data.put("stage", "engine_event_observed; later listeners may still change cancellable events");
        if (bridge != null) bridge.forwardEngineEvent(type, data);
    }
    @EventHandler public void health(EntityHealthChangeEvent event) {
        if (event.isCancelled()) return;
        if (!(event.getEntity() instanceof EntityLiving target)) return;
        var dimension = target.getDimension();
        if (dimension == null) return;
        final float previous = event.getOldHealth();
        final long eventTime = System.currentTimeMillis();
        // The cancellable event carries a proposed value. Read the real
        // component on its dimension thread after this event chain completes.
        dimension.getScheduler().runLater(this, () -> {
            var data = entity(target);
            data.put("health", Map.of("current", target.getHealth(), "max", target.getMaxHealth(), "previous", previous));
            data.put("event_observed_at", eventTime);
            data.put("measurement", "actual_engine_component_after_event; may include intervening changes");
            forward("health_changed", data);
        });
    }
    @EventHandler public void swing(PlayerSwingArmEvent event) {
        var data = entity(event.getPlayer());
        data.put("player_id", event.getPlayer().getUniqueId().toString());
        data.put("input_kind", "arm_swing; hit_or_miss_not_inferred");
        forward("swing", data);
    }
    @EventHandler public void punchAir(PlayerPunchAirEvent event) {
        if (event.isCancelled()) return;
        var data = entity(event.getPlayer());
        data.put("player_id", event.getPlayer().getUniqueId().toString());
        data.put("input_kind", "engine_punch_air");
        forward("punch_air", data);
    }
    @EventHandler public void hurt(EntityDamageEvent event) {
        if (event.isCancelled()) return;
        var data = entity(event.getEntity()); var damage = event.getDamageContainer();
        data.put("damage", damage.getFinalDamage()); data.put("damage_type", damage.getDamageType().toString());
        Object attacker = damage.getAttacker();
        data.put("attacker_known", attacker instanceof Entity);
        if (attacker instanceof Entity source) data.put("attacker", entity(source));
        forward("entity_hurt", data);
    }
    @EventHandler public void death(EntityDieEvent event) { forward("entity_death", entity(event.getEntity())); }
    @EventHandler public void join(PlayerJoinEvent event) {
        if (!event.isCancelled()) forward("session_join", new HashMap<>(Map.of("player", event.getPlayer().getOriginName())));
    }
    @EventHandler public void quit(PlayerQuitEvent event) {
        bridge.removePlayer(event.getPlayer());
        forward("session_leave", new HashMap<>(Map.of("player", event.getPlayer().getOriginName())));
    }
}
