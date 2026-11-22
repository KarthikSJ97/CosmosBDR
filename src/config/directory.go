package config

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// DirectoryLayout resolves a directory backup root for listing.
type DirectoryLayout struct {
	SourcePathPrefix string // stored on jobs.source_path
	ListRootPath     string // absolute path walked by list activity
	DirRelative      string // normalized directory root relative to source (may be ".")
	RemoteIO         bool
	SourceType       string
}

// ResolveDirectoryLayout maps source_id + directory relative_path (may be ".") for directory backups.
func (r *SourceRegistry) ResolveDirectoryLayout(sourceID, dirRelative string) (DirectoryLayout, error) {
	src, ok := r.GetSource(sourceID)
	if !ok {
		return DirectoryLayout{}, fmt.Errorf("unknown source_id %q", sourceID)
	}
	dirRel := NormalizeRelativePath(dirRelative)
	if dirRel == "" {
		dirRel = "."
	}

	layout := DirectoryLayout{
		DirRelative: dirRel,
		SourceType:  strings.ToLower(src.Type),
	}

	switch strings.ToLower(src.Type) {
	case "local":
		if src.ContainerMount == "" {
			return DirectoryLayout{}, fmt.Errorf("source %q: container_mount is required for type local", sourceID)
		}
		layout.SourcePathPrefix = src.ContainerMount
		if dirRel == "." {
			layout.ListRootPath = src.ContainerMount
		} else {
			layout.ListRootPath = filepath.Join(src.ContainerMount, filepath.FromSlash(dirRel))
		}
	case "sftp":
		root := strings.TrimRight(src.RemoteRoot, "/")
		if root == "" {
			return DirectoryLayout{}, fmt.Errorf("source %q: remote_root is required", sourceID)
		}
		layout.SourcePathPrefix = root
		if dirRel == "." {
			layout.ListRootPath = path.Clean(root)
		} else {
			layout.ListRootPath = path.Join(root, dirRel)
		}
		layout.RemoteIO = true
	default:
		return DirectoryLayout{}, fmt.Errorf("source %q: unsupported type %q", sourceID, src.Type)
	}
	return layout, nil
}

// RelToSource converts an absolute object path to a stable relative path from the source root.
func RelToSource(layout DirectoryLayout, absPath string, remote bool) (string, error) {
	prefix := layout.SourcePathPrefix
	if remote {
		absPath = path.Clean(absPath)
		prefix = path.Clean(prefix)
		trim := strings.TrimPrefix(absPath, prefix)
		trim = strings.TrimPrefix(trim, "/")
		if trim == absPath && absPath != prefix {
			return "", fmt.Errorf("path %q is outside source root %q", absPath, prefix)
		}
		return NormalizeRelativePath(trim), nil
	}
	absPath = filepath.Clean(absPath)
	prefix = filepath.Clean(prefix)
	rel, err := filepath.Rel(prefix, absPath)
	if err != nil {
		return "", err
	}
	return NormalizeRelativePath(filepath.ToSlash(rel)), nil
}

// RelToDir converts an absolute object path to a path relative to the directory list root.
func RelToDir(layout DirectoryLayout, absPath string, remote bool) (string, error) {
	root := layout.ListRootPath
	if remote {
		absPath = path.Clean(absPath)
		root = path.Clean(root)
		trim := strings.TrimPrefix(absPath, root)
		trim = strings.TrimPrefix(trim, "/")
		return NormalizeRelativePath(trim), nil
	}
	absPath = filepath.Clean(absPath)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", err
	}
	return NormalizeRelativePath(filepath.ToSlash(rel)), nil
}