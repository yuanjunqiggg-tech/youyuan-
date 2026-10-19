package org.youyuan.bridge;

import java.util.Map;
import java.util.Set;

/** Read-only module protocol. The provider must read the real server engine. */
public final class YouyuanBridgeProtocol {
    public interface Provider {
        Set<String> capabilities();
        Map<String, Object> query(String operation, String target);
        default Set<String> events() { return Set.of(); }
    }
    private final Provider provider;
    public YouyuanBridgeProtocol(Provider provider) { this.provider = provider; }
    public Map<String, Object> reply(String event, Map<String, Object> request) {
        Object id = request.get("request_id");
        if (!(id instanceof String text) || text.isBlank() || text.length() > 128)
            throw new IllegalArgumentException("request_id required");
        if ("Hello".equals(event))
            return Map.of("protocol", 1, "request_id", id, "capabilities", provider.capabilities(), "events", provider.events());
        if (!"Query".equals(event)) throw new IllegalArgumentException("unsupported event");
        Object op = request.get("query");
        if (!(op instanceof String name) || !provider.capabilities().contains(name))
            return Map.of("protocol", 1, "request_id", id, "ok", false, "error", "unsupported_query");
        String target = request.get("target") instanceof String t ? t : "";
        return Map.of("protocol", 1, "request_id", id, "ok", true, "result", provider.query(name, target));
    }
}
