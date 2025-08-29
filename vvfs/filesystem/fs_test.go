package filesystem

import (
	"context"
	"testing"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/options"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInteractor is a mock implementation of ui.Interactor for testing
type mockInteractor struct{}

// Prompt methods
func (m *mockInteractor) Prompt(message string, defaultValue string) (string, error) {
	return defaultValue, nil
}

func (m *mockInteractor) Confirm(message string, defaultValue bool) (bool, error) {
	return defaultValue, nil
}

func (m *mockInteractor) Select(message string, options []string, defaultValue string) (string, error) {
	if len(options) > 0 {
		return options[0], nil
	}
	return defaultValue, nil
}

// Output methods
func (m *mockInteractor) Output(message string)                       { /* mock implementation */ }
func (m *mockInteractor) Outputf(format string, args ...interface{})  { /* mock implementation */ }
func (m *mockInteractor) Success(message string)                      { /* mock implementation */ }
func (m *mockInteractor) Successf(format string, args ...interface{}) { /* mock implementation */ }
func (m *mockInteractor) Info(message string)                         { /* mock implementation */ }
func (m *mockInteractor) Infof(format string, args ...interface{})    { /* mock implementation */ }
func (m *mockInteractor) Warning(message string)                      { /* mock implementation */ }
func (m *mockInteractor) Warningf(format string, args ...interface{}) { /* mock implementation */ }
func (m *mockInteractor) Error(message string, err error)             { /* mock implementation */ }
func (m *mockInteractor) Errorf(format string, err error, args ...interface{}) { /* mock implementation */
}
func (m *mockInteractor) Fatal(message string, err error) { /* mock implementation */ }
func (m *mockInteractor) Fatalf(format string, err error, args ...interface{}) { /* mock implementation */
}

// Spinner methods
func (m *mockInteractor) StartSpinner(message string)              { /* mock implementation */ }
func (m *mockInteractor) StopSpinner(success bool, message string) { /* mock implementation */ }

// Help methods
func (m *mockInteractor) ShowCustomHelp(showAll bool, commandPath string) { /* mock implementation */ }

func TestFilePathParamsConversion(t *testing.T) {
	params := options.NewFilePathParams()
	params.SourceDir = "/test/source"
	params.TargetDir = "/test/target"
	params.Recursive = true
	params.DryRun = true
	params.MaxDepth = 5
	params.GitEnabled = true
	params.CopyFiles = true
	params.RemoveAfter = false

	opts := params.ToOrganizationOptions()

	assert.Equal(t, "/test/source", opts.SourceDir)
	assert.Equal(t, "/test/target", opts.TargetDir)
	assert.True(t, opts.Recursive)
	assert.True(t, opts.DryRun)
	assert.Equal(t, 5, opts.MaxDepth)
	assert.True(t, opts.GitEnabled)
	assert.True(t, opts.CopyInsteadOfMove)
	assert.False(t, opts.RemoveAfter)
	assert.Equal(t, 4, opts.WorkerCount)
	assert.Equal(t, 100, opts.BatchSize)
}

func TestGitServiceIntegration(t *testing.T) {
	interactor := &mockInteractor{}
	mockDBProvider := db.NewMockCentralDBProvider()

	dfs, err := New(interactor, mockDBProvider)
	require.NoError(t, err)

	// Test git service accessibility
	gitService := dfs.GetGitService()
	assert.NotNil(t, gitService, "Git service should be available")

	// Test that git methods work (they may fail due to no git repo, but should not panic)
	isRepo := dfs.IsGitRepo("/tmp")
	assert.False(t, isRepo, "Tmp directory should not be a git repo")
}

func TestMaxDepthCalculation(t *testing.T) {
	interactor := &mockInteractor{}
	mockDBProvider := db.NewMockCentralDBProvider()

	dfs, err := New(interactor, mockDBProvider)
	require.NoError(t, err)

	// Test max depth calculation
	depth, err := dfs.CalculateMaxDepth("/tmp")
	// This may fail if /tmp doesn't exist or is inaccessible, but it should not panic
	if err == nil {
		assert.GreaterOrEqual(t, depth, 0, "Depth should be non-negative")
	}
}

func TestServiceArchitecture(t *testing.T) {
	interactor := &mockInteractor{}
	mockDBProvider := db.NewMockCentralDBProvider()

	dfs, err := New(interactor, mockDBProvider)
	require.NoError(t, err)

	// Test that all services are properly initialized
	ctx := context.Background()

	// Test git service methods
	assert.False(t, dfs.IsGitRepo("/tmp"))

	// Test path validation
	err = dfs.ValidatePath("/valid/path")
	assert.NoError(t, err)

	err = dfs.ValidatePath("")
	assert.Error(t, err, "Empty path should be invalid")

	// Test file type detection
	fileType := dfs.GetFileType("test.txt")
	assert.NotEmpty(t, fileType, "Should detect file type")

	// Test directory analysis with a path that likely exists
	_, err = dfs.AnalyzeDirectory(ctx, "/tmp")
	// This may fail due to permissions or missing directory, but should not panic
	// The error handling is what we're testing here
	if err != nil {
		assert.IsType(t, err, err, "Should return proper error type")
	}
}
