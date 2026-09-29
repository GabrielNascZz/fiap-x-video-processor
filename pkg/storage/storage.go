package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/config"
)

func EnsureStorageDirs(cfg *config.Config) error {
	dirs := []string{cfg.StorageUploads, cfg.StorageOutputs, cfg.StorageTemp}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("erro ao criar diretório %s: %w", dir, err)
		}
	}
	return nil
}

func IsValidVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := []string{".mp4", ".avi", ".mov", ".mkv", ".wmv", ".flv", ".webm"}
	for _, validExt := range validExts {
		if ext == validExt {
			return true
		}
	}
	return false
}

func ProcessVideoToZip(videoPath, timestamp string, cfg *config.Config) (string, int, error) {
	tempDir := filepath.Join(cfg.StorageTemp, timestamp)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", 0, fmt.Errorf("falha ao criar diretório temporário: %w", err)
	}
	defer os.RemoveAll(tempDir)

	framePattern := filepath.Join(tempDir, "frame_%04d.png")

	// Execute ffmpeg to extract 1 frame per second
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", "fps=1",
		"-y",
		framePattern,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("erro na execução do ffmpeg: %w (output: %s)", err, string(output))
	}

	frames, err := filepath.Glob(filepath.Join(tempDir, "*.png"))
	if err != nil || len(frames) == 0 {
		return "", 0, fmt.Errorf("nenhum frame foi extraído do vídeo (%s)", videoPath)
	}

	zipFilename := fmt.Sprintf("frames_%s.zip", timestamp)
	zipFullPath := filepath.Join(cfg.StorageOutputs, zipFilename)

	if err := CreateZipArchive(frames, zipFullPath); err != nil {
		return "", 0, fmt.Errorf("falha ao criar arquivo zip: %w", err)
	}

	return zipFilename, len(frames), nil
}

func CreateZipArchive(files []string, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, file := range files {
		if err := addFileToZip(zipWriter, file); err != nil {
			return err
		}
	}
	return nil
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filepath.Base(filename)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}
