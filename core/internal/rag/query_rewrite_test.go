package rag

import (
	"strings"
	"testing"
)

func TestRewriteQuery(t *testing.T) {
	tests := []struct {
		name     string
		question string
		want     string
	}{
		{
			name:     "strips stop words and trailing punctuation",
			question: "What is the best vector database?",
			want:     "what best vector database",
		},
		{
			name:     "preserves meaningful terms",
			question: "How do goroutines and channels work in Go?",
			want:     "how goroutines channels work go",
		},
		{
			name:     "handles multiple punctuation marks",
			question: "Who should I contact about the incident?!",
			want:     "who contact incident",
		},
		{
			name:     "keeps lone meaningful term",
			question: "Is it the one?",
			want:     "one",
		},
		{
			name:     "empty after trim returns empty",
			question: "?",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteQuery(tt.question)
			if got != tt.want {
				t.Errorf("rewriteQuery(%q) = %q, want %q", tt.question, got, tt.want)
			}
		})
	}
}

func TestRewriteQuery_RemovesStopWords(t *testing.T) {
	q := "What are the advantages of using Postgres with pgvector for the vector database?"
	got := rewriteQuery(q)

	stopList := []string{"are", "the", "of", "for"}
	for _, w := range stopList {
		if strings.Contains(" "+got+" ", " "+w+" ") {
			t.Errorf("stop word %q should have been removed from %q", w, got)
		}
	}

	if !strings.Contains(got, "postgres") || !strings.Contains(got, "pgvector") || !strings.Contains(got, "vector") {
		t.Errorf("expected meaningful terms to be preserved, got %q", got)
	}
}

func TestSummarizeNote(t *testing.T) {
	tests := []struct {
		name string
		body string
		max  int
		want string
	}{
		{
			name: "first sentence within cap",
			body: "We chose Postgres with pgvector. It keeps operations simple.",
			max:  150,
			want: "We chose Postgres with pgvector.",
		},
		{
			name: "truncates long sentence",
			body: "We chose Postgres with pgvector because it keeps operations simple and integrates with our existing stack without adding new infrastructure",
			max:  22,
			want: "We chose Postgres with...",
		},
		{
			name: "empty body",
			body: "",
			max:  150,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeNote(tt.body, tt.max)
			if got != tt.want {
				t.Errorf("summarizeNote(%q, %d) = %q, want %q", tt.body, tt.max, got, tt.want)
			}
		})
	}
}
