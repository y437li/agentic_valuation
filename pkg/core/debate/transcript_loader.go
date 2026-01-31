package debate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TranscriptLoader handles the loading of transcript JSON files
type TranscriptLoader struct {
	BaseDir string
}

// NewTranscriptLoader creates a new loader pointing to the batch_data directory
func NewTranscriptLoader(baseDir string) *TranscriptLoader {
	return &TranscriptLoader{BaseDir: baseDir}
}

// LoadTranscriptsForTicker loads and returns sorted transcripts for a given ticker
func (l *TranscriptLoader) LoadTranscriptsForTicker(ticker string) ([]Transcript, error) {
	// sanitize ticker
	safeTicker := strings.ToUpper(strings.TrimSpace(ticker))
	filename := fmt.Sprintf("%s.json", safeTicker)
	path := filepath.Join(l.BaseDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Transcript{}, nil // Return empty list if no transcripts found
		}
		return nil, err
	}

	var transcripts []Transcript
	if err := json.Unmarshal(data, &transcripts); err != nil {
		return nil, fmt.Errorf("failed to parse transcript json: %v", err)
	}

	// Ensure sorted by date descending (Newest first)
	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].Date > transcripts[j].Date
	})

	return transcripts, nil
}
