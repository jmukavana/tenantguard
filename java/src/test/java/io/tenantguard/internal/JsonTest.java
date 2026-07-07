package io.tenantguard.internal;

import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNull;

class JsonTest {

    @Test
    void objectOfSortsKeysAndOmitsWhitespace() {
        Map<String, String> m = new LinkedHashMap<>();
        m.put("z", "1");
        m.put("a", "2");
        assertEquals("{\"a\":\"2\",\"z\":\"1\"}", Json.objectOf(m));
    }

    @Test
    void singleKeyMatchesFixtureEncoding() {
        assertEquals("{\"region\":\"ke\"}", Json.objectOf(Map.of("region", "ke")));
    }

    @Test
    void roundTripWithEscapes() {
        Map<String, String> m = new LinkedHashMap<>();
        m.put("quote", "a\"b");
        m.put("back\\slash", "c\td\ne");
        m.put("unicode", "x");
        assertEquals(m, Json.parseObject(Json.objectOf(m)));
    }

    @Test
    void parsesEmptyObject() {
        assertEquals(Map.of(), Json.parseObject("{}"));
    }

    @Test
    @SuppressWarnings("unchecked")
    void parseHandlesNestedIngestAckShape() {
        Object root = Json.parse(
                "{\"received\":2,\"accepted\":[\"e1\"],\"duplicate\":[],"
                        + "\"rejected\":[{\"eventId\":\"e2\",\"reason\":\"invalid_signature\"}]}");

        Map<String, Object> obj = assertInstanceOf(Map.class, root);
        assertEquals(2L, obj.get("received"));
        assertEquals(List.of("e1"), obj.get("accepted"));
        assertEquals(List.of(), obj.get("duplicate"));

        List<Object> rejected = (List<Object>) obj.get("rejected");
        Map<String, Object> first = (Map<String, Object>) rejected.get(0);
        assertEquals("e2", first.get("eventId"));
        assertEquals("invalid_signature", first.get("reason"));
    }

    @Test
    void parseHandlesScalarsAndWhitespace() {
        assertEquals(List.of(1L, true, false), Json.parse(" [ 1 , true , false ] "));
        assertNull(((Map<?, ?>) Json.parse("{\"k\":null}")).get("k"));
    }
}
