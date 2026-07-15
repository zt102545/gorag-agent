package vectorstore

import (
	"strings"
	"testing"
)

func TestBuildTextQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "blank", query: "  ", want: ""},
		{name: "Chinese sentence", query: " 退款什么时候到账？ ", want: `退款什么时候到账？`},
		{name: "Redis syntax", query: `order(123): failed`, want: `order\(123\)\: failed`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTextQuery(tt.query); got != tt.want {
				t.Fatalf("buildTextQuery(%q)=%q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestBuildRelaxedTextQuery_ChineseTrigrams(t *testing.T) {
	got := buildRelaxedTextQuery("单分镜改写功能有什么用？")
	for _, want := range []string{"@question:(", "单分镜", "分镜改", "有什么", "什么用", "|"} {
		if !strings.Contains(got, want) {
			t.Fatalf("relaxed query %q does not contain %q", got, want)
		}
	}
	if got := buildRelaxedTextQuery("refund status"); got != "" {
		t.Fatalf("non-Chinese query should not be expanded: %q", got)
	}
}

func TestMergeTextRecords_StrictResultsStayFirstAndResultsAreDeduplicated(t *testing.T) {
	got := mergeTextRecords(
		[]Record{{ID: "strict"}, {ID: "both"}},
		[]Record{{ID: "relaxed"}, {ID: "both"}},
		3,
	)
	if len(got) != 3 || got[0].ID != "strict" || got[1].ID != "both" || got[2].ID != "relaxed" {
		t.Fatalf("unexpected merged records: %+v", got)
	}
}
