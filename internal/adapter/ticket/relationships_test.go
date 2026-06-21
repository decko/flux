package ticket

import (
	"reflect"
	"testing"

	"github.com/decko/flux/internal/model"
)

func TestParseRelationships_BareReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		labels []string
		self   string
		want   []model.Relationship
	}{
		{
			name: "multiple bare references",
			body: "Related to #123 and also #456",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "123"},
				{Type: model.RelationRelatesTo, TargetID: "456"},
			},
		},
		{
			name: "single bare reference",
			body: "See #789",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "789"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, tt.labels, tt.self)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_ClosesKeyword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "closes keyword",
			body: "closes #10",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "10"},
			},
		},
		{
			name: "fixes keyword",
			body: "fixes #20",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "20"},
			},
		},
		{
			name: "resolves keyword",
			body: "resolves #30",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "30"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_BlocksKeyword(t *testing.T) {
	t.Parallel()

	body := "This change blocks #5"
	got := ParseRelationships(body, nil, "")
	want := []model.Relationship{
		{Type: model.RelationBlocks, TargetID: "5"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseRelationships() = %v, want %v", got, want)
	}
}

func TestParseRelationships_BlockedByKeyword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "blocked by phrase",
			body: "blocked by #7",
			want: []model.Relationship{
				{Type: model.RelationBlockedBy, TargetID: "7"},
			},
		},
		{
			name: "depends on phrase",
			body: "depends on #8",
			want: []model.Relationship{
				{Type: model.RelationBlockedBy, TargetID: "8"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_ParentChild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "parent of keyword",
			body: "parent of #1",
			want: []model.Relationship{
				{Type: model.RelationChild, TargetID: "1"},
			},
		},
		{
			name: "child of keyword",
			body: "child of #2",
			want: []model.Relationship{
				{Type: model.RelationParent, TargetID: "2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_Labels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels []string
		want   []model.Relationship
	}{
		{
			name:   "blocked-by label",
			labels: []string{"blocked-by:15"},
			want: []model.Relationship{
				{Type: model.RelationBlockedBy, TargetID: "15"},
			},
		},
		{
			name:   "blocks label",
			labels: []string{"blocks:20"},
			want: []model.Relationship{
				{Type: model.RelationBlocks, TargetID: "20"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships("", tt.labels, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_Deduplication(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "duplicate bare reference",
			body: "See #123 and also #123",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "123"},
			},
		},
		{
			name: "duplicate keyword reference",
			body: "closes #42 and fixes #42",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "42"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_SelfReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		self string
		want []model.Relationship
	}{
		{
			name: "bare self-reference filtered",
			body: "See #5 for details",
			self: "5",
			want: nil,
		},
		{
			name: "keyword self-reference filtered",
			body: "closes #5",
			self: "5",
			want: nil,
		},
		{
			name: "mixed with non-self references kept",
			body: "closes #5 and relates to #10",
			self: "5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "10"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, tt.self)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_EmptyInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		labels []string
		self   string
	}{
		{
			name: "empty body and nil labels",
			body: "",
		},
		{
			name:   "empty body and empty labels",
			body:   "",
			labels: []string{},
		},
		{
			name:   "only unrelated labels",
			body:   "",
			labels: []string{"bug", "enhancement"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, tt.labels, tt.self)
			if len(got) != 0 {
				t.Errorf("ParseRelationships() = %v, want empty slice", got)
			}
		})
	}
}

func TestParseRelationships_MixedKeywords(t *testing.T) {
	t.Parallel()

	body := `Implements feature XYZ

closes #10
blocks #20
blocked by #30
parent of #40
child of #50`

	got := ParseRelationships(body, nil, "")
	want := []model.Relationship{
		{Type: model.RelationRelatesTo, TargetID: "10"},
		{Type: model.RelationBlocks, TargetID: "20"},
		{Type: model.RelationBlockedBy, TargetID: "30"},
		{Type: model.RelationChild, TargetID: "40"},
		{Type: model.RelationParent, TargetID: "50"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseRelationships() = %v, want %v", got, want)
	}
}

func TestParseRelationships_CaseInsensitiveKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "capitalized keyword",
			body: "Closes #1",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "1"},
			},
		},
		{
			name: "uppercase keyword",
			body: "BLOCKS #2",
			want: []model.Relationship{
				{Type: model.RelationBlocks, TargetID: "2"},
			},
		},
		{
			name: "mixed case phrase",
			body: "Blocked By #3",
			want: []model.Relationship{
				{Type: model.RelationBlockedBy, TargetID: "3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_NoFalsePositives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "discloses #5 should not match closes keyword",
			body: "discloses #5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "5"},
			},
		},
		{
			name: "unblocks #5 should not match blocks keyword",
			body: "unblocks #5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "5"},
			},
		},
		{
			name: "encloses #5 should not match closes keyword",
			body: "encloses #5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "5"},
			},
		},
		{
			name: "nodepends on #3 should not match depends on keyword",
			body: "nodepends on #3",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "3"},
			},
		},
		{
			name: "grandparent of #5 should not match parent of keyword",
			body: "grandparent of #5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "5"},
			},
		},
		{
			name: "grandchild of #5 should not match child of keyword",
			body: "grandchild of #5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "5"},
			},
		},
		{
			name: "unblocked by #5 should not match blocked by keyword",
			body: "unblocked by #5",
			want: []model.Relationship{
				{Type: model.RelationRelatesTo, TargetID: "5"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRelationships_KebabCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.Relationship
	}{
		{
			name: "kebab-case blocked-by",
			body: "blocked-by #4",
			want: []model.Relationship{
				{Type: model.RelationBlockedBy, TargetID: "4"},
			},
		},
		{
			name: "kebab-case in sentence",
			body: "This is blocked-by #6 and relates to #7",
			want: []model.Relationship{
				{Type: model.RelationBlockedBy, TargetID: "6"},
				{Type: model.RelationRelatesTo, TargetID: "7"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRelationships(tt.body, nil, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRelationships() = %v, want %v", got, tt.want)
			}
		})
	}
}
