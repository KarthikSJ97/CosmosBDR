package sourceread

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"cosmosbdr/src/config"
	"cosmosbdr/src/dirsel"
	"cosmosbdr/src/types"

	"github.com/pkg/sftp"
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
	// OpenSSH SFTP readdir on Windows can hang indefinitely; use PowerShell for drive-letter roots.
	if isWindowsDriveRoot(layout.ListRootPath) || isWindowsDriveRoot(layout.SourcePathPrefix) {
		return listSFTPWindowsShell(ctx, src, layout, m)
	}

	client, scl, err := connectSFTP(src)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer scl.Close()

	root := path.Clean(layout.ListRootPath)
	var out []types.ListedBackupFile
	if err := walkSFTPDir(ctx, scl, layout, m, root, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		if _, err := scl.ReadDirContext(ctx, root); err != nil {
			return nil, fmt.Errorf("sftp list root %q: %w", root, err)
		}
	}
	return out, nil
}

// walkSFTPDir lists files with ReadDir (not pkg/sftp Walk). OpenSSH on Windows often
// returns SSH_FX_BAD_MESSAGE for junction/reparse dirs during Walk; skipping those subtrees is enough.
func walkSFTPDir(ctx context.Context, scl *sftp.Client, layout config.DirectoryLayout, m *dirsel.Matcher, dir string, out *[]types.ListedBackupFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := scl.ReadDirContext(ctx, dir)
	if err != nil {
		return nil
	}
	for _, ent := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := ent.Name()
		if name == "." || name == ".." {
			continue
		}
		full := path.Join(dir, name)
		if ent.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if ent.IsDir() {
			if err := walkSFTPDir(ctx, scl, layout, m, full, out); err != nil {
				return err
			}
			continue
		}
		relSource, err := config.RelToSource(layout, full, true)
		if err != nil {
			continue
		}
		relDir, err := config.RelToDir(layout, full, true)
		if err != nil {
			continue
		}
		if !m.Accept(relDir, ent.Size()) {
			continue
		}
		*out = append(*out, types.ListedBackupFile{
			RelativeToSource: relSource,
			RelativeToDir:    relDir,
			SizeBytes:        ent.Size(),
		})
	}
	return nil
}

func isWindowsDriveRoot(p string) bool {
	if len(p) < 2 {
		return false
	}
	c := p[0]
	return p[1] == ':' && ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'))
}

func posixToWin(p string) string {
	return strings.ReplaceAll(p, "/", `\`)
}

func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// listSFTPWindowsShell lists files over SSH (SFTP readdir on Windows OpenSSH can hang).
func listSFTPWindowsShell(ctx context.Context, src config.ResolvedSource, layout config.DirectoryLayout, m *dirsel.Matcher) ([]types.ListedBackupFile, error) {
	listRoot := strings.TrimSpace(layout.ListRootPath)
	if listRoot == "" || listRoot == "." {
		return nil, fmt.Errorf("windows directory list: list root path is missing (expected absolute path under remote_root)")
	}
	rootWin := posixToWin(path.Clean(listRoot))

	paths, err := windowsDirListPaths(ctx, src, rootWin)
	if err != nil {
		return nil, err
	}
	files := listedFilesFromWindowsPaths(layout, m, paths)
	if len(files) > 0 {
		return files, nil
	}

	// PowerShell fallback when cmd dir returned nothing.
	script := fmt.Sprintf(
		`[Console]::OutputEncoding = [Text.UTF8Encoding]::UTF8; $OutputEncoding = [Console]::OutputEncoding; $root=%s; Get-ChildItem -LiteralPath $root -Recurse -File -Force -ErrorAction Stop | ForEach-Object { Write-Output ($_.FullName + '|' + $_.Length) }`,
		psSingleQuote(rootWin),
	)
	const psExe = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	cmdLine := psExe + ` -NoProfile -NonInteractive -EncodedCommand ` + powershellEncodedCommand(script)
	out, err := RunSSHStdoutCmd(ctx, src, cmdLine)
	if err != nil && len(paths) == 0 {
		return nil, fmt.Errorf("windows directory list: %w", err)
	}
	out = normalizeSSHStdout(out)

	var psPaths []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		absWin, _, ok := strings.Cut(line, "|")
		if !ok {
			absWin = line
		}
		absWin = strings.TrimSpace(absWin)
		if absWin == "" {
			continue
		}
		psPaths = append(psPaths, absWin)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parse windows list output: %w", err)
	}
	psFiles := listedFilesFromWindowsPaths(layout, m, psPaths)
	if len(psFiles) > 0 {
		return psFiles, nil
	}
	if len(paths) == 0 && len(psPaths) == 0 {
		return nil, fmt.Errorf("windows directory list: no files under %q (check path on host)", rootWin)
	}
	return nil, fmt.Errorf("windows directory list: host returned %d path(s) but none matched scope under %q", len(paths)+len(psPaths), rootWin)
}

func windowsDirListPaths(ctx context.Context, src config.ResolvedSource, rootWin string) ([]string, error) {
	safe := strings.ReplaceAll(rootWin, `"`, `""`)
	cmdLine := fmt.Sprintf(`cmd /c dir /s /b /a-d "%s"`, safe)
	out, err := RunSSHStdoutCmd(ctx, src, cmdLine)
	if err != nil {
		return nil, fmt.Errorf("cmd dir list: %w", err)
	}
	out = normalizeSSHStdout(out)
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

func listedFilesFromWindowsPaths(layout config.DirectoryLayout, m *dirsel.Matcher, absPaths []string) []types.ListedBackupFile {
	var files []types.ListedBackupFile
	for _, absWin := range absPaths {
		absPath := strings.ReplaceAll(absWin, `\`, "/")
		relSource, err := config.RelToSource(layout, absPath, true)
		if err != nil {
			continue
		}
		relDir, err := config.RelToDir(layout, absPath, true)
		if err != nil {
			continue
		}
		sizeBytes := int64(1) // unknown until inspect; passes min_file_bytes: 1
		if !m.Accept(relDir, sizeBytes) {
			continue
		}
		files = append(files, types.ListedBackupFile{
			RelativeToSource: relSource,
			RelativeToDir:    relDir,
			SizeBytes:        sizeBytes,
		})
	}
	return files
}

// normalizeSSHStdout decodes UTF-16 LE console output from Windows PowerShell over SSH when needed.
func normalizeSSHStdout(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		b = b[2:]
	}
	if len(b) >= 2 && len(b)%2 == 0 && b[0] == 0 && b[1] != 0 && b[2] == 0 {
		u16 := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			u16 = append(u16, uint16(b[i])|uint16(b[i+1])<<8)
		}
		runes := utf16.Decode(u16)
		return []byte(string(runes))
	}
	return b
}

func powershellEncodedCommand(script string) string {
	u16 := utf16.Encode([]rune(script))
	buf := make([]byte, 0, len(u16)*2)
	for _, r := range u16 {
		buf = append(buf, byte(r), byte(r>>8))
	}
	return base64.StdEncoding.EncodeToString(buf)
}