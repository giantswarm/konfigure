package k8sresource

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	Client client.Client
	Logger micrologger.Logger
}

type Service struct {
	client client.Client
	logger micrologger.Logger
}

func New(config Config) (*Service, error) {
	if config.Client == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Client must not be empty", config)
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}

	s := &Service{
		client: config.Client,
		logger: config.Logger,
	}

	return s, nil
}

func (s *Service) EnsureCreated(ctx context.Context, hashAnnotation string, desired Object) error {
	s.logger.Debugf(ctx, "ensuring %#q %#q", Kind(desired), NamespacedName(desired))

	err := setHash(hashAnnotation, desired)
	if err != nil {
		return microerror.Mask(err)
	}

	t := reflect.TypeOf(desired).Elem()
	current := reflect.New(t).Interface().(Object)
	err = s.client.Get(ctx, NamespacedName(desired), current)
	if apierrors.IsNotFound(err) {
		err = s.client.Create(ctx, desired)
		if err != nil {
			return microerror.Mask(err)
		}

		s.logger.Debugf(ctx, "created %#q %#q", Kind(desired), NamespacedName(desired))
		return nil
	} else if err != nil {
		return microerror.Mask(err)
	}

	h1, ok1 := getAnnotation(desired, hashAnnotation)
	h2, ok2 := getAnnotation(current, hashAnnotation)

	if ok1 && ok2 && h1 == h2 {
		s.logger.Debugf(ctx, "object %#q %#q is up to date", Kind(desired), NamespacedName(desired))
		return nil
	}

	err = s.client.Update(ctx, desired)
	if err != nil {
		return microerror.Mask(err)
	}

	s.logger.Debugf(ctx, "updated %#q %#q", Kind(desired), NamespacedName(desired))
	return nil
}

func getAnnotation(o Object, key string) (string, bool) {
	a := o.GetAnnotations()
	if a == nil {
		return "", false
	}

	s, ok := a[key]
	return s, ok
}

func setAnnotation(o Object, key, val string) {
	a := o.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}

	a[key] = val
	o.SetAnnotations(a)
}

func setHash(annotation string, o Object) error {
	bytes, err := json.Marshal(o)
	if err != nil {
		return microerror.Mask(err)
	}

	sum := sha256.Sum256(bytes)
	setAnnotation(o, annotation, fmt.Sprintf("%x", sum))

	return nil
}
