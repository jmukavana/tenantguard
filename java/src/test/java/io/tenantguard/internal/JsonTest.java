package io.tenantguard.internal;

import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;

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
}
