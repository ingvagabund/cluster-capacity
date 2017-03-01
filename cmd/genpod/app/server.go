/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/kubernetes-incubator/cluster-capacity/cmd/genpod/app/options"
	nspod "github.com/kubernetes-incubator/cluster-capacity/pkg/client"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/utils"
)

func NewGenPodCommand() *cobra.Command {
	opt := options.NewGenPodOptions()
	cmd := &cobra.Command{
		Use:   "genpod --kubeconfig KUBECONFIG --namespace NAMESPACE",
		Short: "Generate pod based on namespace resource limits and node selector annotations",
		Long:  "Generate pod based on namespace resource limits and node selector annotations",
		Run: func(cmd *cobra.Command, args []string) {
			err := Validate(opt)
			if err != nil {
				fmt.Println(err)
				cmd.Help()
				return
			}
			err = Run(opt)
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	opt.AddFlags(cmd.Flags())
	return cmd
}

func Validate(opt *options.GenPodOptions) error {
	if len(opt.Namespace) == 0 {
		return fmt.Errorf("Cluster namespace missing")
	}

	if len(opt.Format) > 0 && opt.Format != "json" && opt.Format != "yaml" {
		return fmt.Errorf("Output format %v not recognized: only json and yaml are allowed", opt.Format)
	}

	return nil
}

func Run(opt *options.GenPodOptions) error {
	var err error
	opt.Master, err = utils.GetMasterFromKubeConfig(opt.Kubeconfig)
	if err != nil {
		return fmt.Errorf("Failed to parse kubeconfig file: %v ", err)
	}
	client, err := getKubeClient(opt.Master, opt.Kubeconfig)
	if err != nil {
		return err
	}

	pod, err := nspod.RetrieveNamespacePod(client, opt.Namespace)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	} else {
		var contentType string
		switch opt.Format {
		case "json":
			contentType = runtime.ContentTypeJSON
		case "yaml":
			contentType = "application/yaml"
		default:
			contentType = "application/yaml"
		}

		info, ok := runtime.SerializerInfoForMediaType(testapi.Default.NegotiatedSerializer().SupportedMediaTypes(), contentType)
		if !ok {
			return fmt.Errorf("serializer for %s not registered", contentType)
		}
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
		encoder := api.Codecs.EncoderForVersion(info.Serializer, gvr.GroupVersion())
		stream, err := runtime.Encode(encoder, pod)

		if err != nil {
			return fmt.Errorf("Failed to create pod: %v", err)
		}
		fmt.Print(string(stream))
	}

	return nil
}

func getKubeClient(master string, config string) (clientset.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(master, config)
	if err != nil {
		return nil, fmt.Errorf("Unable to build config: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Invalid API configuration: %v", err)
	}

	if _, err = kubeClient.Discovery().ServerVersion(); err != nil {
		return nil, fmt.Errorf("Unable to get server version: %v\n", err)
	}
	return kubeClient, nil
}
