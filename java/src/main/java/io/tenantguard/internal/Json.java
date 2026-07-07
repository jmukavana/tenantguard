package io.tenantguard.internal;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
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

    /**
     * Parses an arbitrary JSON document into nested {@code Map<String,Object>} (objects),
     * {@code List<Object>} (arrays), {@code String}, {@code Long}/{@code Double} (numbers),
     * {@code Boolean}, or {@code null}. Complements {@link #parseObject} for responses whose
     * shape is not a flat string map, such as AuditForge's IngestAck.
     */
    public static Object parse(String s) {
        int[] pos = {0};
        Object value = parseValue(s, pos);
        skipWs(s, pos);
        if (pos[0] != s.length()) {
            throw new IllegalArgumentException("trailing data at index " + pos[0]);
        }
        return value;
    }

    private static Object parseValue(String s, int[] pos) {
        skipWs(s, pos);
        char c = peek(s, pos);
        return switch (c) {
            case '{' -> parseObjectValue(s, pos);
            case '[' -> parseArray(s, pos);
            case '"' -> parseString(s, pos);
            case 't', 'f' -> parseBoolean(s, pos);
            case 'n' -> {
                parseLiteral(s, pos, "null");
                yield null;
            }
            default -> parseNumber(s, pos);
        };
    }

    private static Map<String, Object> parseObjectValue(String s, int[] pos) {
        Map<String, Object> out = new LinkedHashMap<>();
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
            out.put(key, parseValue(s, pos));
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

    private static List<Object> parseArray(String s, int[] pos) {
        List<Object> out = new ArrayList<>();
        expect(s, pos, '[');
        skipWs(s, pos);
        if (peek(s, pos) == ']') {
            pos[0]++;
            return out;
        }
        while (true) {
            out.add(parseValue(s, pos));
            skipWs(s, pos);
            char c = next(s, pos);
            if (c == ']') {
                return out;
            }
            if (c != ',') {
                throw new IllegalArgumentException("expected ',' or ']' at index " + (pos[0] - 1));
            }
        }
    }

    private static Boolean parseBoolean(String s, int[] pos) {
        if (peek(s, pos) == 't') {
            parseLiteral(s, pos, "true");
            return Boolean.TRUE;
        }
        parseLiteral(s, pos, "false");
        return Boolean.FALSE;
    }

    private static void parseLiteral(String s, int[] pos, String literal) {
        if (!s.startsWith(literal, pos[0])) {
            throw new IllegalArgumentException("expected '" + literal + "' at index " + pos[0]);
        }
        pos[0] += literal.length();
    }

    private static Object parseNumber(String s, int[] pos) {
        int start = pos[0];
        while (pos[0] < s.length() && "+-0123456789.eE".indexOf(s.charAt(pos[0])) >= 0) {
            pos[0]++;
        }
        String num = s.substring(start, pos[0]);
        if (num.isEmpty()) {
            throw new IllegalArgumentException("expected a JSON value at index " + start);
        }
        boolean fractional = num.indexOf('.') >= 0 || num.indexOf('e') >= 0 || num.indexOf('E') >= 0;
        return fractional ? (Object) Double.parseDouble(num) : (Object) Long.parseLong(num);
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
