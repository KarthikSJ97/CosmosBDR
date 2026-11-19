package manifest

import (
	"testing"
	"time"
)

func testMeta() Meta {
	return Meta{
		JobItemID:          "item-1",
		BackedUpAt:         time.Date(2025, 11, 19, 10, 30, 0, 0, time.UTC),
		FileChecksumSHA256: "filehash",
		FileSizeBytes:      1,
		ChunkSizeBytes:     ChunkSizeBytes,
	}
}

func TestBuild_contiguousChunks(t *testing.T) {
	snap, err := Build([]ChunkInput{
		{Index: 0, ChunkHash: "aaa"},
		{Index: 1, ChunkHash: "bbb"},
	}, testMeta())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(snap.Chunks))
	}
	if snap.Chunks[0].Object != "chunks/aaa" || snap.Chunks[1].Object != "chunks/bbb" {
		t.Fatalf("unexpected objects: %+v", snap.Chunks)
	}
	if snap.FileChecksumSHA256 != "filehash" || snap.ChunkSizeBytes != ChunkSizeBytes {
		t.Fatalf("unexpected meta: %+v", snap)
	}
}

func TestBuild_rejectsGap(t *testing.T) {
	_, err := Build([]ChunkInput{
		{Index: 0, ChunkHash: "aaa"},
		{Index: 2, ChunkHash: "bbb"},
	}, testMeta())
	if err == nil {
		t.Fatal("expected error for non-contiguous indices")
	}
}

func TestBuild_requiresChecksum(t *testing.T) {
	meta := testMeta()
	meta.FileChecksumSHA256 = ""
	_, err := Build(nil, meta)
	if err == nil {
		t.Fatal("expected error for missing file checksum")
	}
}

func TestSnapshotObjectKey(t *testing.T) {
	folder := SnapshotFolderName("id-1", time.Date(2025, 11, 19, 10, 30, 0, 0, time.UTC))
	key := SnapshotObjectKey("reports/foo.pdf", folder)
	want := "reports/foo.pdf/snapshots/2025-11-19T10-30-00Z__id-1/manifest.json"
	if key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}