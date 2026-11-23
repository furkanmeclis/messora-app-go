package segment

import (
	"encoding/json"
	"strings"
	"testing"
)

func compile(t *testing.T, def string) Compiled {
	t.Helper()
	n, err := Parse([]byte(def))
	if err != nil {
		t.Fatalf("%s: %v", def, err)
	}
	c, err := Compile(n, 1)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCompileTable(t *testing.T) {
	cases := []struct {
		def  string
		sql  string
		args int
	}{
		{`{"field":"tag","operator":"has","value":"vip"}`, "EXISTS (SELECT 1 FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id WHERE ct.contact_id = c.id AND lower(t.name) = lower($1))", 1},
		{`{"field":"var1","operator":"eq","value":"Istanbul"}`, "lower(c.var1) = lower($1)", 1},
		{`{"field":"name","operator":"contains","value":"50%_off"}`, `c.name ILIKE $1 ESCAPE '\'`, 1},
		{`{"field":"email","operator":"is_not_set"}`, "(c.email::text IS NULL OR c.email::text = '')", 0},
		{`{"field":"opted_out","operator":"is_false"}`, "(NOT (c.opted_out_at IS NOT NULL))", 0},
		{`{"field":"last_inbound_at","operator":"within_days","value":30}`, "c.last_inbound_at >= now() - make_interval(days => $1::int)", 1},
		{`{"field":"created_at","operator":"after","value":"2026-01-31"}`, "c.created_at > $1", 1},
		{`{"op":"and","rules":[{"field":"tag","operator":"has_any","value":["A","b"]},{"not":true,"op":"or","rules":[{"field":"phone","operator":"starts_with","value":"+90"},{"field":"var2","operator":"in","value":["x","y"]}]}]}`, "", 3},
		{`{"field":"group","operator":"in_group","value":"0190a1b2-0000-7000-8000-000000000001"}`, "EXISTS (SELECT 1 FROM contact_group_members gm WHERE gm.contact_id = c.id AND gm.group_id = $1::uuid)", 1},
	}
	for _, tc := range cases {
		c := compile(t, tc.def)
		if tc.sql != "" && c.SQL != tc.sql {
			t.Errorf("%s\n got %s\nwant %s", tc.def, c.SQL, tc.sql)
		}
		if len(c.Args) != tc.args {
			t.Errorf("%s: %d args, want %d", tc.def, len(c.Args), tc.args)
		}
	}
	c := compile(t, `{"field":"name","operator":"contains","value":"50%_off"}`)
	if c.Args[0] != `%50\%\_off%` {
		t.Errorf("LIKE wildcards not escaped: %q", c.Args[0])
	}
	c = compile(t, `{"op":"and","rules":[{"field":"tag","operator":"has_any","value":["A","b"]},{"field":"var1","operator":"eq","value":"x"}]}`)
	if !strings.Contains(c.SQL, "$2") || strings.Contains(c.SQL, "$3") {
		t.Errorf("placeholder numbering: %s", c.SQL)
	}
	off, _ := Compile(&Node{Field: "var1", Operator: "eq", Value: json.RawMessage(`"x"`)}, 7)
	if off.SQL != "lower(c.var1) = lower($7)" {
		t.Errorf("offset ignored: %s", off.SQL)
	}
}

func TestRejectsInvalidDefinitions(t *testing.T) {
	deep := `{"field":"var1","operator":"eq","value":"x"}`
	for range MaxDepth {
		deep = `{"op":"and","rules":[` + deep + `]}`
	}
	many := make([]string, MaxRules+1)
	for i := range many {
		many[i] = `{"field":"var1","operator":"eq","value":"x"}`
	}
	bad := []string{
		``, `null`, `[]`, `{}`,
		`{"field":"password_hash","operator":"eq","value":"x"}`,
		`{"field":"name; DROP TABLE contacts","operator":"eq","value":"x"}`,
		`{"field":"name","operator":"eq) OR 1=1 --","value":"x"}`,
		`{"field":"name","operator":"before","value":"2026-01-01"}`,
		`{"field":"name","operator":"eq","value":5}`,
		`{"field":"name","operator":"eq","value":""}`,
		`{"field":"name","operator":"eq","value":"` + strings.Repeat("a", MaxTextLen+1) + `"}`,
		`{"field":"name","operator":"is_set","value":"x"}`,
		`{"field":"name","operator":"in","value":[]}`,
		`{"field":"last_inbound_at","operator":"within_days","value":0}`,
		`{"field":"created_at","operator":"after","value":"yesterday"}`,
		`{"field":"group","operator":"in_group","value":"' OR 1=1 --"}`,
		`{"op":"xor","rules":[{"field":"var1","operator":"eq","value":"x"}]}`,
		`{"op":"and","rules":[]}`,
		`{"op":"and","field":"var1","rules":[{"field":"var1","operator":"eq","value":"x"}]}`,
		`{"field":"var1","operator":"eq","value":"x","extra":1}`,
		deep,
		`{"op":"or","rules":[` + strings.Join(many, ",") + `]}`,
	}
	for _, def := range bad {
		if _, err := Parse([]byte(def)); err == nil {
			t.Errorf("accepted %.120s", def)
		} else if !IsInvalid(err) {
			t.Errorf("%.60s: error %v is not a definition error", def, err)
		}
	}
}

// User-supplied strings must only ever appear as bound arguments.
func TestValuesNeverReachSQL(t *testing.T) {
	payloads := []string{`'; DROP TABLE contacts; --`, `") OR 1=1 --`, `\x00`, `%' OR '1'='1`, `c.id) OR (true`}
	for _, p := range payloads {
		v, _ := json.Marshal(p)
		for _, def := range []string{
			`{"field":"name","operator":"eq","value":` + string(v) + `}`,
			`{"field":"var3","operator":"contains","value":` + string(v) + `}`,
			`{"field":"tag","operator":"has","value":` + string(v) + `}`,
			`{"field":"phone","operator":"in","value":[` + string(v) + `]}`,
		} {
			n, err := Parse([]byte(def))
			if err != nil {
				continue
			}
			c, _ := Compile(n, 1)
			if strings.Contains(c.SQL, p) || strings.Contains(c.SQL, "DROP") || strings.Contains(c.SQL, "1=1") {
				t.Fatalf("value leaked into SQL: %s", c.SQL)
			}
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(`{"field":"tag","operator":"has","value":"vip"}`)
	f.Add(`{"op":"or","not":true,"rules":[{"field":"var1","operator":"in","value":["a","b"]},{"field":"created_at","operator":"before","value":"2026-01-01"}]}`)
	f.Add(`{"field":"last_inbound_at","operator":"older_than_days","value":7}`)
	f.Fuzz(func(t *testing.T, def string) {
		n, err := Parse([]byte(def))
		if err != nil {
			if !IsInvalid(err) {
				t.Fatalf("unexpected error type: %v", err)
			}
			return
		}
		c, err := Compile(n, 1)
		if err != nil {
			t.Fatalf("parsed but did not compile: %v", err)
		}
		// Every placeholder has an argument and vice versa.
		for i := 1; i <= len(c.Args); i++ {
			if !strings.Contains(c.SQL, "$"+itoa(i)) {
				t.Fatalf("missing $%d in %s", i, c.SQL)
			}
		}
		if strings.Contains(c.SQL, "$"+itoa(len(c.Args)+1)) {
			t.Fatalf("placeholder without argument in %s", c.SQL)
		}
		// Only the fixed vocabulary can appear: no quotes besides the
		// constant ones the compiler emits.
		stripped := strings.NewReplacer(`ESCAPE '\'`, "", `''`, "").Replace(c.SQL)
		if strings.ContainsAny(stripped, `'";`) {
			t.Fatalf("unexpected quote in %s", c.SQL)
		}
	})
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }
