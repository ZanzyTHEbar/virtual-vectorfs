package watcher

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"io"
	"os"
)

// SimhashFingerprinter implements the Fingerprinter interface using simhash
type SimhashFingerprinter struct {
	sampleSize int // Number of tokens to sample from file
}

// NewSimhashFingerprinter creates a new simhash fingerprinter
func NewSimhashFingerprinter() *SimhashFingerprinter {
	return &SimhashFingerprinter{
		sampleSize: 1000, // Sample 1000 tokens by default
	}
}

// Fingerprint generates a fingerprint for the given path
func (f *SimhashFingerprinter) Fingerprint(path string) (*Fingerprint, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// For directories, use size and modtime only
	if info.IsDir() {
		return &Fingerprint{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    0, // No content hash for directories
		}, nil
	}

	// For files, generate simhash of content
	hash, err := f.generateSimhash(path)
	if err != nil {
		return nil, fmt.Errorf("failed to generate simhash for %s: %w", path, err)
	}

	return &Fingerprint{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Hash:    hash,
	}, nil
}

// Compare compares two fingerprints and returns similarity score (0.0 to 1.0)
func (f *SimhashFingerprinter) Compare(a, b *Fingerprint) float64 {
	if a == nil || b == nil {
		return 0.0
	}

	// If paths are different, they're not the same file
	if a.Path != b.Path {
		return 0.0
	}

	// If both are directories, compare size and modtime
	if a.Hash == 0 && b.Hash == 0 {
		if a.Size == b.Size && a.ModTime.Equal(b.ModTime) {
			return 1.0
		}
		return 0.0
	}

	// If one is directory and other is file, they're different
	if (a.Hash == 0) != (b.Hash == 0) {
		return 0.0
	}

	// For files, compare size, modtime, and simhash
	if a.Size != b.Size {
		return 0.0
	}

	if !a.ModTime.Equal(b.ModTime) {
		return 0.0
	}

	// Calculate Hamming distance between simhashes
	distance := hammingDistance(a.Hash, b.Hash)
	maxBits := uint(64) // uint64 has 64 bits
	similarity := 1.0 - float64(distance)/float64(maxBits)

	return similarity
}

// generateSimhash generates a simhash for a file's content
func (f *SimhashFingerprinter) generateSimhash(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Initialize vector for simhash calculation
	vector := make([]int64, 64) // 64-bit simhash

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

	tokensProcessed := 0
	for scanner.Scan() && tokensProcessed < f.sampleSize {
		token := scanner.Text()

		// Hash the token
		hash := fnv.New64a()
		hash.Write([]byte(token))
		tokenHash := hash.Sum64()

		// Update vector based on token hash bits
		for i := 0; i < 64; i++ {
			if (tokenHash & (1 << uint(i))) != 0 {
				vector[i]++
			} else {
				vector[i]--
			}
		}

		tokensProcessed++
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error scanning file: %w", err)
	}

	// Convert vector to simhash
	var simhash uint64
	for i := 0; i < 64; i++ {
		if vector[i] >= 0 {
			simhash |= (1 << uint(i))
		}
	}

	return simhash, nil
}

// hammingDistance calculates the Hamming distance between two uint64 values
func hammingDistance(a, b uint64) int {
	distance := 0
	x := a ^ b // XOR to find differing bits

	// Count set bits using Brian Kernighan's algorithm
	for x != 0 {
		x &= x - 1
		distance++
	}

	return distance
}

// ContentFingerprinter provides more sophisticated content fingerprinting
type ContentFingerprinter struct {
	maxSampleSize int64 // Maximum bytes to sample from large files
}

// NewContentFingerprinter creates a new content fingerprinter
func NewContentFingerprinter() *ContentFingerprinter {
	return &ContentFingerprinter{
		maxSampleSize: 1024 * 1024, // 1MB sample size
	}
}

// Fingerprint generates a SHA256-based fingerprint for file content
func (f *ContentFingerprinter) Fingerprint(path string) (*Fingerprint, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	if info.IsDir() {
		return &Fingerprint{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    0,
		}, nil
	}

	hash, err := f.generateSHA256(path, info.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to generate SHA256 for %s: %w", path, err)
	}

	return &Fingerprint{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Hash:    hash,
	}, nil
}

// generateSHA256 generates SHA256 hash of file content
func (f *ContentFingerprinter) generateSHA256(path string, size int64) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	hasher := sha256.New()

	// For large files, only sample the beginning
	sampleSize := size
	if sampleSize > f.maxSampleSize {
		sampleSize = f.maxSampleSize
	}

	if _, err := io.CopyN(hasher, file, sampleSize); err != nil && err != io.EOF {
		return 0, err
	}

	// Convert first 8 bytes of SHA256 to uint64
	hashBytes := hasher.Sum(nil)
	hash := uint64(hashBytes[0])<<56 |
		uint64(hashBytes[1])<<48 |
		uint64(hashBytes[2])<<40 |
		uint64(hashBytes[3])<<32 |
		uint64(hashBytes[4])<<24 |
		uint64(hashBytes[5])<<16 |
		uint64(hashBytes[6])<<8 |
		uint64(hashBytes[7])

	return hash, nil
}

// Compare compares two content fingerprints
func (f *ContentFingerprinter) Compare(a, b *Fingerprint) float64 {
	if a == nil || b == nil {
		return 0.0
	}

	if a.Path != b.Path {
		return 0.0
	}

	// For directories
	if a.Hash == 0 && b.Hash == 0 {
		if a.Size == b.Size && a.ModTime.Equal(b.ModTime) {
			return 1.0
		}
		return 0.0
	}

	// For files, exact match required
	if a.Size == b.Size && a.ModTime.Equal(b.ModTime) && a.Hash == b.Hash {
		return 1.0
	}

	return 0.0
}
