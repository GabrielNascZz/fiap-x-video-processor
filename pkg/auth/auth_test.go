package auth

import (
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "minhasenha123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Erro ao gerar hash de senha: %v", err)
	}

	if hash == password {
		t.Errorf("A senha em hash não deve ser igual em texto plano")
	}

	if !CheckPasswordHash(password, hash) {
		t.Errorf("Verificação de hash de senha falhou para senha válida")
	}

	if CheckPasswordHash("senhaincorreta", hash) {
		t.Errorf("Verificação de hash de senha deve falhar para senha inválida")
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := "test-secret-key-123"
	userID := uint(42)
	username := "gabriel"

	token, err := GenerateToken(userID, username, secret)
	if err != nil {
		t.Fatalf("Erro ao gerar token JWT: %v", err)
	}

	if token == "" {
		t.Fatalf("Token JWT gerado é vazio")
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("Erro ao validar token JWT válido: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Esperado UserID %d, obtido %d", userID, claims.UserID)
	}

	if claims.Username != username {
		t.Errorf("Esperado Username %s, obtido %s", username, claims.Username)
	}

	_, err = ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Errorf("Validação deve falhar com chave secreta incorreta")
	}
}
