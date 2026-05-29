// Package vectors provides pure Go vector operations for similarity search.
package vectors

import (
	"math"
	"sort"
)

// Match represents a similarity match result.
type Match struct {
	Index      int
	Similarity float32
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	if len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		aVal := float64(a[i])
		bVal := float64(b[i])
		dot += aVal * bVal
		normA += aVal * aVal
		normB += bVal * bVal
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// TopK finds the k most similar vectors to the query vector.
// Returns matches sorted by similarity (highest first).
func TopK(query []float32, vectors [][]float32, k int) []Match {
	if k <= 0 {
		return nil
	}

	matches := make([]Match, 0, len(vectors))
	for i, v := range vectors {
		sim := CosineSimilarity(query, v)
		matches = append(matches, Match{
			Index:      i,
			Similarity: sim,
		})
	}

	// Sort by similarity descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Similarity > matches[j].Similarity
	})

	if k > len(matches) {
		k = len(matches)
	}
	return matches[:k]
}

// Normalize L2-normalizes a vector in place.
// After normalization, the vector has unit length (magnitude = 1).
func Normalize(v []float32) {
	var sum float64
	for i := range v {
		val := float64(v[i])
		sum += val * val
	}

	if sum == 0 {
		return
	}

	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
}

// DotProduct computes the dot product of two vectors.
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return float32(sum)
}

// EuclideanDistance computes the Euclidean distance between two vectors.
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}
	return float32(math.Sqrt(sum))
}
