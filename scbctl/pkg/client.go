// SPDX-FileCopyrightText: the secureCodeBox authors
//
// SPDX-License-Identifier: Apache-2.0
package client

import (
	"context"
	"fmt"
	"net/http"

	v1 "github.com/secureCodeBox/secureCodeBox/operator/apis/execution/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	scheme = runtime.NewScheme()
)

type ClientProvider interface {
	GetClient(flags *genericclioptions.ConfigFlags) (client.Client, string, error)
}

type DefaultClientProvider struct{}

func (d *DefaultClientProvider) GetClient(flags *genericclioptions.ConfigFlags) (client.Client, string, error) {
	return GetClient(flags)
}

func init() {
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

func GetClient(flags *genericclioptions.ConfigFlags) (client.Client, string, error) {
	cnfLoader := flags.ToRawKubeConfigLoader()

	cnf, err := cnfLoader.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate config from kubeconfig")
	}

	namespace, _, err := cnfLoader.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("failed to read namespace from kubeconfig")
	}

	utilruntime.Must(v1.AddToScheme(scheme))

	client, err := client.New(cnf, client.Options{Scheme: scheme})
	if err != nil {
		return nil, "", err
	}

	return client, namespace, nil
}

func GetLogs(podName string, namespace string, opts *corev1.PodLogOptions) (string, error) {
	// Get the REST config
	cfg, err := config.GetConfig()
	if err != nil {
			return "", err
	}

	// Set up the GVK for pods
	gvk := schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
	}


	restClient, err := apiutil.RESTClientForGVK(gvk, false, cfg, serializer.NewCodecFactory(scheme), &http.Client{})
	if err != nil {
			return "", err
	}


	req := restClient.
			Get().
			Namespace(namespace).
			Name(podName).
			Resource("pods").
			SubResource("log")

	res := req.Do(context.Background())
	if res.Error() != nil {
			return "", res.Error()
	}

	rawLogs, err := res.Raw()
	if err != nil {
			return "", err
	}

	return string(rawLogs), nil
}
