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

// cleanRemotePath normalizes Windows/SFTP absolute paths for comparison.
func cleanRemotePath(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(strings.ToLower(p), `\\?\`) {
		p = p[4:]
	}
	if strings.HasPrefix(strings.ToLower(p), `\\.\`) {
		p = p[4:]
	}
	return path.Clean(strings.ReplaceAll(p, `\`, "/"))
}

// relTrimAfterRoot returns absPath relative to root (remote paths, slash-separated).
func relTrimAfterRoot(absPath, root string) (string, error) {
	absPath = cleanRemotePath(absPath)
	root = cleanRemotePath(root)
	if strings.EqualFold(absPath, root) {
		return "", nil
	}
	if !pathUnderRemoteRoot(absPath, root) {
		return "", fmt.Errorf("path %q is outside root %q", absPath, root)
	}
	suffix := strings.TrimPrefix(absPath[len(root):], "/")
	return NormalizeRelativePath(suffix), nil
}

// pathUnderRemoteRoot reports whether absPath is root or a descendant (POSIX paths on SFTP).
func pathUnderRemoteRoot(absPath, root string) bool {
	absPath = cleanRemotePath(absPath)
	root = cleanRemotePath(root)
	if absPath == root || strings.EqualFold(absPath, root) {
		return true
	}
	if len(root) >= 2 && root[1] == ':' {
		return strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(root)+"/")
	}
	return strings.HasPrefix(absPath, root+"/")
}

// RelToSource converts an absolute object path to a stable relative path from the source root.
func RelToSource(layout DirectoryLayout, absPath string, remote bool) (string, error) {
	prefix := layout.SourcePathPrefix
	if remote {
		return relTrimAfterRoot(absPath, prefix)
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
		return relTrimAfterRoot(absPath, root)
	}
	absPath = filepath.Clean(absPath)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", err
	}
	return NormalizeRelativePath(filepath.ToSlash(rel)), nil
}