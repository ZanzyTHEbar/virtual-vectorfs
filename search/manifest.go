package search

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const manifestName = "manifest.json"

type evidenceWriter struct {
	file      *os.File
	encoder   *json.Encoder
	tempPath  string
	finalPath string
	count     int
}

func newGenerationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate index generation ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

// Evidence stays durable for both storage modes; only the memory Bleve index is ephemeral.
func evidenceDirectory(indexPath string, storage Storage) string {
	if storage == StorageMemory {
		return indexPath + ".memory.evidence"
	}
	return indexPath + ".evidence"
}

func manifestPath(options Options, generation string) string {
	name := manifestName
	if options.Storage == StorageMemory {
		name = "manifest-" + generation + ".json"
	}
	return filepath.Join(evidenceDirectory(options.IndexPath, options.Storage), name)
}

func (e *Engine) newEvidenceWriter(generation string) (*evidenceWriter, error) {
	directory := evidenceDirectory(e.options.IndexPath, e.options.Storage)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".evidence-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create evidence artifact: %w", err)
	}
	return &evidenceWriter{
		file:      file,
		encoder:   json.NewEncoder(file),
		tempPath:  file.Name(),
		finalPath: filepath.Join(directory, generation+".jsonl"),
	}, nil
}

func (w *evidenceWriter) Write(evidence FileEvidence) error {
	if err := w.encoder.Encode(evidence); err != nil {
		return fmt.Errorf("write file evidence for %q: %w", evidence.Path, err)
	}
	w.count++
	return nil
}

func (w *evidenceWriter) Commit() error {
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync evidence artifact: %w", err)
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close evidence artifact: %w", err)
	}
	w.file = nil
	if err := os.Rename(w.tempPath, w.finalPath); err != nil {
		return fmt.Errorf("publish evidence artifact: %w", err)
	}
	return nil
}

func (w *evidenceWriter) Abort() {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	_ = os.Remove(w.tempPath)
}

func (e *Engine) loadManifest() error {
	if e.options.Storage == StorageMemory {
		return nil
	}
	data, err := os.ReadFile(manifestPath(e.options, ""))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read search manifest: %w", err)
	}
	var report BuildReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("decode search manifest: %w", err)
	}
	if report.Root != e.options.Root || report.GenerationID == "" {
		return errors.New("search manifest root or generation does not match the configured index")
	}
	e.previousEvidencePath = report.EvidencePath
	e.stateMu.Lock()
	e.generationID = report.GenerationID
	e.lastReport = report
	e.stateMu.Unlock()
	return nil
}

func (e *Engine) writeManifest(ctx context.Context, report BuildReport) error {
	directory := evidenceDirectory(e.options.IndexPath, e.options.Storage)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("encode search manifest: %w", err)
	}
	file, err := os.CreateTemp(directory, ".manifest-*")
	if err != nil {
		return fmt.Errorf("create temporary manifest: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write search manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync search manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close search manifest: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, manifestPath(e.options, report.GenerationID)); err != nil {
		return fmt.Errorf("replace search manifest: %w", err)
	}
	return nil
}
