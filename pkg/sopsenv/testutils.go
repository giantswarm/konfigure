package sopsenv

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgofake "k8s.io/client-go/kubernetes/fake"

	"github.com/giantswarm/konfigure/pkg/sopsenv/key"
)

func SetupNewSopsEnvironmentFromFakeKubernetes(secrets []*corev1.Secret) (*SOPSEnv, error) {
	var se *SOPSEnv
	var err error
	{
		k8sObj := make([]runtime.Object, 0)
		for _, sec := range secrets {
			k8sObj = append(k8sObj, sec)
		}

		client := clientgofake.NewClientset(k8sObj...)

		se, err = NewSOPSEnv(SOPSEnvConfig{
			K8sClient:  client,
			KeysDir:    "",
			KeysSource: key.KeysSourceKubernetes,
			Logger:     logr.Discard(),
		})
		if err != nil {
			return nil, err
		}
	}

	if len(secrets) != 0 {
		err = se.Setup(context.TODO())
		if err != nil {
			return nil, err
		}
	}

	return se, nil
}
