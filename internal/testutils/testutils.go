package testutils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UntarFile(path, name string) error {
	keysArchive, err := os.Open(fmt.Sprintf("%s/%s", path, name))
	if err != nil {
		return microerror.Mask(err)
	}

	gzr, err := gzip.NewReader(keysArchive)
	if err != nil {
		return microerror.Mask(err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return microerror.Mask(err)
		case header == nil:
			continue
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		key := filepath.Join(path, header.Name) //#nosec G305
		modeInt32 := uint32(header.Mode)        //#nosec G115
		file, err := os.OpenFile(key, os.O_CREATE|os.O_RDWR, os.FileMode(modeInt32))
		if err != nil {
			return microerror.Mask(err)
		}

		if _, err := io.Copy(file, tr); err != nil { //nolint:gosec
			return microerror.Mask(err)
		}

		_ = file.Close()
	}
}

func NewSecret(name, namespace string, keys bool, data map[string][]byte) *corev1.Secret {
	s := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{},
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	if keys {
		s.Labels["konfigure.giantswarm.io/data"] = "sops-keys"
	}

	return s
}

func GetFile(path string) []byte {
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return bytes.TrimSuffix(file, []byte("\n"))
}
