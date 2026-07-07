package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Loads the shared, language-neutral compliance fixtures
 * (testkit/compliance-tests/fixtures) that define TenantGuard conformance across all
 * language bindings.
 */
final class Fixtures {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private Fixtures() {
    }

    /** Reads and parses the named fixture file (e.g. "tenant-context.json"). */
    static JsonNode load(String name) throws IOException {
        return MAPPER.readTree(dir().resolve(name).toFile());
    }

    /** Locates the fixtures directory by walking up from the working directory. */
    static Path dir() {
        Path start = Paths.get("").toAbsolutePath();
        for (Path p = start; p != null; p = p.getParent()) {
            Path candidate = p.resolve("testkit/compliance-tests/fixtures");
            if (Files.isDirectory(candidate)) {
                return candidate;
            }
        }
        throw new IllegalStateException(
                "could not locate testkit/compliance-tests/fixtures from " + start);
    }

    /** Returns the text of a field, or "" when absent or JSON null. */
    static String text(JsonNode node, String field) {
        JsonNode v = node.get(field);
        return v == null || v.isNull() ? "" : v.asText();
    }

    /** Reads a JSON object into a String->String map ({@code null}/absent yields empty). */
    static Map<String, String> map(JsonNode node) {
        Map<String, String> out = new HashMap<>();
        if (node != null && node.isObject()) {
            node.fields().forEachRemaining(e -> out.put(e.getKey(), e.getValue().asText()));
        }
        return out;
    }

    /** Reads a JSON array into a list of strings ({@code null}/absent yields empty). */
    static List<String> list(JsonNode node) {
        List<String> out = new ArrayList<>();
        if (node != null && node.isArray()) {
            node.forEach(n -> out.add(n.asText()));
        }
        return out;
    }
}
