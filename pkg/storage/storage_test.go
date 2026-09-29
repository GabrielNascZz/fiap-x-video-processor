package storage

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidVideoFile(t *testing.T) {
	validFiles := []string{"video.mp4", "movie.AVI", "clip.mov", "test.mkv"}
	for _, f := range validFiles {
		if !IsValidVideoFile(f) {
			t.Errorf("Esperado arquivo %s ser válido", f)
		}
	}

	invalidFiles := []string{"image.png", "doc.pdf", "script.sh", "archive.zip"}
	for _, f := range invalidFiles {
		if IsValidVideoFile(f) {
			t.Errorf("Esperado arquivo %s ser inválido", f)
		}
	}
}

func TestCreateZipArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zip_test_*")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file1 := filepath.Join(tempDir, "frame_0001.png")
	file2 := filepath.Join(tempDir, "frame_0002.png")
	_ = os.WriteFile(file1, []byte("fake frame 1 content"), 0644)
	_ = os.WriteFile(file2, []byte("fake frame 2 content"), 0644)

	zipOutput := filepath.Join(tempDir, "output.zip")
	err = CreateZipArchive([]string{file1, file2}, zipOutput)
	if err != nil {
		t.Fatalf("Erro ao criar ZIP: %v", err)
	}

	r, err := zip.OpenReader(zipOutput)
	if err != nil {
		t.Fatalf("Erro ao ler ZIP gerado: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Errorf("Esperado 2 arquivos no ZIP, obtido %d", len(r.File))
	}
}
