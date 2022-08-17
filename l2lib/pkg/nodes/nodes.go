package nodes

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/test-network-function/l2discovery/l2lib/pkg/l2client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodesSelector represent the label selector used to filter impacted nodes.
var NodesSelector string

func init() {
	NodesSelector = os.Getenv("NODES_SELECTOR")
}

type NodeTopology struct {
	NodeName      string
	InterfaceList []string
	NodeObject    *corev1.Node
}

func LabelNode(nodeName, key, value string) (*corev1.Node, error) {
	NodeObject, err := l2client.Client.K8sClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	NodeObject.Labels[key] = value
	NodeObject, err = l2client.Client.K8sClient.CoreV1().Nodes().Update(context.Background(), NodeObject, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return NodeObject, nil
}

func IsSingleNodeCluster() (bool, error) {
	nodes, err := l2client.Client.K8sClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	return len(nodes.Items) == 1, nil
}

// expectedReachabilityStatus true means test if the node is reachable, false means test if the node is unreachable
func WaitForNodeReachability(node *corev1.Node, timeout time.Duration, expectedReachabilityStatus bool) {
	isCurrentlyReachable := false

	for start := time.Now(); time.Since(start) < timeout; {
		isCurrentlyReachable = IsNodeReachable(node)

		if isCurrentlyReachable == expectedReachabilityStatus {
			break
		}
		if isCurrentlyReachable {
			logrus.Printf("The node %s is reachable via ping", node.Name)
		} else {
			logrus.Printf("The node %s is unreachable via ping", node.Name)
		}
		time.Sleep(time.Second)
	}
	if expectedReachabilityStatus {
		logrus.Printf("The node %s is reachable via ping", node.Name)
	} else {
		logrus.Printf("The node %s is unreachable via ping", node.Name)
	}
}

func IsNodeReachable(node *corev1.Node) bool {
	const (
		timeout20s = 20 * time.Second
	)
	_, err := ExecAndLogCommand(false, timeout20s, "ping", "-c", "3", "-W", "10", node.Name)

	return err == nil
}

func ExecAndLogCommand(logCommand bool, timeout time.Duration, name string, arg ...string) ([]byte, error) {
	// Create a new context and add a timeout to it
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)

	defer cancel() // The cancel should be deferred so resources are cleaned up

	if logCommand {
		logrus.Printf("run command '%s %v'", name, arg)
	}

	out, err := exec.CommandContext(ctx, name, arg...).Output()

	// We want to check the context error to see if the timeout was executed.
	// The error returned by cmd.Output() will be OS specific based on what
	// happens when a process is killed.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("command '%s %v' failed because of the timeout", name, arg)
	}

	if logCommand {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			log.Printf("err=%v:\n  stderr=%s\n  output=%s\n", err, exitError.Stderr, string(out))
		}
	}

	return out, err
}
