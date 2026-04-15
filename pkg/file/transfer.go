package file

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Transfer handles file upload/download operations
type Transfer struct {
	chunkSize int
}

// NewTransfer creates a new file transfer handler
func NewTransfer(chunkSize int) *Transfer {
	if chunkSize <= 0 {
		chunkSize = 64 * 1024 // 64KB default
	}
	return &Transfer{chunkSize: chunkSize}
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	Path      string
	Size      int64
	Checksum  string // SHA256 hex
	Offset    int64  // For resume
	Timestamp int64
}

// DownloadRequest represents a file download request
type DownloadRequest struct {
	Path   string
	Offset int64 // For resume
}

// TransferResponse represents server response
type TransferResponse struct {
	OK        bool
	Message   string
	Size      int64
	Checksum  string // SHA256 hex
	ChunkSize int
}

// Chunk represents a file data chunk
type Chunk struct {
	Path   string
	Offset int64
	Data   []byte
	IsEOF  bool
}

// Ack represents transfer acknowledgment
type Ack struct {
	Path   string
	Offset int64
	OK     bool
}

// Upload handles file upload to local path
func (t *Transfer) Upload(req *UploadRequest, dataChan <-chan *Chunk) error {
	// Check if resuming
	flags := os.O_CREATE | os.O_WRONLY
	if req.Offset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(req.Path, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Seek to offset for resume
	if req.Offset > 0 {
		if _, err := file.Seek(req.Offset, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek: %w", err)
		}
	}

	hash := sha256.New()
	writer := io.MultiWriter(file, hash)
	var totalWritten int64

	for chunk := range dataChan {
		if _, err := writer.Write(chunk.Data); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
		totalWritten += int64(len(chunk.Data))

		if chunk.IsEOF {
			break
		}
	}

	// Verify checksum if provided
	if req.Checksum != "" {
		actualChecksum := hex.EncodeToString(hash.Sum(nil))
		if actualChecksum != req.Checksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", req.Checksum, actualChecksum)
		}
	}

	// Set modification time
	if req.Timestamp > 0 {
		mtime := time.Unix(req.Timestamp, 0)
		os.Chtimes(req.Path, mtime, mtime)
	}

	return nil
}

// Download handles file download from local path
func (t *Transfer) Download(req *DownloadRequest, chunkChan chan<- *Chunk) error {
	file, err := os.Open(req.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Seek to offset
	if req.Offset > 0 {
		if _, err := file.Seek(req.Offset, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek: %w", err)
		}
	}

	buf := make([]byte, t.chunkSize)
	currentOffset := req.Offset

	for {
		n, err := file.Read(buf)
		if n > 0 {
			chunk := &Chunk{
				Path:   req.Path,
				Offset: currentOffset,
				Data:   make([]byte, n),
			}
			copy(chunk.Data, buf[:n])

			select {
			case chunkChan <- chunk:
				currentOffset += int64(n)
			default:
				return fmt.Errorf("chunk channel full")
			}
		}

		if err == io.EOF {
			// Send EOF chunk
			chunkChan <- &Chunk{
				Path:   req.Path,
				Offset: currentOffset,
				IsEOF:  true,
			}
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}

	return nil
}

// GetFileInfo returns file information for download request
func (t *Transfer) GetFileInfo(path string) (*TransferResponse, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &TransferResponse{OK: false, Message: err.Error()}, nil
	}

	if info.IsDir() {
		return &TransferResponse{OK: false, Message: "path is a directory"}, nil
	}

	// Calculate checksum
	checksum, err := calculateChecksum(path)
	if err != nil {
		return &TransferResponse{OK: false, Message: err.Error()}, nil
	}

	return &TransferResponse{
		OK:        true,
		Size:      info.Size(),
		Checksum:  checksum,
		ChunkSize: t.chunkSize,
	}, nil
}

// ValidatePath ensures path is safe (no directory traversal)
func ValidatePath(basePath, targetPath string) (string, error) {
	// Clean and join paths
	fullPath := filepath.Join(basePath, targetPath)
	cleanPath := filepath.Clean(fullPath)

	// Ensure path is within base directory
	if !filepath.HasPrefix(cleanPath, filepath.Clean(basePath)) {
		return "", fmt.Errorf("path traversal detected")
	}

	return cleanPath, nil
}

// calculateChecksum calculates SHA256 checksum of a file
func calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ResumePoint returns the position to resume upload/download
func ResumePoint(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
