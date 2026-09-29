package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	fmt.Println("🚀 Iniciando FIAP X Video Processor (API Gateway)...")
	cmd := exec.Command("go", "run", "./cmd/api")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Erro ao executar API Gateway: %v\n", err)
	}
}
