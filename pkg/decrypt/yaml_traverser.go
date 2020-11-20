package decrypt

import (
	"context"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/valuemodifier"
)

type YAMLTraverserConfig struct {
	Decrypter Decrypter
}

type YAMLTraverser struct {
	decrypter Decrypter
}

func NewYAMLTraverser(config YAMLTraverserConfig) (*YAMLTraverser, error) {
	if config.Decrypter == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Decrypter must not be empty", config)
	}

	t := &YAMLTraverser{
		decrypter: config.Decrypter,
	}

	return t, nil
}

func (t *YAMLTraverser) Traverse(ctx context.Context, yamlData []byte) ([]byte, error) {
	var err error
	var modifier *valuemodifier.Service
	{
		c := valuemodifier.Config{
			ValueModifiers: []valuemodifier.ValueModifier{
				newValueModifier(ctx, t.decrypter),
			},
		}

		modifier, err = valuemodifier.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	decrypted, err := modifier.Traverse(yamlData)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return decrypted, nil
}
