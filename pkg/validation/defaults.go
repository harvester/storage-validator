package validation

const (
	DefaultNamespace        = "default"
	DefaultCPU              = 2
	DefaultMem              = "4Gi"
	DefaultDiskSize         = "10Gi"
	DefaultTimeout          = 300 //will be calculated as duration in seconds
	defaultSCAnnotation     = "storageclass.kubernetes.io/is-default-class"
	DefaultPVCSize          = "1Gi"
	DefaultPVCResizeRequest = "2Gi"
	LonghornProvisioner     = "driver.longhorn.io"
	maxRetryCount           = 3
)
