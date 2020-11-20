package decrypt

import (
	"context"

	"github.com/giantswarm/microerror"
)

type valueModifier struct {
	// ctx is here to bypass valuemodifier.ValueModifer interface which
	// doesn't take the context.
	ctx       context.Context
	decrypter Decrypter
}

func newValueModifier(ctx context.Context, d Decrypter) *valueModifier {
	return &valueModifier{
		ctx:       ctx,
		decrypter: d,
	}
}

func (m *valueModifier) Modify(bs []byte) ([]byte, error) {
	bs, err := m.decrypter.Decrypt(m.ctx, bs)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return bs, nil
}
