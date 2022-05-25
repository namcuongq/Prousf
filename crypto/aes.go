package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func AESEncrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

func AESDecrypt(key []byte, cryptoText []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(cryptoText) < aes.BlockSize {
		return nil, fmt.Errorf("message error")
	}

	iv := cryptoText[:aes.BlockSize]
	cryptoText = cryptoText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	stream.XORKeyStream(cryptoText, cryptoText)

	return cryptoText, nil
}
