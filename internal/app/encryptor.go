package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const passPhrase = "passphrasewhichneedstobe32bytes!"

type Encryptor struct {
	ctx               context.Context
	logger            *zap.Logger
	sem               *semaphore.Weighted
	encryptedRootPath string
	decryptedRootPath string
}

func NewEncryptor(ctx context.Context, logger *zap.Logger, sem *semaphore.Weighted, encryptedRootPath string, decryptedRootPath string) *Encryptor {
	if err := os.MkdirAll(encryptedRootPath, os.ModePerm); err != nil && !os.IsExist(err) {
		logger.Fatal("FATAL %v", zap.Error(err))
	}
	if err := os.MkdirAll(decryptedRootPath, os.ModePerm); err != nil && !os.IsExist(err) {
		logger.Fatal("FATAL %v", zap.Error(err))
	}

	return &Encryptor{
		ctx:               ctx,
		logger:            logger,
		sem:               sem,
		encryptedRootPath: encryptedRootPath,
		decryptedRootPath: decryptedRootPath,
	}
}

func (e *Encryptor) Encrypt(toEncrypt <-chan DTO) <-chan DTO {
	resultStream := make(chan DTO)
	go func() {
		defer close(resultStream)
		for te := range toEncrypt {
			select {
			case <-e.ctx.Done():
				return
			case resultStream <- e.encryptFile(te):
			}
		}
	}()
	return resultStream
}

func (e *Encryptor) encryptFile(toEncrypt DTO) DTO {
	// get and release local semaphore
	semErr := e.sem.Acquire(e.ctx, 1)
	defer func() {
		if semErr == nil {
			e.sem.Release(1)
		}
	}()
	if semErr != nil {
		e.logger.Error("encryptFile: skipping, unable to obtain local semaphore")
		return NewDTOInstance(fmt.Errorf("error while encrypting: %v", errors.New("skipping, unable to obtain local semaphore")), "")
	}

	// start encryption process
	plain, err := ioutil.ReadFile(toEncrypt.Content())
	if err != nil {
		e.logger.Error("encryptFile: error reading encrypted file", zap.Error(err))
		return NewDTOInstance(fmt.Errorf("error while encrypting: %v", err), "")
	}

	key := []byte(passPhrase)
	block, err := aes.NewCipher(key)
	if err != nil {
		e.logger.Error("encryptFile: error creating new cipher", zap.Error(err))
		return NewDTOInstance(fmt.Errorf("error while encrypting: %v", err), "")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		e.logger.Error("encryptFile: error setting gcm", zap.Error(err))
		return NewDTOInstance(fmt.Errorf("error while encrypting: %v", err), "")
	}

	// Never use more than 2^32 random nonces with a given key
	// because of the risk of repeat.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		e.logger.Error("encryptFile: error reading nonce", zap.Error(err))
		return NewDTOInstance(fmt.Errorf("error while encrypting: %v", err), "")
	}

	ciphered := gcm.Seal(nonce, nonce, plain, nil)
	// Save back to file
	encryptedFilePath := path.Join(e.encryptedRootPath, strings.Replace(path.Base(toEncrypt.Content()), path.Ext(toEncrypt.Content()), "", -1))
	err = ioutil.WriteFile(encryptedFilePath, ciphered, 0777)
	if err != nil {
		e.logger.Error("encryptFile: error writing encrypted content to file", zap.Error(err))
		return NewDTOInstance(fmt.Errorf("error while encrypting: %v", err), "")
	}
	return NewDTOInstance(nil, encryptedFilePath)
}

func (e *Encryptor) DecryptFileAs(encryptedFilePath string, extension string) (string, error) {
	key := []byte(passPhrase)
	ciphered, err := ioutil.ReadFile(encryptedFilePath)
	if err != nil {
		e.logger.Error("decryptFileAs: error reading encrypted content", zap.Error(err))
		return "", err
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		e.logger.Error("decryptFileAs: error creating new cipher", zap.Error(err))
		return "", err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		e.logger.Error("decryptFileAs: error creating new cipher", zap.Error(err))
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphered) < nonceSize {
		e.logger.Error("decryptFileAs: encrypted content does not satisfy the gcm nonceSize", zap.Error(err))
		return "", err
	}

	nonce, ciphered := ciphered[:nonceSize], ciphered[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphered, nil)
	if err != nil {
		e.logger.Error("decryptFileAs: error while opening encrypted content with gcm", zap.Error(err))
		return "", err
	}

	// Save back to file
	plainFilePath := path.Join(e.decryptedRootPath, path.Base(encryptedFilePath)+extension)
	err = ioutil.WriteFile(plainFilePath, plain, 0777)
	if err != nil {
		e.logger.Error("decryptFileAs: error writing decrypted content to file", zap.Error(err))
		return "", err
	}
	return plainFilePath, nil
}
