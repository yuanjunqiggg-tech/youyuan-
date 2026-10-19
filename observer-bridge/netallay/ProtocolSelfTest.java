package org.youyuan.bridge;
import java.util.Map;
import java.util.Set;
public final class ProtocolSelfTest {
    public static void main(String[] args) {
        final int[] calls = {0};
        var protocol = new YouyuanBridgeProtocol(new YouyuanBridgeProtocol.Provider() {
            public Set<String> capabilities() { return Set.of("test_lookup"); }
            public Map<String, Object> query(String op, String target) {
                calls[0]++; return Map.of("test_target", target);
            }
        });
        if (!protocol.reply("Hello", Map.of("request_id", "a")).get("request_id").equals("a")) throw new AssertionError();
        if (!protocol.reply("Query", Map.of("request_id", "b", "query", "unknown")).get("ok").equals(false) || calls[0] != 0) throw new AssertionError();
        if (!protocol.reply("Query", Map.of("request_id", "c", "query", "test_lookup", "target", "123")).get("ok").equals(true) || calls[0] != 1) throw new AssertionError();
        try { protocol.reply("Query", Map.of("query", "test_lookup")); throw new AssertionError(); }
        catch (IllegalArgumentException expected) { }
        System.out.println("PASS: correlation, capability discovery, unsupported query, provider invocation, missing request rejection; mock provider only");
    }
}
