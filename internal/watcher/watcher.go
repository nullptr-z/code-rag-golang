package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/zheng/crag/internal/analyzer"
	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/storage"
)

// Watcher watches for file changes and triggers reanalysis
type Watcher struct {
	projectPath string
	dbPath      string
	fsWatcher   *fsnotify.Watcher

	// Debouncing
	debounceDelay time.Duration
	pendingFiles  map[string]struct{}
	pendingMu     sync.Mutex
	debounceTimer *time.Timer

	// Callbacks
	onAnalysisStart func()
	onAnalysisDone  func(nodeCount, edgeCount int64, duration time.Duration)
	onError         func(error)

	// Control
	done chan struct{}
}

// WatcherOption configures the watcher
type WatcherOption func(*Watcher)

// WithDebounceDelay sets the debounce delay
func WithDebounceDelay(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.debounceDelay = d
	}
}

// WithOnAnalysisStart sets the callback for when analysis starts
func WithOnAnalysisStart(fn func()) WatcherOption {
	return func(w *Watcher) {
		w.onAnalysisStart = fn
	}
}

// WithOnAnalysisDone sets the callback for when analysis completes
func WithOnAnalysisDone(fn func(nodeCount, edgeCount int64, duration time.Duration)) WatcherOption {
	return func(w *Watcher) {
		w.onAnalysisDone = fn
	}
}

// WithOnError sets the callback for errors
func WithOnError(fn func(error)) WatcherOption {
	return func(w *Watcher) {
		w.onError = fn
	}
}

// New creates a new Watcher
func New(projectPath, dbPath string, opts ...WatcherOption) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		projectPath:   projectPath,
		dbPath:        dbPath,
		fsWatcher:     fsWatcher,
		debounceDelay: 500 * time.Millisecond, // Default debounce
		pendingFiles:  make(map[string]struct{}),
		done:          make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	// Add all directories to watch
	if err := w.addDirs(); err != nil {
		fsWatcher.Close()
		return nil, fmt.Errorf("failed to add directories to watch: %w", err)
	}

	return w, nil
}

// addDirs recursively adds all directories to the watcher
func (w *Watcher) addDirs() error {
	return filepath.Walk(w.projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common non-source directories
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}

		return nil
	})
}

// Start begins watching for changes
func (w *Watcher) Start() {
	go w.eventLoop()
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	close(w.done)
	return w.fsWatcher.Close()
}

// eventLoop handles file system events
func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			if w.onError != nil {
				w.onError(err)
			}
		}
	}
}

// handleEvent processes a single file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Only care about Go files
	if !strings.HasSuffix(event.Name, ".go") {
		return
	}

	// Skip test files
	if strings.HasSuffix(event.Name, "_test.go") {
		return
	}

	// Only care about write/create/remove events
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	// Handle new directories
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.fsWatcher.Add(event.Name)
		}
	}

	// Add to pending files and reset debounce timer
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	w.pendingFiles[event.Name] = struct{}{}

	// Reset debounce timer
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}
	w.debounceTimer = time.AfterFunc(w.debounceDelay, w.triggerAnalysis)
}

// triggerAnalysis runs the analysis after debounce
func (w *Watcher) triggerAnalysis() {
	w.pendingMu.Lock()
	files := make([]string, 0, len(w.pendingFiles))
	for f := range w.pendingFiles {
		files = append(files, f)
	}
	w.pendingFiles = make(map[string]struct{})
	w.pendingMu.Unlock()

	if len(files) == 0 {
		return
	}

	if w.onAnalysisStart != nil {
		w.onAnalysisStart()
	}

	startTime := time.Now()

	// Run full analysis
	nodeCount, edgeCount, err := w.runAnalysis()
	if err != nil {
		if w.onError != nil {
			w.onError(fmt.Errorf("analysis failed: %w", err))
		}
		return
	}

	duration := time.Since(startTime)

	if w.onAnalysisDone != nil {
		w.onAnalysisDone(nodeCount, edgeCount, duration)
	}
}

// runAnalysis performs the actual code analysis
func (w *Watcher) runAnalysis() (nodeCount, edgeCount int64, err error) {
	// Load packages
	pkgs, err := analyzer.LoadPackages(w.projectPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to load packages: %w", err)
	}

	// Filter packages with source
	pkgs = analyzer.FilterMainPackages(pkgs)
	if len(pkgs) == 0 {
		return 0, 0, fmt.Errorf("no valid Go packages found")
	}

	// Build SSA
	prog, _ := analyzer.BuildSSA(pkgs)

	// Build call graph
	cg, err := analyzer.BuildCallGraph(prog)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to build call graph: %w", err)
	}

	// Open database
	db, err := storage.Open(w.dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Clear existing data
	if err := db.Clear(); err != nil {
		return 0, 0, fmt.Errorf("failed to clear database: %w", err)
	}

	// Build and store graph
	builder := graph.NewBuilder(
		prog.Fset,
		pkgs,
		db.InsertNode,
		db.InsertEdge,
	)

	if err := builder.Build(cg); err != nil {
		return 0, 0, fmt.Errorf("failed to build graph: %w", err)
	}

	nodeCount, edgeCount, _ = db.GetStats()
	return nodeCount, edgeCount, nil
}
