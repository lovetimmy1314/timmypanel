package model

import (
	"encoding/json"
	"testing"
)

func engineNames(s Settings) []string {
	out := make([]string, 0, len(s.Search.Engines))
	for _, e := range s.Search.Engines {
		out = append(out, e.Name)
	}
	return out
}

func hasEngine(s Settings, name string) bool {
	for _, n := range engineNames(s) {
		if n == name {
			return true
		}
	}
	return false
}

// 老数据里 engines 非空但没有 engineSeed：新增的内置引擎要补进去，已有的不能重复。
func TestDecodeSeedsNewBuiltinEngines(t *testing.T) {
	old := `{"search":{"enabled":true,"default":"local","engines":[
		{"name":"Google","url":"https://www.google.com/search?q=%s","icon":"mdi:google"},
		{"name":"Bing","url":"https://www.bing.com/search?q=%s","icon":"mdi:microsoft-bing"}
	]}}`
	got := (&Setting{Data: old}).Decode()

	if !hasEngine(got, "磁力搜索") {
		t.Fatalf("新增的内置引擎没补上，实际有：%v", engineNames(got))
	}
	seen := map[string]int{}
	for _, n := range engineNames(got) {
		seen[n]++
		if seen[n] > 1 {
			t.Fatalf("引擎 %q 重复了：%v", n, engineNames(got))
		}
	}
}

// 用户删掉内置引擎并保存过（engineSeed 已是当前值）：不能自己长回来。
func TestDecodeKeepsDeletedBuiltinDeleted(t *testing.T) {
	s := DefaultSettings()
	kept := s.Search.Engines[:0:0]
	for _, e := range s.Search.Engines {
		if e.Name != "磁力搜索" {
			kept = append(kept, e)
		}
	}
	s.Search.Engines = kept
	data, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}

	got := (&Setting{Data: data}).Decode()
	if hasEngine(got, "磁力搜索") {
		t.Fatalf("删掉的内置引擎又被补回来了：%v", engineNames(got))
	}
}

// language 是后加的字段，老数据里没有这个键，必须补成 zh 而不是留空——
// 留空的话前端拿到 ” 会挑不到词典。
func TestDecodeFillsMissingLanguage(t *testing.T) {
	got := (&Setting{Data: `{"theme":"dark"}`}).Decode()
	if got.Language != "zh" {
		t.Fatalf("老数据的 language = %q，期望 zh", got.Language)
	}
	if got := (&Setting{Data: `{"language":"en"}`}).Decode(); got.Language != "en" {
		t.Fatalf("language = %q，期望保留 en", got.Language)
	}
}

// engineSeed 只是记账字段，前端不发它——JSON 里缺席时必须当作 0 而不是继承默认值。
func TestDecodeTreatsMissingEngineSeedAsZero(t *testing.T) {
	var raw map[string]json.RawMessage
	data, err := Encode(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["engineSeed"]; !ok {
		t.Fatal("默认设置应当写出 engineSeed")
	}
	delete(raw, "engineSeed")
	stripped, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := (&Setting{Data: string(stripped)}).Decode().EngineSeed; got != 0 {
		t.Fatalf("缺席的 engineSeed 应解成 0，实际 %d", got)
	}
}
