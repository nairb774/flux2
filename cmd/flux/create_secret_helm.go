/*
Copyright 2021 The Flux authors

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

package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/flux2/internal/utils"
)

var createSecretHelmCmd = &cobra.Command{
	Use:   "helm [name]",
	Short: "Create or update a Kubernetes secret for Helm repository authentication",
	Long: `
The create secret helm command generates a Kubernetes secret with basic authentication credentials.`,
	Example: `    # Create a Helm authentication secret on disk and encrypt it with Mozilla SOPS

  flux create secret helm repo-auth \
    --namespace=my-namespace \
    --username=my-username \
    --password=my-password \
    --export > repo-auth.yaml

  sops --encrypt --encrypted-regex '^(data|stringData)$' \
    --in-place repo-auth.yaml

  # Create an authentication secret using a custom TLS cert
  flux create secret helm repo-auth \
    --username=username \
    --password=password \
    --cert-file=./cert.crt \
    --key-file=./key.crt \
    --ca-file=./ca.crt
`,
	RunE: createSecretHelmCmdRun,
}

type secretHelmFlags struct {
	username string
	password string
	certFile string
	keyFile  string
	caFile   string
}

var secretHelmArgs secretHelmFlags

func init() {
	createSecretHelmCmd.Flags().StringVarP(&secretHelmArgs.username, "username", "u", "", "basic authentication username")
	createSecretHelmCmd.Flags().StringVarP(&secretHelmArgs.password, "password", "p", "", "basic authentication password")
	createSecretHelmCmd.Flags().StringVar(&secretHelmArgs.certFile, "cert-file", "", "TLS authentication cert file path")
	createSecretHelmCmd.Flags().StringVar(&secretHelmArgs.keyFile, "key-file", "", "TLS authentication key file path")
	createSecretHelmCmd.Flags().StringVar(&secretHelmArgs.caFile, "ca-file", "", "TLS authentication CA file path")

	createSecretCmd.AddCommand(createSecretHelmCmd)
}

func createSecretHelmCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("secret name is required")
	}
	name := args[0]

	secretLabels, err := parseLabels()
	if err != nil {
		return err
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rootArgs.namespace,
			Labels:    secretLabels,
		},
		StringData: map[string]string{},
	}

	if secretHelmArgs.username != "" && secretHelmArgs.password != "" {
		secret.StringData["username"] = secretHelmArgs.username
		secret.StringData["password"] = secretHelmArgs.password
	}

	if secretHelmArgs.certFile != "" && secretHelmArgs.keyFile != "" {
		cert, err := ioutil.ReadFile(secretHelmArgs.certFile)
		if err != nil {
			return fmt.Errorf("failed to read repository cert file '%s': %w", secretHelmArgs.certFile, err)
		}
		secret.StringData["certFile"] = string(cert)

		key, err := ioutil.ReadFile(secretHelmArgs.keyFile)
		if err != nil {
			return fmt.Errorf("failed to read repository key file '%s': %w", secretHelmArgs.keyFile, err)
		}
		secret.StringData["keyFile"] = string(key)
	}

	if secretHelmArgs.caFile != "" {
		ca, err := ioutil.ReadFile(secretHelmArgs.caFile)
		if err != nil {
			return fmt.Errorf("failed to read repository CA file '%s': %w", secretHelmArgs.caFile, err)
		}
		secret.StringData["caFile"] = string(ca)
	}

	if createArgs.export {
		return exportSecret(secret)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := utils.KubeClient(rootArgs.kubeconfig, rootArgs.kubecontext)
	if err != nil {
		return err
	}

	if err := upsertSecret(ctx, kubeClient, secret); err != nil {
		return err
	}
	logger.Actionf("secret '%s' created in '%s' namespace", name, rootArgs.namespace)

	return nil
}
