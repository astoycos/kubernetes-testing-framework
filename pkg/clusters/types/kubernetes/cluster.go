package kind

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"


	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Generic Kubernetes Cluster
// -----------------------------------------------------------------------------

const (
	// GenericClusterType indicates that the Kubernetes cluster was generic.
	GenericClusterType clusters.Type = "generic"
)

// Cluster is a clusters.Cluster implementation for any Kubernetes cluster, it does NOT
// support the builder interface.
type Cluster struct {
	name       string
	client     *kubernetes.Clientset
	cfg        *rest.Config
	addons     clusters.Addons
	deployArgs []string
	l          *sync.RWMutex
}

// New provides a new clusters.Cluster backed by a Generic Kubernetes Cluster.
func New(ctx context.Context) (*Cluster, error) {
	deployArgs := make([]string, 0)

	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	if err != nil {
		kubeConfig :=
			clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			panic(err)
		}

	} 

	kc, err := kubernetes.NewForConfig(cfg)

	cluster := &Cluster{
		name:       string(GenericClusterType),
		client:     kc,
		cfg:        cfg,
		addons:     make(clusters.Addons),
		deployArgs: deployArgs,
		l:          &sync.RWMutex{},
	}
	
	return cluster, err
}

// -----------------------------------------------------------------------------
// Generic Cluster - Cluster Implementation
// -----------------------------------------------------------------------------

func (c *Cluster) Name() string {
	return c.name
}

func (c *Cluster) Type() clusters.Type {
	return GenericClusterType
}

func (c *Cluster) Version() (semver.Version, error) {
	versionInfo, err := c.Client().ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return semver.Parse(strings.TrimPrefix(versionInfo.String(), "v"))
}

func (c *Cluster) Cleanup(ctx context.Context) error {
	c.l.Lock()
	defer c.l.Unlock()

	return nil
}

func (c *Cluster) Client() *kubernetes.Clientset {
	return c.client
}

func (c *Cluster) Config() *rest.Config {
	return c.cfg
}

func (c *Cluster) GetAddon(name clusters.AddonName) (clusters.Addon, error) {
	c.l.RLock()
	defer c.l.RUnlock()

	for addonName, addon := range c.addons {
		if addonName == name {
			return addon, nil
		}
	}

	return nil, fmt.Errorf("addon %s not found", name)
}

func (c *Cluster) ListAddons() []clusters.Addon {
	c.l.RLock()
	defer c.l.RUnlock()

	addonList := make([]clusters.Addon, 0, len(c.addons))
	for _, v := range c.addons {
		addonList = append(addonList, v)
	}

	return addonList
}

func (c *Cluster) DeployAddon(ctx context.Context, addon clusters.Addon) error {
	c.l.Lock()
	if _, ok := c.addons[addon.Name()]; ok {
		c.l.Unlock()
		return fmt.Errorf("addon component %s is already loaded into cluster %s", addon.Name(), c.Name())
	}
	c.addons[addon.Name()] = addon
	c.l.Unlock()

	return addon.Deploy(ctx, c)
}

func (c *Cluster) DeleteAddon(ctx context.Context, addon clusters.Addon) error {
	c.l.Lock()
	defer c.l.Unlock()

	if _, ok := c.addons[addon.Name()]; !ok {
		return nil
	}

	if err := addon.Delete(ctx, c); err != nil {
		return err
	}

	delete(c.addons, addon.Name())

	return nil
}

// DumpDiagnostics produces diagnostics data for the cluster at a given time.
// It uses the provided meta string to write to meta.txt file which will allow
// for diagnostics identification.
// It returns the path to directory containing all the diagnostic files and an error.
func (c *Cluster) DumpDiagnostics(ctx context.Context, meta string) (string, error) {
	// create a tempdir
	outDir, err := os.MkdirTemp(os.TempDir(), "ktf-diag-")
	if err != nil {
		return "", err
	}

	err = clusters.DumpDiagnostics(ctx, c, meta, outDir)
	return outDir, err
}
