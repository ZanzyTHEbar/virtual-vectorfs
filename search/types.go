package search

import (
	"errors"
	"time"
)

// MaxSearchLimit caps results returned by Search and FindFiles; FFF grep uses
// its own match-target and file-offset pagination limits.
const MaxSearchLimit = 10000

// MaxExpandBytes caps the UTF-8 bytes returned by one Expand request.
const MaxExpandBytes = 16 << 10

const (
	// MaxGrepFilesPerPage bounds distinct files represented in a grep result page.
	MaxGrepFilesPerPage = 100
	// MaxGrepMatchesPerFile bounds returned matches for one file. FFF may finish
	// the last file, so a match-target page can exceed its target by this amount.
	MaxGrepMatchesPerFile = 20
	// MaxGrepMatchTarget leaves room for one file's final matches under the output-file cap.
	MaxGrepMatchTarget = MaxGrepFilesPerPage - MaxGrepMatchesPerFile
	// MaxGrepContextLines caps context lines on each side of a match.
	MaxGrepContextLines = 5
	// MaxGrepLineBytes caps each returned matched or context line in UTF-8 bytes.
	MaxGrepLineBytes = 384
	// MaxGrepFileBytes is the largest file FFF reads for a grep query.
	MaxGrepFileBytes = 16 << 20
	// MaxGrepTimeBudgetMS bounds each FFF grep request, including sparse queries.
	MaxGrepTimeBudgetMS = 500
)

type Storage string

const (
	StorageDisk   Storage = "disk"
	StorageMemory Storage = "memory"
)

type VectorMetric string

const (
	VectorCosine VectorMetric = "cosine"
	VectorDot    VectorMetric = "dot"
	VectorL2     VectorMetric = "l2"
)

type VectorQuality string

const (
	VectorLatency         VectorQuality = "latency"
	VectorRecall          VectorQuality = "recall"
	VectorMemoryEfficient VectorQuality = "memory_efficient"
)

type Options struct {
	Root             string
	IndexPath        string
	Storage          Storage
	VectorDimensions int // Required per index; must match vectors supplied by the driver.
	VectorMetric     VectorMetric
	VectorQuality    VectorQuality
	FilterFields     []string
	MaxFileBytes     int64
	Workers          int
	QueueCapacity    int
	WriteBatchBytes  int64 // Target document-text bytes per Bleve batch; zero uses 32 MiB.

	FFFContentCache     bool
	FFFMmapCache        bool
	FFFCacheMaxFiles    int
	FFFCacheMaxBytes    uint64
	FFFCacheMaxFileSize uint64
}

type Document struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Path        string            `json:"path"`
	Title       string            `json:"title"`
	Text        string            `json:"text"`
	Context     string            `json:"context"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Vector      []float32         `json:"vector,omitempty"`
	ContentHash string            `json:"content_hash,omitempty"`
}

type VectorInput struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Path        string            `json:"path"`
	Title       string            `json:"title"`
	Text        string            `json:"text"`
	Context     string            `json:"context"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ContentHash string            `json:"content_hash"`
	HasVector   bool              `json:"has_vector"`
	Stale       bool              `json:"stale"`
}

type VectorUpdate struct {
	ID                  string    `json:"id"`
	ExpectedContentHash string    `json:"expected_content_hash"`
	Vector              []float32 `json:"vector"`
}

var ErrStaleContent = errors.New("document content is stale")

type StaleContentError struct {
	ID       string
	Expected string
	Actual   string
	Stale    bool
}

func (e *StaleContentError) Error() string {
	if e.Stale {
		return "vector update rejected for " + e.ID + ": indexed document is stale"
	}
	return "vector update rejected for " + e.ID + ": content hash changed"
}

func (e *StaleContentError) Unwrap() error { return ErrStaleContent }

type QueryMode string

const (
	ModeKeyword  QueryMode = "keyword"
	ModeSemantic QueryMode = "semantic"
	ModeHybrid   QueryMode = "hybrid"
)

type Query struct {
	Mode    QueryMode
	Text    string
	Vector  []float32
	Filters map[string]string
	Limit   int
	// IncludeRankEvidence adds per-channel ranks to hits; hybrid mode needs two extra searches.
	IncludeRankEvidence bool
}

type RankEvidence struct {
	KeywordRank int `json:"keyword_rank,omitempty"`
	VectorRank  int `json:"vector_rank,omitempty"`
}

type Hit struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	// Snippet is a bounded index-time preview, not a query-centered highlight.
	Snippet          string            `json:"snippet"`
	SnippetTruncated bool              `json:"snippet_truncated"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ContentHash      string            `json:"content_hash,omitempty"`
	Score            float64           `json:"score"`
	// ByteOffset is the preview's source start (0), not the query match offset.
	ByteOffset int64         `json:"byte_offset"`
	Locator    string        `json:"locator"`
	Ranks      *RankEvidence `json:"ranks,omitempty"`
	Generation string        `json:"generation"`
	Stale      bool          `json:"stale,omitempty"`
	HasVector  bool          `json:"has_vector"`
}

// ExpandRequest selects a versioned byte range from one explicit source field.
type ExpandRequest struct {
	ID                  string
	ExpectedContentHash string
	ExpectedSource      string
	Field               string
	Offset              int
	Limit               int
}

const (
	OriginLiveFile      = "live_file"
	OriginIndexedRecord = "indexed_record"
)

// Excerpt describes a verified UTF-8 byte range, not a query-match location.
type Excerpt struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Path        string `json:"path"`
	Field       string `json:"field"`
	ContentHash string `json:"content_hash"`
	Origin      string `json:"origin"`
	StartByte   int64  `json:"start_byte"`
	EndByte     int64  `json:"end_byte"`
	TotalBytes  int64  `json:"total_bytes"`
	Text        string `json:"text"`
}

type BuildReport struct {
	GenerationID        string    `json:"generation_id"`
	Root                string    `json:"root"`
	Scanned             int       `json:"scanned"`
	InventoryPages      int       `json:"inventory_pages"`
	InventoryPageSize   int       `json:"inventory_page_size"`
	Workers             int       `json:"workers"`
	MaxFileBytes        int64     `json:"max_file_bytes"`
	WriteBatchBytes     int64     `json:"write_batch_bytes"`
	WriteBatchDocuments int       `json:"write_batch_documents"`
	WriteBatches        int       `json:"write_batches"`
	Indexed             int       `json:"indexed"`
	Skipped             int       `json:"skipped"`
	SkippedUnsupported  int       `json:"skipped_unsupported"`
	SkippedBinary       int       `json:"skipped_binary"`
	SkippedOverLimit    int       `json:"skipped_over_limit"`
	Stale               int       `json:"stale"`
	Errors              int       `json:"errors"`
	Bytes               int64     `json:"bytes"`
	MissingVectors      int       `json:"missing_vectors"`
	EvidencePath        string    `json:"evidence_path"`
	EvidenceCount       int       `json:"evidence_count"`
	CreatedAt           time.Time `json:"created_at"`
}

type FileEvidence struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	Status    string `json:"status"`
	HasVector bool   `json:"has_vector"`
	Error     string `json:"error,omitempty"`
}

type Status struct {
	Root         string  `json:"root"`
	IndexPath    string  `json:"index_path"`
	Storage      Storage `json:"storage"`
	GenerationID string  `json:"generation_id,omitempty"`
	// LastSyncIndexedFiles is the successful full-sync count; event updates are asynchronous.
	LastSyncIndexedFiles int    `json:"last_sync_indexed_files"`
	Closed               bool   `json:"closed"`
	FFFReady             bool   `json:"fff_ready"`
	FFFWatchError        string `json:"fff_watch_error,omitempty"`
	LastError            string `json:"last_error,omitempty"`
}

type FileMatch struct {
	Path             string   `json:"path"`
	Size             int64    `json:"size"`
	Modified         int64    `json:"modified"`
	Rank             int      `json:"rank"`
	Score            int      `json:"score"`
	MatchByteOffsets [][2]int `json:"match_byte_offsets"`
}

type FileQuery struct {
	Text   string
	Offset int
	Limit  int
}

type FilePage struct {
	Matches      []FileMatch `json:"matches"`
	TotalMatched int         `json:"total_matched"`
	TotalFiles   int         `json:"total_files"`
	NextOffset   int         `json:"next_offset"`
}

type GrepMatch struct {
	Path             string   `json:"path"`
	LineNumber       int      `json:"line_number"`
	Column           int      `json:"column"`
	ByteOffset       int64    `json:"byte_offset"`
	LineContent      string   `json:"line_content"`
	MatchByteOffsets [][2]int `json:"match_byte_offsets"`
	ContextBefore    []string `json:"context_before"`
	ContextAfter     []string `json:"context_after"`
	Truncated        bool     `json:"truncated"`
}

type GrepMode string

const (
	GrepModePlain GrepMode = "plain"
	GrepModeRegex GrepMode = "regex"
	GrepModeFuzzy GrepMode = "fuzzy"
)

type GrepQuery struct {
	Text          string
	Mode          GrepMode
	FileOffset    int // File cursor returned by GrepPage.NextFileOffset.
	Limit         int // FFF match target; the final file may add up to MaxGrepMatchesPerFile matches.
	ContextBefore int
	ContextAfter  int
}

type GrepPage struct {
	Matches           []GrepMatch `json:"matches"`
	TotalFiles        int         `json:"total_files"`
	FilesSearched     int         `json:"files_searched"`
	FilteredFileCount int         `json:"filtered_file_count"`
	NextFileOffset    int         `json:"next_file_offset"` // FFF file cursor for the next page, independent of match count.
	// LiteralFallback is the query text FFF retried literally, or empty when unused.
	LiteralFallback string `json:"literal_fallback"`
}
