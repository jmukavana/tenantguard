package io.tenantguard.internal;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.TreeMap;

/**
 * Minimal, dependency-free JSON for a flat {@code String->String} object. Used to encode
 * and decode tenant metadata carried in W3C Baggage, so the core needs no JSON library.
 * Keys are emitted in sorted order for deterministic, cross-language-comparable output.
 *
 * <p>Not a general JSON implementation: only flat objects of string values are supported.
 */
public final class Json {

    private Json() {
    }

    /** Serializes a flat string map to a JSON object with sorted keys and no whitespace. */
    public static String objectOf(Map<String, String> map) {
        StringBuilder sb = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, String> e : new TreeMap<>(map).entrySet()) {
            if (!first) {
                sb.append(',');
            }
            first = false;
            sb.append(quote(e.getKey())).append(':').append(quote(e.getValue()));
        }
        return sb.append('}').toString();
    }

    /** Parses a flat JSON object of string values (the inverse of {@link #objectOf}). */
    public static Map<String, String> parseObject(String s) {
        Map<String, String> out = new LinkedHashMap<>();
        int[] pos = {0};
        skipWs(s, pos);
        expect(s, pos, '{');
        skipWs(s, pos);
        if (peek(s, pos) == '}') {
            pos[0]++;
            return out;
        }
        while (true) {
            skipWs(s, pos);
            String key = parseString(s, pos);
            skipWs(s, pos);
            expect(s, pos, ':');
            skipWs(s, pos);
            out.put(key, parseString(s, pos));
            skipWs(s, pos);
            char c = next(s, pos);
            if (c == '}') {
                return out;
            }
            if (c != ',') {
                throw new IllegalArgumentException("expected ',' or '}' at index " + (pos[0] - 1));
            }
        }
    }

    /** Returns {@code s} as a quoted, escaped JSON string. */
    public static String quote(String s) {
        StringBuilder sb = new StringBuilder(s.length() + 2).append('"');
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                default -> {
                    if (c < 0x20) {
                        sb.append(String.format("\\u%04x", (int) c));
                    } else {
                        sb.append(c);
                    }
                }
            }
        }
        return sb.append('"').toString();
    }

    private static String parseString(String s, int[] pos) {
        expect(s, pos, '"');
        StringBuilder sb = new StringBuilder();
        while (true) {
            if (pos[0] >= s.length()) {
                throw new IllegalArgumentException("unterminated string");
            }
            char c = s.charAt(pos[0]++);
            if (c == '"') {
                return sb.toString();
            }
            if (c != '\\') {
                sb.append(c);
                continue;
            }
            char e = next(s, pos);
            switch (e) {
                case '"' -> sb.append('"');
                case '\\' -> sb.append('\\');
                case '/' -> sb.append('/');
                case 'b' -> sb.append('\b');
                case 'f' -> sb.append('\f');
                case 'n' -> sb.append('\n');
                case 'r' -> sb.append('\r');
                case 't' -> sb.append('\t');
                case 'u' -> {
                    sb.append((char) Integer.parseInt(s.substring(pos[0], pos[0] + 4), 16));
                    pos[0] += 4;
                }
                default -> throw new IllegalArgumentException("bad escape \\" + e);
            }
        }
    }

    private static void skipWs(String s, int[] pos) {
        while (pos[0] < s.length() && Character.isWhitespace(s.charAt(pos[0]))) {
            pos[0]++;
        }
    }

    private static char peek(String s, int[] pos) {
        return pos[0] < s.length() ? s.charAt(pos[0]) : '\0';
    }

    private static char next(String s, int[] pos) {
        if (pos[0] >= s.length()) {
            throw new IllegalArgumentException("unexpected end of input");
        }
        return s.charAt(pos[0]++);
    }

    private static void expect(String s, int[] pos, char c) {
        char got = next(s, pos);
        if (got != c) {
            throw new IllegalArgumentException("expected '" + c + "' but got '" + got + "'");
        }
    }
}
