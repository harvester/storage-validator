package validation

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	harvesterv1beta1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	snapshot "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"github.com/harvester/storage-validator/pkg/api"

	"github.com/rancher/wrangler/v3/pkg/signals"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"kubevirt.io/client-go/kubecli"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	ServerVersionSetting = "server-version"
)

type ValidationRun struct {
	ConfigFile     string
	ctx            context.Context
	Configuration  *api.Configuration
	Report         *api.Report
	createdObjects []client.Object
	cfg            *rest.Config
	clients        HarvesterClient
	pvcName        string // used to track baseline pvc used for snapshots
	vmImageName    string // used to track vmimage created for subsequent vm creation
	vmName         string // used to track vm created for hot plug and snapshot operations
	storageClass   *storagev1.StorageClass
	Version        string
}

type HarvesterClient struct {
	kubevirtClient kubecli.KubevirtClient
	runtimeClient  client.Client
}

func init() {
	// register schemas
	utilruntime.Must(snapshot.AddToScheme(scheme))
	utilruntime.Must(storagev1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(harvesterv1beta1.AddToScheme(scheme))
	utilruntime.Must(cdiv1.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
}

var (
	scheme = runtime.NewScheme()
)

func (v *ValidationRun) Execute() error {
	// initialise context
	v.ctx = signals.SetupSignalContext()

	// read configuration file
	if err := v.readConfig(); err != nil {
		return err
	}

	// initialise reporting structure
	v.Report = &api.Report{
		Configuration: *v.Configuration,
	}

	// generate k8s clients
	if err := v.setupClients(); err != nil {
		return err
	}

	// run preflight checks
	initiateCheck("preflight checks")
	if err := v.preFlightChecks(); err != nil {
		return err
	}
	completedCheck("preflight checks")

	envInfo, err := v.fetchEnvironmentInfo()
	if err != nil {
		return err
	}

	v.Report.EnvironmentInfo = envInfo
	// apply systemwide defaults
	if err := v.applyValidatinoDefaults(); err != nil {
		return err
	}

	if err := v.runChecks(); err != nil {
		logrus.Errorf("validation failed with error: %v", err)
	}

	resultByte, err := yaml.Marshal(v.Report)
	if err != nil {
		return fmt.Errorf("err marshalling result data: %w", err)
	}

	fmt.Println("-------------------------------------")
	fmt.Println(string(resultByte))
	return nil
}

// readConfig will read the configuration file and prep the
func (v *ValidationRun) readConfig() error {
	contents, err := os.ReadFile(v.ConfigFile)
	if err != nil {
		return fmt.Errorf("error reading configFile %s: %w", v.ConfigFile, err)
	}

	configObj := &api.Configuration{}
	err = yaml.Unmarshal(contents, configObj)
	if err != nil {
		return fmt.Errorf("error unmarshalling configfile: %v", err)
	}
	v.Configuration = configObj
	return nil
}

// running preflight checks
func (v *ValidationRun) preFlightChecks() error {
	if v.Configuration.ImageURL == "" {
		return errors.New("no imageURL specified, aborting run")
	}

	nodeList := &corev1.NodeList{}
	if err := v.clients.runtimeClient.List(v.ctx, nodeList); err != nil {
		return fmt.Errorf("error listing nodes during pre-flight checks: %w", err)
	}

	count := 0
	for _, node := range nodeList.Items {
		if node.DeletionTimestamp == nil && isNodeReady(node) {
			count++
		}
	}

	if count < 2 {
		return errors.New("cluster does not have atleast 2 nodes, aborting run")
	}
	return nil
}

// ApplyDefaults will apply sane defaults for the storage validation configuration
func (v *ValidationRun) applyValidatinoDefaults() error {
	if v.Configuration.VMConfig.CPU == 0 {
		v.Configuration.VMConfig.CPU = DefaultCPU
	}

	if v.Configuration.VMConfig.Memory == "" {
		v.Configuration.VMConfig.Memory = DefaultMem
	}

	if v.Configuration.VMConfig.Memory == "" {
		v.Configuration.VMConfig.Memory = DefaultMem
	}

	if v.Configuration.Timeout == nil {
		v.Configuration.Timeout = &[]int{DefaultTimeout}[0]
	}

	if v.Configuration.SkipCleanup == nil {
		v.Configuration.SkipCleanup = &[]bool{true}[0]
	}

	if v.Configuration.Namespace == "" {
		v.Configuration.Namespace = DefaultNamespace
	}

	if v.Configuration.VMConfig.DiskSize == "" {
		v.Configuration.VMConfig.DiskSize = DefaultDiskSize
	}

	// verify and apply default storageClass if one is not present
	if v.Configuration.StorageClass == "" {
		logrus.Warnf("no default storage class specified, looking up default storageclass")
		scList := &storagev1.StorageClassList{}
		err := v.clients.runtimeClient.List(v.ctx, scList)
		if err != nil {
			return fmt.Errorf("error listing storageclasses: %w", err)
		}
		for _, sc := range scList.Items {
			if val, ok := sc.Annotations[defaultSCAnnotation]; ok && val == "true" {
				v.Configuration.StorageClass = sc.Name
				v.storageClass = &sc
			}
		}
	} else {
		scObj := &storagev1.StorageClass{}
		err := v.clients.runtimeClient.Get(v.ctx, types.NamespacedName{Name: v.Configuration.StorageClass}, scObj)
		if err != nil {
			return fmt.Errorf("error finding storageClass %s: %w", v.Configuration.StorageClass, err)
		}
		v.storageClass = scObj
	}

	// verify if there is no snapshot class if one can be identified from
	// the underlying cdi storage profile
	if v.Configuration.SnapshotClass == "" {
		storageProfileList := &cdi.StorageProfileList{}
		err := v.clients.runtimeClient.List(v.ctx, storageProfileList)
		if err != nil {
			return fmt.Errorf("error listing cdi storageprofiles: %w", err)
		}
		var found bool
		for _, profile := range storageProfileList.Items {
			if profile.Status.StorageClass != nil && *profile.Status.StorageClass == v.Configuration.StorageClass {
				found = true
				v.Configuration.SnapshotClass = *profile.Status.SnapshotClass
			}
		}

		if !found {
			return fmt.Errorf("no storageprofile matching storageclass %s found, no snapshot class specified, aborting check since snapshot based tests cannot be run", v.Configuration.StorageClass)
		}
	}

	return nil
}

func (v *ValidationRun) setupClients() error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("error loading kubeconfig %v", err)
	}

	kubevirtClient, err := kubecli.GetKubevirtClientFromRESTConfig(cfg)
	if err != nil {
		return fmt.Errorf("error generating kubevirt client: %w", err)
	}
	runtimeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("error generating dynamic client interface: %w", err)
	}
	clients := HarvesterClient{
		kubevirtClient: kubevirtClient,
		runtimeClient:  runtimeClient,
	}

	v.cfg = cfg
	v.clients = clients
	return nil
}

// isNodeReady will check from conditions if Ready condition is True
func isNodeReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (v *ValidationRun) IsLonghornV1Engine() bool {
	if v.storageClass.Provisioner == LonghornProvisioner {
		val, ok := v.storageClass.Parameters["dataEngine"]
		if !ok || val != "v2" {
			return true
		}
	}
	return false
}

func (v *ValidationRun) fetchEnvironmentInfo() (api.EnvironmentInfo, error) {
	envInfo := api.EnvironmentInfo{}
	nodeList := &corev1.NodeList{}
	if err := v.clients.runtimeClient.List(v.ctx, nodeList); err != nil {
		return envInfo, fmt.Errorf("error listing nodes in cluster: %w", err)
	}

	envInfo.NodeCount = len(nodeList.Items)

	setting := &v1beta1.Setting{}
	if err := v.clients.runtimeClient.Get(v.ctx, types.NamespacedName{Name: ServerVersionSetting, Namespace: ""}, setting); err != nil {
		return envInfo, fmt.Errorf("error fetching harvester version: %w", err)
	}
	envInfo.HarvesterVersion = setting.Value
	envInfo.ValidatorVersion = v.Version

	return envInfo, nil
}
