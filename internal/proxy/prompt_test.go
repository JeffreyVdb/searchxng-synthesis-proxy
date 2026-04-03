package proxy

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestStripFences
// ---------------------------------------------------------------------------

func TestStripFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain json unchanged",
			input: `{"answer":"x","citations":[1]}`,
			want:  `{"answer":"x","citations":[1]}`,
		},
		{
			name:  "triple-backtick fenced json",
			input: "```json\n{\"answer\":\"x\",\"citations\":[1]}\n```",
			want:  `{"answer":"x","citations":[1]}`,
		},
		{
			name:  "fenced with trailing whitespace",
			input: "```json\n{\"a\":1}\n```   ",
			want:  `{"a":1}`,
		},
		{
			name:  "CRLF line endings",
			input: "```json\r\n{\"a\":1}\r\n```",
			want:  `{"a":1}`,
		},
		{
			name:  "uppercase fence tag JSON",
			input: "```JSON\n{\"a\":1}\n```",
			want:  `{"a":1}`,
		},
		{
			name:  "space before info string",
			input: "``` json\n{\"a\":1}\n```",
			want:  `{"a":1}`,
		},
		{
			name:  "tilde fence json",
			input: "~~~json\n{\"a\":1}\n~~~",
			want:  `{"a":1}`,
		},
		{
			name:  "BOM before fence",
			input: "\ufeff```json\n{\"a\":1}\n```",
			want:  `{"a":1}`,
		},
		{
			name:  "leading and trailing blank lines",
			input: "\n\n```json\n{\"a\":1}\n```\n\n",
			want:  `{"a":1}`,
		},
		{
			name:  "incomplete fence returns original",
			input: "```json\n{\"a\":1}",
			want:  "```json\n{\"a\":1}",
		},
		{
			name:  "bare backtick fence without info string",
			input: "```\n{\"a\":1}\n```",
			want:  `{"a":1}`,
		},
		{
			name:  "4-backtick fence",
			input: "````json\n{\"a\":1}\n````",
			want:  `{"a":1}`,
		},
		{
			name:  "tilde fence 3 chars",
			input: "~~~\n{\"a\":1}\n~~~",
			want:  `{"a":1}`,
		},
		{
			name:  "non-json info string returns original",
			input: "```python\nprint('hi')\n```",
			want:  "```python\nprint('hi')\n```",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripFences(tc.input)
			if got != tc.want {
				t.Errorf("stripFences() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExtractFirstJSONObject
// ---------------------------------------------------------------------------

func TestExtractFirstJSONObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantBool bool
	}{
		{
			name:     "pure json object",
			input:    `{"answer":"x","citations":[1]}`,
			want:     `{"answer":"x","citations":[1]}`,
			wantBool: true,
		},
		{
			name:     "preamble and trailing note",
			input:    `Here is the result: {"answer":"x","citations":[1]} Hope that helps!`,
			want:     `{"answer":"x","citations":[1]}`,
			wantBool: true,
		},
		{
			name:     "nested objects and arrays",
			input:    `{"a":{"b":[1,2]},"c":[]}`,
			want:     `{"a":{"b":[1,2]},"c":[]}`,
			wantBool: true,
		},
		{
			name:     "braces inside strings",
			input:    `{"answer":"use map[string]int { key: val }","citations":[1]}`,
			want:     `{"answer":"use map[string]int { key: val }","citations":[1]}`,
			wantBool: true,
		},
		{
			name:     "escaped quotes inside strings",
			input:    `{"answer":"he said \"hello {world}\"","citations":[1]}`,
			want:     `{"answer":"he said \"hello {world}\"","citations":[1]}`,
			wantBool: true,
		},
		{
			name:     "no object present",
			input:    `just some text`,
			want:     "",
			wantBool: false,
		},
		{
			name:     "unbalanced object",
			input:    `{"a": {"b": 1}`,
			want:     "",
			wantBool: false,
		},
		{
			name:     "multiple objects returns first",
			input:    `{"a":1} {"b":2}`,
			want:     `{"a":1}`,
			wantBool: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractFirstJSONObject(tc.input)
			if ok != tc.wantBool {
				t.Errorf("extractFirstJSONObject() ok = %v, want %v", ok, tc.wantBool)
			}
			if got != tc.want {
				t.Errorf("extractFirstJSONObject() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestParseSynthesisJSON
// ---------------------------------------------------------------------------

func TestParseSynthesisJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantAns string
		wantErr bool
	}{
		{
			name:    "clean json parses",
			input:   `{"answer":"context is used for cancellation","citations":[1,2]}`,
			wantAns: "context is used for cancellation",
			wantErr: false,
		},
		{
			name:    "fenced json parses",
			input:   "```json\n{\"answer\":\"ok\",\"citations\":[1]}\n```",
			wantAns: "ok",
			wantErr: false,
		},
		{
			name:    "BOM plus fenced json parses",
			input:   "\ufeff```json\n{\"answer\":\"ok\",\"citations\":[1]}\n```",
			wantAns: "ok",
			wantErr: false,
		},
		{
			name:    "prose-wrapped json parses via fallback",
			input:   "Here is the result:\n{\"answer\":\"see sources\",\"citations\":[1]}\nHope that helps!",
			wantAns: "see sources",
			wantErr: false,
		},
		{
			name:    "fenced json with extra prose outside parses via fallback",
			input:   "Sure!\n```json\n{\"answer\":\"ok\",\"citations\":[1]}\n```\nLet me know if you need more.",
			wantAns: "ok",
			wantErr: false,
		},
		{
			name:    "empty answer returns missing answer error",
			input:   `{"answer":"   ","citations":[1]}`,
			wantAns: "",
			wantErr: true,
		},
		{
			name:    "completely non-JSON returns error",
			input:   "This is just plain text with no JSON at all.",
			wantAns: "",
			wantErr: true,
		},
		{
			name:    "tilde fenced json parses",
			input:   "~~~json\n{\"answer\":\"tilde works\",\"citations\":[2]}\n~~~",
			wantAns: "tilde works",
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := parseSynthesisJSON(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got payload: %+v", payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if payload.Answer != tc.wantAns {
				t.Errorf("Answer = %q, want %q", payload.Answer, tc.wantAns)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestTrimLLMOutput
// ---------------------------------------------------------------------------

func TestTrimLLMOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no bom", `hello`, `hello`},
		{"bom prefix", "\ufeffhello", "hello"},
		{"bom plus whitespace", "\ufeff  hello  ", "hello"},
		{"whitespace only", "  \t  ", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := trimLLMOutput(tc.input)
			if got != tc.want {
				t.Errorf("trimLLMOutput() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestParseFenceLine
// ---------------------------------------------------------------------------

func TestParseFenceLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantChar   byte
		wantLen    int
		wantInfo   string
	}{
		{"empty", "", 0, 0, ""},
		{"plain text", "hello", 0, 0, ""},
		{"three backticks", "```", '`', 3, ""},
		{"backticks json", "```json", '`', 3, "json"},
		{"backticks space json", "``` json", '`', 3, " json"},
		{"four backticks", "````", '`', 4, ""},
		{"three tildes", "~~~", '~', 3, ""},
		{"tildes json", "~~~json", '~', 3, "json"},
		{"uppercase JSON", "```JSON", '`', 3, "JSON"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch, n, info := parseFenceLine(tc.line)
			if ch != tc.wantChar {
				t.Errorf("char = %c, want %c", ch, tc.wantChar)
			}
			if n != tc.wantLen {
				t.Errorf("len = %d, want %d", n, tc.wantLen)
			}
			if info != tc.wantInfo {
				t.Errorf("info = %q, want %q", info, tc.wantInfo)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSplitLines
// ---------------------------------------------------------------------------

func TestSplitLines(t *testing.T) {
	input := "a\r\nb\nc\r\n"
	got := splitLines(input)
	want := []string{"a", "b", "c", ""}
	if len(got) != len(want) {
		t.Fatalf("splitLines() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("splitLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// TestBuildUserPrompt
// ---------------------------------------------------------------------------

func TestBuildUserPrompt_ContainsNumberedSources(t *testing.T) {
	results := []sourceForResult{
		{index: 1, title: "Go Blog", url: "https://go.dev/blog", snippet: "Learn Go", engine: "google"},
		{index: 2, title: "Go Tour", url: "https://go.dev/tour", snippet: "Interactive tour", engine: "duckduckgo"},
	}
	prompt := buildUserPrompt("golang", results)

	if !strings.Contains(prompt, "[1]") {
		t.Error("prompt missing [1]")
	}
	if !strings.Contains(prompt, "[2]") {
		t.Error("prompt missing [2]")
	}
	if !strings.Contains(prompt, "golang") {
		t.Error("prompt missing query")
	}
	if !strings.Contains(prompt, "https://go.dev/blog") {
		t.Error("prompt missing URL")
	}
	if !strings.Contains(prompt, "Learn Go") {
		t.Error("prompt missing snippet")
	}
}
