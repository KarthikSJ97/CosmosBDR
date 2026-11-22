package sourceread

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/dirsel"
	"cosmosbdr/src/types"

)

// ListDirectoryFiles walks a directory on the source and returns files matching scope.
func ListDirectoryFiles(ctx context.Context, reg *config.SourceRegistry, sourceID string, layout config.DirectoryLayout, scope types.DirectoryScope) ([]types.ListedBackupFile, error) {
	m, err := dirsel.NewMatcher(scope)
	if err != nil {
		return nil, err
	}

	src, ok := reg.GetSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("unknown source_id %q", sourceID)
	}

	switch strings.ToLower(src.Type) {
	case "local":
		return listLocal(ctx, layout, m)
	case "sftp":
		return listSFTP(ctx, src, layout, m)
	default:
		return nil, fmt.Errorf("unsupported source type %q", src.Type)
	}
}

func listLocal(ctx context.Context, layout config.DirectoryLayout, m *dirsel.Matcher) ([]types.ListedBackupFile, error) {
	root := layout.ListRootPath
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat list root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("list root is not a directory: %s", root)
	}

	var out []types.ListedBackupFile
	err = filepath.WalkDir(root, func(absPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			if absPath != root {
				// Do not descend into symlinks to other trees.
				if d.Type()&fs.ModeSymlink != 0 {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		relSource, err := config.RelToSource(layout, absPath, false)
		if err != nil {
			return err
		}
		relDir, err := config.RelToDir(layout, absPath, false)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !m.Accept(relDir, info.Size()) {
			return nil
		}
		out = append(out, types.ListedBackupFile{
			RelativeToSource: relSource,
			RelativeToDir:    relDir,
			SizeBytes:        info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk local %q: %w", root, err)
	}
	return out, nil
}

func listSFTP(ctx context.Context, src config.ResolvedSource, layout config.DirectoryLayout, m *dirsel.Matcher) ([]types.ListedBackupFile, error) {
	client, scl, err := connectSFTP(src)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer scl.Close()

	root := path.Clean(layout.ListRootPath)
	st, err := scl.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("sftp stat list root: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("list root is not a directory: %s", root)
	}

	var out []types.ListedBackupFile
	walker := scl.Walk(root)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return nil, fmt.Errorf("sftp walk: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		path := walker.Path()
		st := walker.Stat()
		if st == nil || st.IsDir() {
			continue
		}
		relSource, err := config.RelToSource(layout, path, true)
		if err != nil {
			return nil, err
		}
		relDir, err := config.RelToDir(layout, path, true)
		if err != nil {
			return nil, err
		}
		if !m.Accept(relDir, st.Size()) {
			continue
		}
		out = append(out, types.ListedBackupFile{
			RelativeToSource: relSource,
			RelativeToDir:    relDir,
			SizeBytes:        st.Size(),
		})
	}
	return out, nil
}