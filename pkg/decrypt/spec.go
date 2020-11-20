package decrypt

import "context"

type Decrypter interface {
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}
