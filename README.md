# storage validation utility
Utility to run some basic storage validation tests against a harvester installation.

The utility uses a configuration file to driver the input for the validation.

At minimal a config.yml with following information is needed

```yaml
namespace: default 
imageURL: "https://download.opensuse.org/repositories/Cloud:/Images:/Leap_15.6/images/openSUSE-Leap-15.6.x86_64-NoCloud.qcow2"
storageClass: lvm
snapshotClass: lvm-snapshot
vmConfig:
  cpu: 2
  ram: 2Gi
  diskSize: 10Gi
skipCleanup: false
timeout: 600
```


Field details are as follows:

| Field | Description | Required | Default |
| --- | --- | --- | --- |
| namespace | namespace to run tests in | no | "default" |
| imageURL | url to a cloud image | yes | |
| storageClass | storage class to be used for running tests | no | defauts to cluster default storage class |
| snapshotClass | snapshot class associated with storage class to be used for snapshot operations | no | defaults to a snapshot class from identified storage class |
| vmConfig.cpu | cores in provisioned VM | no | 2 |
| vmConfig.ram | memory of provisioned VM | no | 2Gi |
| vmConfig.diskSize | size of vm boot disk | no | 10Gi |
| skipClean | skip clean up of resources after validation run, useful for debugging failures | no | false |
| timeout | time in seconds to wait before timing out the validation run | no | 600 seconds |
| accessMode | PVC access mode for all created volumes; set `ReadWriteOnce` for node-local / RWO-only drivers such as LVM CSI | no | ReadWriteMany |
| singleNode | run on a single-node cluster by relaxing the pre-flight `>=2` Ready nodes requirement to `>=1`; implies `skipMigration` | no | false |
| skipMigration | skip the live-migration check (for node-local storage whose volumes cannot migrate) | no | false |

Each of `accessMode`, `singleNode` and `skipMigration` can also be set with the
corresponding command-line flag (`-access-mode`, `-single-node`,
`-skip-migration`), which takes precedence over the config file.

> **Note:** with `singleNode` (or `skipMigration`) the live-migration check is
> **not** run. Live migration is a critical storage capability, so the run
> records it as `skipped` and the report carries a top-level `warnings` entry
> making clear the results do not attest to it. Validate migration on a
> multi-node cluster with migration-capable (RWX) storage.

### To run
`storage-validator` accepts following flags

```
storage-validator -h
Usage of /tmp/storage-validator:
  -access-mode string
    	Override PVC access mode for all created volumes, e.g. ReadWriteOnce for node-local/RWO-only drivers such as LVM CSI. Defaults to config or ReadWriteMany.
  -config string
    	Path to config file (default "config.yaml")
  -debug
    	Debug mode
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -single-node
    	Allow running on a single-node cluster by relaxing the pre-flight >=2 Ready nodes requirement to >=1. Implies -skip-migration.
  -skip-migration
    	Skip the live-migration check (for node-local storage such as LVM CSI whose volumes cannot migrate).
```

### Node-local / RWO-only storage (e.g. LVM CSI)

Node-local, topology-pinned drivers such as the Harvester LVM CSI driver only
support `ReadWriteOnce` and their volumes cannot live-migrate. Such a driver can
still be validated (create/use, snapshot, offline expand, image, VM boot,
hotplug), including on a single-node cluster, using the settings below:

```yaml
namespace: storage-validator-test
imageURL: "https://download.opensuse.org/repositories/Cloud:/Images:/Leap_15.6/images/openSUSE-Leap-15.6.x86_64-NoCloud.qcow2"
storageClass: lvm-thin-encrypted   # your LVM CSI StorageClass
snapshotClass: lvm-snapshot
accessMode: ReadWriteOnce           # LVM CSI rejects ReadWriteMany
singleNode: true                    # run on a single node; implies skipMigration
vmConfig:
  cpu: 2
  ram: 2Gi
  diskSize: 10Gi
skipCleanup: false
timeout: 2400
```

The same run can be driven from flags on top of a standard config:

```
storage-validator -config ./config.yaml -access-mode ReadWriteOnce -single-node
```

Because this skips live migration, the report will include a top-level
`warnings` entry and the migration check will appear with `status: skipped`.

Sample output of utility will be as follows

```
storage-validator -config ./sample/config.yaml
INFO[0000] 🚀 initiate: preflight checks
INFO[0003] ✅  completed: preflight checks
INFO[0007] 🚀 initiate: ensure volume is created and used successfully
INFO[0015] ✅  completed: ensure volume is created and used successfully
INFO[0015] 🚀 initiate: ensure volume snapshot can be created successfully
INFO[0022] ✅  completed: ensure volume snapshot can be created successfully
INFO[0022] 🚀 initiate: ensure offline volume expansion is successful
INFO[0059] ✅  completed: ensure offline volume expansion is successful
INFO[0059] 🚀 initiate: ensure vm image creation is successful
INFO[0077] ✅  completed: ensure vm image creation is successful
INFO[0077] 🚀 initiate: ensure vm can boot from recently created vmimage
INFO[0113] ✅  completed: ensure vm can boot from recently created vmimage
INFO[0113] 🚀 initiate: trigger VM migration
INFO[0130] ✅  completed: trigger VM migration
INFO[0130] 🚀 initiate: hotplug 2 volumes to existing VM
INFO[0136] ✅  completed: hotplug 2 volumes to existing VM
INFO[0136] cleaning up objects created from validation
-------------------------------------
environmentInfo:
  harvesterVersion: v1.6.0
  nodeCount: 2
  validatorVersion: dev
inputConfiguration:
  imageURL: http://10.115.1.6/iso/opensuse/openSUSE-Leap-15.5.x86_64-NoCloud.qcow2
  namespace: default
  skipCleanup: false
  snapshotClass: longhorn-snapshot
  storageClass: harvester-longhorn
  timeout: 600
  vmConfig:
    cpu: 2
    diskSize: 10Gi
results:
- name: hotplug 2 volumes to existing VM
  status: success
- name: trigger VM migration
  status: success
- name: ensure vm can boot from recently created vmimage
  status: success
- name: ensure vm image creation is successful
  status: success
- name: ensure offline volume expansion is successful
  status: success
- name: ensure volume snapshot can be created successfully
  status: success
- name: ensure volume is created and used successfully
  status: success

```