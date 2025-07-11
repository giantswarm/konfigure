package testutils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockFilesystem struct {
	tempDirPath string

	ExpectedConfigmap string
	ExpectedSecret    string
}

type TestFile struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

func NewMockFilesystem(temporaryDirectory, caseFile string) *MockFilesystem {
	mockFileSystem := MockFilesystem{
		tempDirPath: temporaryDirectory,
	}
	for _, p := range []string{"default", "installations", "include"} {
		if err := os.MkdirAll(path.Join(temporaryDirectory, p), 0750); err != nil {
			panic(err)
		}
	}

	rawData, err := os.ReadFile(path.Clean(caseFile))
	if err != nil {
		panic(err)
	}

	// Necessary to avoid cutting SOPS-encrypted files
	splitFiles := strings.Split(string(rawData), "\n---\n")

	for _, rawYaml := range splitFiles {
		rawYaml = rawYaml + "\n"

		file := TestFile{}
		if err := yaml.Unmarshal([]byte(rawYaml), &file); err != nil {
			panic(err)
		}

		p := path.Join(temporaryDirectory, file.Path)
		dir, filename := path.Split(p)

		switch filename {
		case "configmap-values.yaml.golden":
			mockFileSystem.ExpectedConfigmap = file.Data
			continue
		case "secret-values.yaml.golden":
			mockFileSystem.ExpectedSecret = file.Data
			continue
		}

		if err := os.MkdirAll(dir, 0750); err != nil {
			panic(err)
		}

		err := os.WriteFile(p, []byte(file.Data), 0644) // nolint:gosec
		if err != nil {
			panic(err)
		}
	}

	return &mockFileSystem
}

func (fs *MockFilesystem) ReadFile(filepath string) ([]byte, error) {
	data, err := os.ReadFile(path.Clean(path.Join(fs.tempDirPath, filepath)))
	if err != nil {
		return []byte{}, &NotFoundError{message: fmt.Sprintf("%q not found", filepath)}
	}
	return data, nil
}

func (fs *MockFilesystem) ReadDir(dirpath string) ([]fs.DirEntry, error) {
	p := path.Join(fs.tempDirPath, dirpath)
	return os.ReadDir(path.Clean(p))
}

func UntarFile(path, name string) error {
	keysArchive, err := os.Open(fmt.Sprintf("%s/%s", path, name))
	if err != nil {
		return err
	}

	gzr, err := gzip.NewReader(keysArchive)
	if err != nil {
		return err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		key := filepath.Join(path, header.Name) //#nosec G305
		key = filepath.Clean(key)
		modeInt32 := uint32(header.Mode) //#nosec G115
		file, err := os.OpenFile(key, os.O_CREATE|os.O_RDWR, os.FileMode(modeInt32))
		if err != nil {
			return err
		}

		if _, err := io.Copy(file, tr); err != nil { //nolint:gosec
			return err
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
	path = filepath.Clean(path)
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return bytes.TrimSuffix(file, []byte("\n"))
}
