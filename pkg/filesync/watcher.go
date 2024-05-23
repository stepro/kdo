package filesync

import (
	"os"
	"sort"
	"time"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
)

type fileinfo struct {
	path string
	mode os.FileMode
	mod  time.Time
}

const interval = 200 * time.Millisecond

func find2(root string, files []fileinfo, dir string, pm *PatternMatcher) []fileinfo {
	file, err := os.Open(root + "/" + dir)
	if err != nil {
		return nil
	}
	infos, err := file.Readdir(-1)
	file.Close()
	if err != nil {
		return nil
	}
	for _, info := range infos {
		path := dir + info.Name()
		exclude, _ := pm.Matches(path)
		if info.IsDir() {
			path = path + "/"
		}
		if !exclude {
			files = append(files, fileinfo{
				path: path,
				mode: info.Mode(),
				mod:  info.ModTime(),
			})
		}
		if info.IsDir() && (!exclude || pm.Exclusions()) {
			files = find2(root, files, path, pm)
		}
	}
	return files
}

type fileinfos []fileinfo

func (a fileinfos) Len() int           { return len(a) }
func (a fileinfos) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a fileinfos) Less(i, j int) bool { return a[i].path < a[j].path }

func find(root string, pm *PatternMatcher) fileinfos {
	var files fileinfos
	files = find2(root, files, "", pm)
	if files != nil {
		sort.Sort(files)
	}
	return files
}

func compare(baseline fileinfos, latest fileinfos) (added []string, updated []string, deleted []string) {
	var b, l int
	for b < len(baseline) && l < len(latest) {
		if baseline[b].path < latest[l].path {
			deleted = append(deleted, baseline[b].path)
			b++
		} else if baseline[b].path == latest[l].path {
			if baseline[b].mode != latest[l].mode || baseline[b].mod.UnixNano() < latest[l].mod.UnixNano() {
				updated = append(updated, latest[l].path)
			}
			b++
			l++
		} else {
			added = append(added, latest[l].path)
			l++
		}
	}
	for b < len(baseline) {
		deleted = append(deleted, baseline[b].path)
		b++
	}
	for l < len(latest) {
		added = append(added, latest[l].path)
		l++
	}

	return
}

func start(dir string, fn func(added []string, updated []string, deleted []string)) error {
	var patterns []string
	f, err := os.Open(dir + "/.dockerignore")
	if err == nil {
		patterns, err = dockerignore.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
	}

	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		return err
	}

	baseline := find(dir, pm)

	go func() {
		for {
			time.Sleep(interval)
			latest := find(dir, pm)
			if latest != nil {
				if baseline != nil {
					added, updated, deleted := compare(baseline, latest)
					if len(added) > 0 || len(updated) > 0 || len(deleted) > 0 {
						fn(added, updated, deleted)
					}
				}
				baseline = latest
			}
		}
	}()

	return nil
}

// PatternMatcher is a custom implementation to match file patterns
type PatternMatcher struct {
	patterns []string
}

// NewPatternMatcher creates a new PatternMatcher with the given patterns
func NewPatternMatcher(patterns []string) (*PatternMatcher, error) {
	return &PatternMatcher{patterns: patterns}, nil
}

// Matches checks if the given path matches any of the patterns
func (pm *PatternMatcher) Matches(path string) (bool, error) {
	for _, pattern := range pm.patterns {
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// Exclusions checks if there are any exclusion patterns
func (pm *PatternMatcher) Exclusions() bool {
	for _, pattern := range pm.patterns {
		if strings.HasPrefix(pattern, "!") {
			return true
		}
	}
	return false
}
