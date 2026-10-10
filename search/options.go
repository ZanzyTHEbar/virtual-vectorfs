package search

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func normalizeOptions(options Options) (Options, error) {
	if options.Root == "" {
		return Options{}, errors.New("root is required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return Options{}, fmt.Errorf("resolve root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Options{}, fmt.Errorf("resolve root symlinks: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Options{}, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return Options{}, errors.New("root must be a directory")
	}
	options.Root = root

	if options.Storage == "" {
		options.Storage = StorageDisk
	}
	if options.Storage != StorageDisk && options.Storage != StorageMemory {
		return Options{}, fmt.Errorf("unsupported storage %q", options.Storage)
	}
	if options.IndexPath == "" {
		name := filepath.Base(root)
		if name == string(filepath.Separator) || name == "." || name == "" {
			name = "root"
		}
		options.IndexPath = filepath.Join(filepath.Dir(root), "."+name+".vvfs-search")
	}
	options.IndexPath, err = resolveFuturePath(options.IndexPath)
	if err != nil {
		return Options{}, fmt.Errorf("resolve index path: %w", err)
	}
	evidencePath, err := resolveFuturePath(evidenceDirectory(options.IndexPath, options.Storage))
	if err != nil {
		return Options{}, fmt.Errorf("resolve evidence path: %w", err)
	}
	if pathWithin(root, options.IndexPath) || pathWithin(root, evidencePath) {
		return Options{}, errors.New("index and evidence directories must be outside the FFF root")
	}

	if options.VectorDimensions < 1 || options.VectorDimensions > 4096 {
		return Options{}, errors.New("vector dimensions must be between 1 and 4096")
	}
	if options.VectorMetric == "" {
		options.VectorMetric = VectorCosine
	}
	if options.VectorMetric != VectorCosine && options.VectorMetric != VectorDot && options.VectorMetric != VectorL2 {
		return Options{}, fmt.Errorf("unsupported vector metric %q", options.VectorMetric)
	}
	if options.VectorQuality == "" {
		options.VectorQuality = VectorLatency
	}
	if options.VectorQuality != VectorLatency && options.VectorQuality != VectorRecall && options.VectorQuality != VectorMemoryEfficient {
		return Options{}, fmt.Errorf("unsupported vector quality %q", options.VectorQuality)
	}
	if options.MaxFileBytes == 0 {
		options.MaxFileBytes = 10 << 20
	}
	if options.MaxFileBytes < 0 {
		return Options{}, errors.New("maximum file size must not be negative")
	}
	if options.Workers == 0 {
		options.Workers = 4
	}
	if options.Workers < 1 || options.Workers > 128 {
		return Options{}, errors.New("workers must be between 1 and 128")
	}
	if options.QueueCapacity == 0 {
		options.QueueCapacity = 256
	}
	if options.QueueCapacity < 1 {
		return Options{}, errors.New("queue capacity must be positive")
	}
	if options.WriteBatchBytes == 0 {
		options.WriteBatchBytes = 32 << 20
	}
	if options.WriteBatchBytes < 0 {
		return Options{}, errors.New("write batch byte budget must be positive")
	}
	if options.FFFCacheMaxFiles < 0 {
		return Options{}, errors.New("FFF cache max files must not be negative")
	}
	options.FilterFields = append([]string(nil), options.FilterFields...)
	fields := make(map[string]struct{}, len(options.FilterFields))
	for i, field := range options.FilterFields {
		field = strings.TrimSpace(field)
		if field == "" || strings.Contains(field, ".") || strings.ContainsRune(field, '\x00') {
			return Options{}, fmt.Errorf("invalid filter field %q", field)
		}
		switch field {
		case fieldID, fieldSource, fieldPath:
			return Options{}, fmt.Errorf("filter field %q is reserved for top-level filters", field)
		}
		if _, ok := fields[field]; ok {
			return Options{}, fmt.Errorf("duplicate filter field %q", field)
		}
		fields[field] = struct{}{}
		options.FilterFields[i] = field
	}
	return options, nil
}

func resolveFuturePath(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absolute)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		info, statErr := os.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("resolve dangling symlink %q: %w", current, err)
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve path %q: %w", value, err)
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)))
}

func validateQuery(query Query, dimensions int) (int, error) {
	switch query.Mode {
	case ModeKeyword:
		if strings.TrimSpace(query.Text) == "" {
			return 0, errors.New("keyword mode requires text")
		}
	case ModeSemantic:
		if len(query.Vector) == 0 {
			return 0, errors.New("semantic mode requires a vector")
		}
	case ModeHybrid:
		if strings.TrimSpace(query.Text) == "" {
			return 0, errors.New("hybrid mode requires text")
		}
		if len(query.Vector) == 0 {
			return 0, errors.New("hybrid mode requires a vector")
		}
	default:
		return 0, fmt.Errorf("unsupported query mode %q", query.Mode)
	}
	if query.Mode != ModeKeyword {
		if dimensions == 0 {
			return 0, errors.New("vector dimensions are not configured")
		}
		if len(query.Vector) != dimensions {
			return 0, fmt.Errorf("query vector has dimension %d; want %d", len(query.Vector), dimensions)
		}
		if err := validateVector(query.Vector); err != nil {
			return 0, err
		}
	}
	return normalizeSearchLimit(query.Limit)
}

func normalizeSearchLimit(limit int) (int, error) {
	if limit < 0 {
		return 0, errors.New("search limit must not be negative")
	}
	if limit == 0 {
		return 10, nil
	}
	return min(limit, MaxSearchLimit), nil
}

func normalizeGrepMatchTarget(limit int) (int, error) {
	if limit < 0 {
		return 0, errors.New("grep match target must not be negative")
	}
	if limit == 0 {
		return min(10, MaxGrepMatchTarget), nil
	}
	return min(limit, MaxGrepMatchTarget), nil
}
