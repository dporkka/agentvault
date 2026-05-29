package vectors

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0, 4.0}
	b := []float32{1.0, 2.0, 3.0, 4.0}

	sim := CosineSimilarity(a, b)
	if math.Abs(float64(sim)-1.0) > 1e-5 {
		t.Errorf("Expected similarity 1.0 for identical vectors, got %f", sim)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{-1.0, -2.0, -3.0}

	sim := CosineSimilarity(a, b)
	if math.Abs(float64(sim)-(-1.0)) > 1e-5 {
		t.Errorf("Expected similarity -1.0 for opposite vectors, got %f", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{0.0, 1.0, 0.0}

	sim := CosineSimilarity(a, b)
	if math.Abs(float64(sim)-0.0) > 1e-5 {
		t.Errorf("Expected similarity 0.0 for orthogonal vectors, got %f", sim)
	}
}

func TestCosineSimilarityDifferentLengths(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0}

	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("Expected similarity 0 for different length vectors, got %f", sim)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	sim := CosineSimilarity([]float32{}, []float32{})
	if sim != 0 {
		t.Errorf("Expected similarity 0 for empty vectors, got %f", sim)
	}
}

func TestCosineSimilarityKnownValue(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{4.0, 5.0, 6.0}

	sim := CosineSimilarity(a, b)
	// dot = 4 + 10 + 18 = 32
	// |a| = sqrt(1 + 4 + 9) = sqrt(14)
	// |b| = sqrt(16 + 25 + 36) = sqrt(77)
	// sim = 32 / sqrt(14 * 77) = 32 / sqrt(1078) ≈ 0.97463
	expected := 32.0 / math.Sqrt(14.0*77.0)
	if math.Abs(float64(sim)-expected) > 1e-5 {
		t.Errorf("Expected similarity %f, got %f", expected, sim)
	}
}

func TestTopK(t *testing.T) {
	query := []float32{1.0, 0.0, 0.0}
	vectors := [][]float32{
		{0.0, 1.0, 0.0},  // orthogonal, sim = 0
		{1.0, 0.0, 0.0},  // identical, sim = 1
		{-1.0, 0.0, 0.0}, // opposite, sim = -1
		{0.5, 0.5, 0.0},  // sim ≈ 0.707
	}

	matches := TopK(query, vectors, 2)

	if len(matches) != 2 {
		t.Fatalf("Expected 2 matches, got %d", len(matches))
	}

	// Highest similarity should be the identical vector at index 1
	if matches[0].Index != 1 {
		t.Errorf("Expected index 1 (identical) first, got index %d", matches[0].Index)
	}
	if math.Abs(float64(matches[0].Similarity)-1.0) > 1e-5 {
		t.Errorf("Expected similarity 1.0, got %f", matches[0].Similarity)
	}

	// Second should be the 0.5, 0.5, 0.0 vector at index 3
	if matches[1].Index != 3 {
		t.Errorf("Expected index 3 second, got index %d", matches[1].Index)
	}
}

func TestTopKKLargerThanN(t *testing.T) {
	query := []float32{1.0, 0.0}
	vectors := [][]float32{
		{1.0, 0.0},
		{0.0, 1.0},
	}

	matches := TopK(query, vectors, 10)
	if len(matches) != 2 {
		t.Fatalf("Expected 2 matches, got %d", len(matches))
	}
}

func TestTopKZeroK(t *testing.T) {
	query := []float32{1.0, 0.0}
	vectors := [][]float32{{1.0, 0.0}}

	matches := TopK(query, vectors, 0)
	if matches != nil {
		t.Error("Expected nil for k=0")
	}
}

func TestNormalize(t *testing.T) {
	v := []float32{3.0, 4.0}
	Normalize(v)

	// After normalization, magnitude should be 1
	var sum float64
	for _, x := range v {
		sum += float64(x * x)
	}
	mag := math.Sqrt(sum)
	if math.Abs(mag-1.0) > 1e-5 {
		t.Errorf("Expected magnitude 1.0 after normalization, got %f", mag)
	}

	// Direction should be preserved: 3:4 ratio
	if math.Abs(float64(v[0])/float64(v[1])-0.75) > 1e-5 {
		t.Errorf("Expected ratio 0.75, got %f", float64(v[0])/float64(v[1]))
	}
}

func TestNormalizeUnitVector(t *testing.T) {
	// Already a unit vector
	v := []float32{1.0, 0.0, 0.0}
	Normalize(v)

	if math.Abs(float64(v[0])-1.0) > 1e-5 {
		t.Errorf("Expected v[0] = 1.0, got %f", v[0])
	}
	if math.Abs(float64(v[1])-0.0) > 1e-5 {
		t.Errorf("Expected v[1] = 0.0, got %f", v[1])
	}
}

func TestNormalizeZeroVector(t *testing.T) {
	v := []float32{0.0, 0.0, 0.0}
	Normalize(v)

	// Should not panic, vector stays zero
	for i, x := range v {
		if x != 0 {
			t.Errorf("Expected v[%d] = 0, got %f", i, x)
		}
	}
}

func TestDotProduct(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{4.0, 5.0, 6.0}

	result := DotProduct(a, b)
	expected := float32(4.0 + 10.0 + 18.0) // 32.0
	if result != expected {
		t.Errorf("Expected dot product %f, got %f", expected, result)
	}
}

func TestEuclideanDistance(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{4.0, 5.0, 6.0}

	dist := EuclideanDistance(a, b)
	// sqrt((4-1)^2 + (5-2)^2 + (6-3)^2) = sqrt(9 + 9 + 9) = sqrt(27) ≈ 5.196
	expected := float32(math.Sqrt(27.0))
	if math.Abs(float64(dist)-float64(expected)) > 1e-5 {
		t.Errorf("Expected distance %f, got %f", expected, dist)
	}
}
