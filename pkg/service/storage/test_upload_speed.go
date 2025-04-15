package storage

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"log/slog"
	"path/filepath"
	"time"
)

const (
	testNumFiles int   = 3
	mibBytes     int64 = 1024 * 1024
	testFileSize       = 200 * mibBytes
)

// TestUploadSpeed measures the average upload speed to the given storage using predefined settings.
// It orchestrates the generation of test data, uploading multiple files, measuring performance,
// and cleaning up resources. Logging is performed using slog.
//
// Parameters:
//   - ctx:      Context for cancellation and deadlines.
//   - storage:  The storage backend implementation (conforming to model.Storage).
//
// Returns:
//   - float64: The average upload speed in MiB/s (Megabytes per second). Returns 0 if the test fails or no files are uploaded.
//   - error:   An error if any critical part of the process fails.
func TestUploadSpeed(ctx context.Context, storage model.Storage) (int, error) {
	tempFolderName := "bandwidth_test"
	slog.Info("Starting bandwidth test",
		slog.String("temp_folder", tempFolderName),
		slog.Int64("file_size_bytes", testFileSize),
		slog.Int("num_files", testNumFiles),
	)

	defer func() {
		if err := DeleteFolder(context.Background(), storage, tempFolderName); err != nil {
			slog.Warn("Failed to delete temporary folder",
				slog.String("folder", tempFolderName),
				slog.Any("error", err),
			)
		}
	}()

	content, _ := generateTestData(testFileSize)

	_, speedsMiBps, _, err := runUploadTestLoop(ctx, storage, tempFolderName, content, testNumFiles)
	if err != nil {
		return 0, fmt.Errorf("upload test loop failed: %w", err)
	}

	// --- Calculate Average Speed ---
	if len(speedsMiBps) == 0 {
		slog.Error("No files were uploaded successfully, cannot calculate average speed")
		return 0, fmt.Errorf("no successful uploads to calculate average speed") // Should ideally be caught by loop error, but safeguard.
	}

	var totalSpeed float64
	for _, speed := range speedsMiBps {
		totalSpeed += speed
	}
	averageSpeedMiBps := totalSpeed / float64(len(speedsMiBps))

	return int(averageSpeedMiBps), nil
}

// generateTestData creates a byte slice of the specified size filled with random data.
func generateTestData(size int64) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("file size must be positive")
	}
	content := make([]byte, size)
	_, _ = rand.Read(content)
	return content, nil
}

// uploadSingleFileTest uploads the given content to the specified fileName using the storage backend
// and returns the time duration it took.
func uploadSingleFileTest(ctx context.Context, storage model.Storage, fileName string, content []byte) (time.Duration, error) {
	startTime := time.Now()
	_ = WriteDataFile(ctx, storage, fileName, content)
	duration := time.Since(startTime)

	return duration, nil
}

// runUploadTestLoop uploads a number of test files sequentially and returns the duration and calculated speed for each successful upload.
func runUploadTestLoop(
	ctx context.Context,
	storage model.Storage,
	tempFolderName string,
	content []byte,
	numFiles int,
) (durations []time.Duration, speedsMiBps []float64, totalDuration time.Duration, err error) {
	fileSize := int64(len(content))
	durations = make([]time.Duration, 0, numFiles)
	speedsMiBps = make([]float64, 0, numFiles)

	for i := 0; i < numFiles; i++ {
		// Check context cancellation before each upload attempt
		select {
		case <-ctx.Done():
			slog.Warn("Context cancelled during upload loop",
				slog.Int("completed_files", i),
				slog.Int("total_files_planned", numFiles),
				slog.Any("error", ctx.Err()),
			)
			// Return partially collected results along with the cancellation error
			err = fmt.Errorf("operation cancelled: %w", ctx.Err())
			return
		default:
		}

		fileName := filepath.Join(tempFolderName, fmt.Sprintf("testfile_%d.dat", i))
		slog.Info("Uploading test file",
			slog.Int("file_index", i+1),
			slog.Int("total_files", numFiles),
			slog.String("filename", fileName),
			slog.Int64("size_bytes", fileSize),
		)

		var uploadDuration time.Duration
		uploadDuration, err = uploadSingleFileTest(ctx, storage, fileName, content)
		if err != nil {
			return
		}

		speed := float64(fileSize) / float64(mibBytes) / uploadDuration.Seconds()
		durations = append(durations, uploadDuration)
		speedsMiBps = append(speedsMiBps, speed)
		totalDuration += uploadDuration

		slog.Info("File upload completed",
			slog.Int("file_index", i+1),
			slog.Int("total_files", numFiles),
			slog.Duration("duration", uploadDuration),
			slog.Float64("speed_MiBps", speed),
		)
	}

	// All uploads completed successfully
	return durations, speedsMiBps, totalDuration, nil
}
