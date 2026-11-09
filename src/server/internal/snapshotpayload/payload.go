package snapshotpayload

import (
	"fmt"
	"io"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const MaxSize int64 = 64 << 20

func NewFile(payload []byte) (*filesystem.File, error) {
	file, err := filesystem.NewFileFromBytes(payload, "snapshot.json")
	if err != nil {
		return nil, fmt.Errorf("create snapshot payload file: %w", err)
	}
	return file, nil
}

func Read(app core.App, record *core.Record) ([]byte, error) {
	name := record.GetString("payload")
	if name == "" {
		return nil, fmt.Errorf("snapshot %s has no payload file", record.Id)
	}
	fsys, err := app.NewFilesystem()
	if err != nil {
		return nil, fmt.Errorf("open snapshot payload storage: %w", err)
	}
	// The payload is fully read into memory below before this returns, so a
	// failure to close the filesystem handle cannot corrupt or lose data;
	// it is safe to discard.
	defer func() { _ = fsys.Close() }()
	return readPayloadFile(fsys, record.BaseFilesPath()+"/"+name)
}

// readPayloadFile reads one stored payload, up to the size limit. A payload
// over the limit is refused rather than read into memory.
func readPayloadFile(fsys *filesystem.System, path string) ([]byte, error) {
	reader, err := fsys.GetReader(path)
	if err != nil {
		return nil, fmt.Errorf("open snapshot payload: %w", err)
	}
	// Same reasoning as Read: the payload is fully read below before this
	// returns, so a close failure cannot affect the data already returned.
	defer func() { _ = reader.Close() }()
	payload, err := io.ReadAll(io.LimitReader(reader, MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read snapshot payload: %w", err)
	}
	if int64(len(payload)) > MaxSize {
		return nil, fmt.Errorf("snapshot payload exceeds %d bytes", MaxSize)
	}
	return payload, nil
}
