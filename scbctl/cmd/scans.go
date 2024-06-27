// SPDX-FileCopyrightText: the secureCodeBox authors
//
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	v1 "github.com/secureCodeBox/secureCodeBox/operator/apis/execution/v1"
	kubernetes2 "github.com/secureCodeBox/secureCodeBox/scbctl/pkg"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubeconfigArgs                           = genericclioptions.NewConfigFlags(false)
	clientProvider kubernetes2.ClientProvider = &kubernetes2.DefaultClientProvider{}
	scheme                                   = runtime.NewScheme()
)

func init() {
	utilruntime.Must(v1.AddToScheme(scheme))
}

var ScanCmd = &cobra.Command{
	Use:   "scan [name] [target]",
	Short: "Create a new scanner",
	Long:  `Create a new execution (Scan) in the default namespace if no namespace is provided`,
	Example: `
	# Create a new scan
	scbctl scan nmap 
	# Create in a different namespace
	scbctl scan nmap scanme.nmap.org --namespace foobar
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New("You must specify the name of the scan and the target")
		}

		scanName := args[0]
		target := args[1]

		fmt.Println("🎬 Initializing Kubernetes client")

		kubeclient, namespace, err := clientProvider.GetClient(kubeconfigArgs)
		if err != nil {
			return fmt.Errorf("Error initializing Kubernetes client: %s", err)
		}

		fmt.Printf("🆕 Creating a new scan with name '%s' and target '%s'\n", scanName, target)

		scan := &v1.Scan{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Scan",
				APIVersion: "execution.securecodebox.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      scanName,
				Namespace: namespace,
			},
			Spec: v1.ScanSpec{
				ScanType: scanName,
				Parameters: []string{
					target,
				},
			},
		}

		fmt.Println("🔁 Launching the scan")

		err = kubeclient.Create(context.TODO(), scan)
		if err != nil {
			return fmt.Errorf("Failed to create Scan: %s", err)
		}

		fmt.Printf("🚀 Successfully created a new Scan '%s'\n", args[0])

		follow, err := cmd.Flags().GetBool("follow")
    if err != nil {
        return fmt.Errorf("Error reading follow flag: %s", err)
    }

    if follow {
        fmt.Println("📡 Following the scan logs")
        err = followScanLogs(context.TODO(), kubeclient, namespace, scanName)
        if err != nil {
            return fmt.Errorf("Error following scan logs: %s", err)
        }
    }
		fmt.Printf("Scanner Ready")

		return nil

	},
}

func followScanLogs(ctx context.Context, kubeclient client.Client, namespace, scanName string) error {
	// Find the job associated with the scan
	jobList := &batchv1.JobList{}
  // err := kubeclient.List(ctx, jobList)
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Error listing jobs: %v\n", err)
	// 	os.Exit(1)
	// }
	// labelSelector := client.MatchingLabels{"securecodebox.io/scan-name": scanName}
	// // fmt.Printf("label %s,", labelSelector)
	// // fmt.Printf("jobname %s,", jobList)
 
	for {
			err := kubeclient.List(ctx, jobList, client.InNamespace(namespace))
			if err != nil {
					return fmt.Errorf("error listing jobs: %s", err)
			}

			if len(jobList.Items) == 0 {
				fmt.Println("No jobs found, retrying...")
				time.Sleep(2 * time.Second)
				continue
			}

			var job *batchv1.Job
			for _, j := range jobList.Items {
					if strings.HasPrefix(j.Name, fmt.Sprintf("scan-%s", scanName)) {
							job = &j
							break
					}
			}

			if job == nil {
					fmt.Println("Waiting for job to be created...")
					time.Sleep(2 * time.Second)
					continue
			}

			jobName := job.Name
			containerName := scanName // Assuming container name matches scan name

			fmt.Printf("📡 Streaming logs for job '%s' and container '%s'\n", jobName, containerName)

			// Execute kubectl logs command
			cmd := exec.CommandContext(ctx, "kubectl", "logs", fmt.Sprintf("job/%s", jobName), containerName, "--follow", "-n", namespace)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
					return fmt.Errorf("error streaming logs: %s", err)
			}

			break
	}

	return nil
}
