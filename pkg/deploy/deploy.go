package deploy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"strings"

	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	InfraStack       = "infra"
	OpniStack        = "opni"
	MINIO_ACCESS_KEY = "MINIO_ACCESS_KEY"
	MINIO_SECRET_KEY = "MINIO_SECRET_KEY"
	MINIO_VERSION    = "MINIO_VERSION"
	NATS_VERSION     = "NATS_VERSION"
	NATS_PASSWORD    = "NATS_PASSWORD"
	NATS_REPLICAS    = "NATS_REPLICAS"
	NATS_MAX_PAYLOAD = "NATS_MAX_PAYLOAD"
	NVIDIA_VERSION   = "NVIDIA_VERSION"
	TRAEFIK_VERSION  = "TRAEFIK_VERSION"
	ES_USER          = "ES_USER"
	ES_PASSWORD      = "ES_PASSWORD"
)

func Install(ctx context.Context, sc *Context, values map[string]string) error {
	// installing infra resources
	logrus.Infof("Deploying infrastructure resources")
	infraObjs, infraOwner, err := objs(InfraStack, values)
	if err != nil {
		return err
	}
	os := objectset.NewObjectSet()
	os.Add(infraObjs...)
	if err := sc.Apply.WithOwner(infraOwner).WithSetID(InfraStack).Apply(os); err != nil {
		return err
	}

	// installing opni stack
	logrus.Infof("Deploying opni stack")
	opniObjs, opniOwner, err := objs(OpniStack, values)
	if err != nil {
		return err
	}
	os = objectset.NewObjectSet()
	os.Add(opniObjs...)
	return sc.Apply.WithOwner(opniOwner).WithSetID(OpniStack).Apply(os)
}

func objs(dir string, values map[string]string) ([]runtime.Object, *corev1.ConfigMap, error) {
	owner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dir,
			Namespace: "kube-system",
		},
	}

	cfgSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dir,
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			MINIO_ACCESS_KEY: []byte(values[MINIO_ACCESS_KEY]),
			MINIO_SECRET_KEY: []byte(values[MINIO_ACCESS_KEY]),
			NATS_PASSWORD:    []byte(values[NATS_PASSWORD]),
			ES_PASSWORD:      []byte(values[ES_PASSWORD]),
		},
	}
	objs := []runtime.Object{}
	for _, asset := range AssetNames() {
		if !strings.HasPrefix(asset, dir) {
			continue
		}
		content, err := getManifest(asset)
		if err != nil {
			return nil, nil, err
		}
		newContent := replaceValues(content, values)
		assetObj, err := yamlToObjects(bytes.NewBuffer(newContent))
		if err != nil {
			return nil, nil, err
		}
		objs = append(objs, assetObj...)
	}
	objs = append(objs, owner, cfgSecret)
	return objs, owner, nil
}

func yamlToObjects(in io.Reader) ([]runtime.Object, error) {
	var result []runtime.Object
	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		obj, err := toObjects(raw)
		if err != nil {
			return nil, err
		}

		result = append(result, obj...)
	}

	return result, nil
}

func toObjects(bytes []byte) ([]runtime.Object, error) {
	bytes, err := yamlDecoder.ToJSON(bytes)
	if err != nil {
		return nil, err
	}

	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(bytes, nil, nil)
	if err != nil {
		return nil, err
	}

	if l, ok := obj.(*unstructured.UnstructuredList); ok {
		var result []runtime.Object
		for _, obj := range l.Items {
			copy := obj
			result = append(result, &copy)
		}
		return result, nil
	}

	return []runtime.Object{obj}, nil
}

func getManifest(name string, args ...string) ([]byte, error) {
	asset, err := Asset(name)
	if err != nil {
		return nil, err
	}
	for _, arg := range args {
		kv := strings.Split(arg, "=")
		asset = []byte(strings.Replace(string(asset), "%"+kv[0]+"%", kv[1], 1))
	}
	return asset, nil
}

func replaceValues(content []byte, values map[string]string) []byte {
	contentStr := string(content)
	for k, v := range values {
		contentStr = strings.Replace(contentStr, "%"+k+"%", v, 1)
	}
	return []byte(contentStr)
}

func base64Encode(value string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(value)))
}
