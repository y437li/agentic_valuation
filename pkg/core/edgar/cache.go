// Package edgar provides caching for SEC EDGAR filings.
package edgar

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MarkdownCache provides file-based caching for converted markdown
type MarkdownCache struct {
	cacheDir string
}

// NewMarkdownCache creates a new markdown cache
// Cache directory defaults to .cache/edgar in the current working directory
func NewMarkdownCache() *MarkdownCache {
	cacheDir := filepath.Join(".cache", "edgar", "markdown")
	os.MkdirAll(cacheDir, 0755)
	return &MarkdownCache{cacheDir: cacheDir}
}

// NewMarkdownCacheWithDir creates a cache with a custom directory
func NewMarkdownCacheWithDir(dir string) *MarkdownCache {
	os.MkdirAll(dir, 0755)
	return &MarkdownCache{cacheDir: dir}
}

// cacheKey generates a unique key for a filing
func (c *MarkdownCache) cacheKey(cik, accession string) string {
	// Normalize accession number (remove dashes)
	accession = strings.ReplaceAll(accession, "-", "")
	return fmt.Sprintf("%s_%s", cik, accession)
}

// filePath returns the file path for a cache entry
func (c *MarkdownCache) filePath(key string) string {
	return filepath.Join(c.cacheDir, key+".md")
}

// Get retrieves cached markdown for a filing
// Returns empty string if not cached
func (c *MarkdownCache) Get(cik, accession string) string {
	key := c.cacheKey(cik, accession)
	path := c.filePath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(data)
}

// Set stores markdown in the cache
func (c *MarkdownCache) Set(cik, accession, markdown string) error {
	key := c.cacheKey(cik, accession)
	path := c.filePath(key)

	return os.WriteFile(path, []byte(markdown), 0644)
}

// Has checks if a filing is cached
func (c *MarkdownCache) Has(cik, accession string) bool {
	key := c.cacheKey(cik, accession)
	path := c.filePath(key)

	_, err := os.Stat(path)
	return err == nil
}

// GetCacheDir returns the cache directory path
func (c *MarkdownCache) GetCacheDir() string {
	return c.cacheDir
}

// ClearCache removes all cached files
func (c *MarkdownCache) ClearCache() error {
	return os.RemoveAll(c.cacheDir)
}

// ContentHash returns MD5 hash of content for verification
func ContentHash(content string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}
